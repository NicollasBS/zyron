package ws

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"zyron-server/internal/auth"
	"zyron-server/internal/db"
	"zyron-server/internal/models"

	"github.com/google/uuid"
)

// ActiveSession mantém o estado em memória da sessão de suporte P2P ativa.
type ActiveSession struct {
	TicketID         string
	ClientUsername   string
	ClientName       string
	OperatorUsername string
	OperatorName     string
	ClientIP         string
	StartTime        time.Time
}

type Hub struct {
	clients        map[string]*Client            // key: client.ID (UUID único por conexão)
	userClients    map[string]map[string]*Client // key: username -> map[clientID]*Client
	activeSessions map[string]*ActiveSession     // indexado por username e ticket_id
	mu             sync.RWMutex
}

var adService = auth.NewLDAPService(nil)

// NewHub cria e inicializa uma nova instância do Hub.
func NewHub() *Hub {
	return &Hub{
		clients:        make(map[string]*Client),
		userClients:    make(map[string]map[string]*Client),
		activeSessions: make(map[string]*ActiveSession),
	}
}

// Register adiciona o cliente conectado ao Hub e registra presença no Redis.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c.ID] = c
	if h.userClients[c.Username] == nil {
		h.userClients[c.Username] = make(map[string]*Client)
	}
	h.userClients[c.Username][c.ID] = c
	h.mu.Unlock()

	fmt.Printf("User connected: %s (Operator: %v, ClientID: %s)\n", c.Username, c.IsOperator, c.ID)

	// Registra presença no Redis
	if err := db.SetPresence(context.Background(), c.Username); err != nil {
		log.Printf("Erro ao registrar presença no Redis para %s: %v\n", c.Username, err)
	}

	// Se for operador, envia o estado atual da fila de chamados
	if c.IsOperator {
		queue, err := db.GetQueue(context.Background())
		if err == nil {
			c.Send(models.WsMessage{
				Type:    "queue_update",
				Payload: queue,
			})
		}
	}
}

// Unregister remove o cliente do Hub, remove presença do Redis e limpa a fila em caso de queda.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c.ID]; ok {
		delete(h.clients, c.ID)
		if clientsMap, exists := h.userClients[c.Username]; exists {
			delete(clientsMap, c.ID)
			if len(clientsMap) == 0 {
				delete(h.userClients, c.Username)
			}
		}
		c.Close()
		fmt.Printf("User disconnected: %s (ClientID: %s)\n", c.Username, c.ID)
	}

	// Verifica se este usuário ainda possui outras conexões ativas
	hasOtherConnections := false
	if clientsMap := h.getUserClientsLocked(c.Username); len(clientsMap) > 0 {
		hasOtherConnections = true
	}

	// Verifica se estava em sessão ativa
	session := h.getActiveSessionLocked(c.Username)
	inSession := session != nil
	if inSession {
		delete(h.activeSessions, session.ClientUsername)
		delete(h.activeSessions, session.OperatorUsername)
		delete(h.activeSessions, strings.ToLower(session.ClientUsername))
		delete(h.activeSessions, strings.ToLower(session.OperatorUsername))
		if session.TicketID != "" {
			delete(h.activeSessions, session.TicketID)
		}
	}
	h.mu.Unlock()

	// Se o usuário não possui nenhuma outra conexão ativa aberta:
	if !hasOtherConnections {
		// Remove presença do Redis
		if err := db.RemovePresence(context.Background(), c.Username); err != nil {
			log.Printf("Erro ao remover presença no Redis para %s: %v\n", c.Username, err)
		}

		// Remove da fila do Redis caso o usuário desconectado tenha chamado pendente
		removed, err := db.RemoveTicketByUsername(context.Background(), c.Username)
		if err != nil {
			log.Printf("Erro ao remover usuário %s da fila do Redis: %v\n", c.Username, err)
		} else if removed != nil {
			log.Printf("Chamado %s (usuário %s, pos anterior: %d) excluído da fila após desconexão. Atualizando posições dos clientes subsequentes...\n",
				removed.ID, c.Username, removed.Position)
			h.BroadcastQueueAndNotifyClients()
		}
	}

	// Se caiu durante uma sessão ativa, grava auditoria e notifica a outra ponta
	if inSession && session != nil {
		partner := session.OperatorUsername
		if c.Username == session.OperatorUsername {
			partner = session.ClientUsername
		}

		h.sendToUser(partner, models.WsMessage{
			Type: "peer_disconnected",
			From: c.Username,
			To:   partner,
			Payload: map[string]interface{}{
				"message": "A outra ponta desconectou abruptamente.",
				"user":    c.Username,
			},
		})

		endTime := time.Now()
		record := &db.SupportSession{
			ID:               uuid.New(),
			TicketID:         session.TicketID,
			ClientUsername:   session.ClientUsername,
			OperatorUsername: session.OperatorUsername,
			ClientIP:         session.ClientIP,
			StartTime:        session.StartTime,
			EndTime:          endTime,
			DurationSeconds:  int64(endTime.Sub(session.StartTime).Seconds()),
		}
		if err := db.SaveSessionRecord(record); err != nil {
			log.Printf("Erro ao salvar auditoria na desconexão abrupta: %v\n", err)
		}
	}
}

// broadcastQueue envia a lista atualizada de chamados para todos os operadores conectados.
// Se notifyClients for true (ex: quando um chamado é atendido ou removido), também notifica os clientes que avançaram de posição.
func (h *Hub) broadcastQueue(notifyClients bool) {
	queue, err := db.GetQueue(context.Background())
	if err != nil {
		log.Printf("Erro ao consultar fila no Redis: %v\n", err)
		return
	}

	msgOp := models.WsMessage{
		Type:    "queue_update",
		Payload: queue,
	}

	h.mu.RLock()
	var operators []*Client
	for _, client := range h.clients {
		if client.IsOperator {
			operators = append(operators, client)
		}
	}
	h.mu.RUnlock()

	// 1. Notifica os operadores com a lista completa e posições
	for _, op := range operators {
		op.Send(msgOp)
	}

	// 2. Notifica cada cliente na fila sobre sua posição atualizada quando a fila se move
	if notifyClients {
		for _, ticket := range queue {
			msgClient := models.WsMessage{
				Type:    "queue_position",
				From:    "server",
				To:      ticket.ClientUser,
				Payload: ticket,
			}
			h.sendToUserFiltered(ticket.ClientUser, msgClient, func(c *Client) bool {
				return !c.IsOperator
			})
		}
	}
}

// BroadcastQueue envia a lista atualizada de chamados para todos os operadores conectados.
func (h *Hub) BroadcastQueue() {
	h.broadcastQueue(false)
}

// BroadcastQueueAndNotifyClients atualiza a fila para os operadores e notifica o avanço de posição aos clientes.
func (h *Hub) BroadcastQueueAndNotifyClients() {
	h.broadcastQueue(true)
}

// getUserClientsLocked busca as conexões ativas de um usuário com correspondência exata ou case-insensitive.
func (h *Hub) getUserClientsLocked(username string) map[string]*Client {
	if cm, exists := h.userClients[username]; exists && len(cm) > 0 {
		return cm
	}
	lower := strings.ToLower(username)
	for u, cm := range h.userClients {
		if strings.ToLower(u) == lower && len(cm) > 0 {
			return cm
		}
	}
	return nil
}

// getActiveSessionLocked busca uma sessão ativa associada a uma chave (username ou ticket_id) case-insensitively.
func (h *Hub) getActiveSessionLocked(key string) *ActiveSession {
	if s, exists := h.activeSessions[key]; exists {
		return s
	}
	lower := strings.ToLower(key)
	for k, s := range h.activeSessions {
		if strings.ToLower(k) == lower {
			return s
		}
	}
	return nil
}

// sendToUser envia uma mensagem de forma não bloqueante para todas as conexões de um usuário específico.
func (h *Hub) sendToUser(username string, msg models.WsMessage) {
	h.sendToUserFiltered(username, msg, nil)
}

// sendToUserFiltered envia uma mensagem para conexões de um usuário específico que atendam ao predicado de filtro.
func (h *Hub) sendToUserFiltered(username string, msg models.WsMessage, filter func(*Client) bool) {
	h.mu.RLock()
	var targets []*Client
	if clientsMap := h.getUserClientsLocked(username); clientsMap != nil {
		for _, client := range clientsMap {
			if filter == nil || filter(client) {
				targets = append(targets, client)
			}
		}
	}
	h.mu.RUnlock()

	for _, target := range targets {
		target.Send(msg)
	}
}

// RouteMessage processa e roteia as mensagens do WebSocket.
// Aceita opcionalmente o cliente emissor (senderClient) para direcionamento contextual.
func (h *Hub) RouteMessage(msg models.WsMessage, senderClient ...*Client) {
	var sender *Client
	if len(senderClient) > 0 {
		sender = senderClient[0]
	}

	switch msg.Type {

	case "request_support":
		// Cliente entra na fila de suporte (ou consulta posição se já estiver na fila)
		ticketID := uuid.New().String()
		hostname := ""
		ip := ""
		clientName := ""

		if payloadMap, ok := msg.Payload.(map[string]interface{}); ok {
			if hName, ok := payloadMap["hostname"].(string); ok && hName != "" {
				hostname = hName
			}
			if id, ok := payloadMap["id"].(string); ok && id != "" {
				ticketID = id
			}
			if clientIP, ok := payloadMap["ip"].(string); ok && clientIP != "" {
				ip = clientIP
			}
			if cName, ok := payloadMap["client_name"].(string); ok && cName != "" {
				clientName = cName
			} else if cName, ok := payloadMap["name"].(string); ok && cName != "" {
				clientName = cName
			}
		}

		if clientName == "" {
			if prof, err := adService.GetUserProfile(msg.From); err == nil && prof != nil {
				clientName = prof.DisplayName
			}
		}
		if clientName == "" {
			clientName = msg.From
		}

		// Se o IP não foi enviado no payload, utiliza o IP da conexão WebSocket
		if ip == "" {
			if sender != nil && sender.RemoteIP != "" {
				ip = sender.RemoteIP
			} else {
				h.mu.RLock()
				if clientsMap, ok := h.userClients[msg.From]; ok {
					for _, c := range clientsMap {
						if c.RemoteIP != "" {
							ip = c.RemoteIP
							break
						}
					}
				}
				h.mu.RUnlock()
			}
		}

		ticketToProcess := &models.Ticket{
			ID:         ticketID,
			ClientUser: msg.From,
			ClientName: clientName,
			Hostname:   hostname,
			IP:         ip,
			Status:     "waiting",
		}

		actualTicket, position, created, err := db.PushTicket(context.Background(), ticketToProcess)
		if err != nil {
			log.Printf("Erro ao processar chamado na fila do Redis: %v\n", err)
			return
		}

		if created {
			log.Printf("Cliente %s (%s) entrou na fila na posição %d (Ticket ID: %s)\n", msg.From, ip, position, actualTicket.ID)
			// Broadcast de atualização da fila para todos os operadores e posições dos clientes
			h.BroadcastQueue()
		} else {
			log.Printf("Cliente %s já possui chamado ativo (Ticket ID: %s), mantido na posição %d\n", msg.From, actualTicket.ID, position)
		}

		// Notifica o cliente com o chamado e sua posição na fila
		ticketCreatedMsg := models.WsMessage{
			Type:    "ticket_created",
			From:    "server",
			To:      msg.From,
			Payload: actualTicket,
		}

		if sender != nil {
			sender.Send(ticketCreatedMsg)
		} else {
			h.sendToUserFiltered(msg.From, ticketCreatedMsg, func(c *Client) bool {
				return !c.IsOperator
			})
		}
		return

	case "accept_ticket":
		// Operador aceita um chamado
		var targetTicketID string
		var targetClient string

		if msg.To != "" {
			targetClient = msg.To
		}

		switch p := msg.Payload.(type) {
		case string:
			targetTicketID = p
		case map[string]interface{}:
			if tID, ok := p["ticket_id"].(string); ok {
				targetTicketID = tID
			} else if tID, ok := p["id"].(string); ok {
				targetTicketID = tID
			}
			if cUser, ok := p["client_user"].(string); ok {
				targetClient = cUser
			} else if cUser, ok := p["client"].(string); ok {
				targetClient = cUser
			}
		}

		// 1. Validação de pré-condição: operador já está em atendimento ativo?
		h.mu.RLock()
		existingOpSession, opInCall := h.activeSessions[msg.From]
		if !opInCall {
			existingOpSession, opInCall = h.activeSessions[strings.ToLower(msg.From)]
		}
		h.mu.RUnlock()

		if opInCall && existingOpSession != nil {
			log.Printf("[Hub] Operador %s já possui sessão ativa (chamado %s)\n", msg.From, existingOpSession.TicketID)
			opErr := models.WsMessage{
				Type: "ticket_accept_failed",
				From: "server",
				To:   msg.From,
				Payload: map[string]interface{}{
					"ticket_id": targetTicketID,
					"error":     "Você já está em um atendimento ativo.",
				},
			}
			if sender != nil {
				sender.Send(opErr)
			}
			return
		}

		// 2. Validação de pré-condição: cliente já está em atendimento com outro operador?
		if targetClient != "" {
			h.mu.RLock()
			existingClientSession, clientInCall := h.activeSessions[targetClient]
			if !clientInCall {
				existingClientSession, clientInCall = h.activeSessions[strings.ToLower(targetClient)]
			}
			h.mu.RUnlock()

			if clientInCall && existingClientSession != nil {
				log.Printf("[Hub] Cliente %s já está em atendimento com %s. Rejeitando aceite concorrente de %s.\n",
					targetClient, existingClientSession.OperatorUsername, msg.From)
				opErr := models.WsMessage{
					Type: "ticket_accept_failed",
					From: "server",
					To:   msg.From,
					Payload: map[string]interface{}{
						"ticket_id": targetTicketID,
						"error":     "Chamado já foi atendido por outra pessoa.",
					},
				}
				if sender != nil {
					sender.Send(opErr)
				}
				h.BroadcastQueueAndNotifyClients()
				return
			}
		}

		// 3. Retirada 100% atômica do chamado da fila (apenas um operador vence a corrida no Redis)
		var removedTicket *models.Ticket
		var err error

		if targetTicketID != "" {
			removedTicket, err = db.RemoveTicket(context.Background(), targetTicketID)
		} else if targetClient != "" {
			removedTicket, err = db.RemoveTicketByUsername(context.Background(), targetClient)
		} else {
			// Se nenhum parâmetro foi enviado, retira o mais antigo da fila
			removedTicket, err = db.PopTicket(context.Background())
		}

		if err != nil {
			log.Printf("Erro ao remover chamado no aceite do operador %s: %v\n", msg.From, err)
			opErr := models.WsMessage{
				Type: "ticket_accept_failed",
				From: "server",
				To:   msg.From,
				Payload: map[string]interface{}{
					"ticket_id": targetTicketID,
					"error":     "Erro ao processar chamado: " + err.Error(),
				},
			}
			if sender != nil {
				sender.Send(opErr)
			}
			return
		}

		// Se o chamado não foi retirado, outro operador foi mais rápido ou o cliente cancelou
		if removedTicket == nil {
			log.Printf("[Hub] Chamado %s (%s) já foi atendido por outro operador ou não existe mais na fila (tentativa de %s)\n",
				targetTicketID, targetClient, msg.From)
			opErr := models.WsMessage{
				Type: "ticket_accept_failed",
				From: "server",
				To:   msg.From,
				Payload: map[string]interface{}{
					"ticket_id": targetTicketID,
					"error":     "Chamado já foi atendido por outra pessoa.",
				},
			}
			if sender != nil {
				sender.Send(opErr)
			} else {
				h.sendToUserFiltered(msg.From, opErr, func(c *Client) bool {
					return c.IsOperator
				})
			}
			h.BroadcastQueueAndNotifyClients()
			return
		}

		targetTicketID = removedTicket.ID
		if targetTicketID == "" {
			targetTicketID = removedTicket.TicketID
		}
		targetClient = removedTicket.ClientUser

		if targetClient == "" {
			log.Printf("[Hub] Chamado %s inválido (sem client_user) no aceite de %s\n", targetTicketID, msg.From)
			return
		}

		// 4. Trava de concorrência em memória para garantir consistência no Hub
		h.mu.Lock()
		if existing, inCall := h.activeSessions[targetClient]; inCall && existing != nil {
			h.mu.Unlock()
			log.Printf("[Hub] Concorrência crítica detectada: cliente %s já em sessão com %s. Abortando aceite de %s.\n",
				targetClient, existing.OperatorUsername, msg.From)
			opErr := models.WsMessage{
				Type: "ticket_accept_failed",
				From: "server",
				To:   msg.From,
				Payload: map[string]interface{}{
					"ticket_id": targetTicketID,
					"error":     "Chamado já foi atendido por outra pessoa.",
				},
			}
			if sender != nil {
				sender.Send(opErr)
			}
			h.BroadcastQueueAndNotifyClients()
			return
		}

		var targetIP string
		if removedTicket != nil {
			targetIP = removedTicket.IP
		}
		if targetIP == "" {
			if clientsMap := h.getUserClientsLocked(targetClient); clientsMap != nil {
				for _, c := range clientsMap {
					if !c.IsOperator && c.RemoteIP != "" {
						targetIP = c.RemoteIP
						break
					}
					if targetIP == "" && c.RemoteIP != "" {
						targetIP = c.RemoteIP
					}
				}
			}
		}

		clientName := ""
		if removedTicket != nil && removedTicket.ClientName != "" {
			clientName = removedTicket.ClientName
		}
		if clientName == "" {
			if prof, err := adService.GetUserProfile(targetClient); err == nil && prof != nil {
				clientName = prof.DisplayName
			}
		}
		if clientName == "" {
			clientName = targetClient
		}

		operatorName := ""
		if prof, err := adService.GetUserProfile(msg.From); err == nil && prof != nil {
			operatorName = prof.DisplayName
		}
		if operatorName == "" {
			operatorName = msg.From
		}

		// Associa o operador ao cliente em sessão ativa
		session := &ActiveSession{
			TicketID:         targetTicketID,
			ClientUsername:   targetClient,
			ClientName:       clientName,
			OperatorUsername: msg.From,
			OperatorName:     operatorName,
			ClientIP:         targetIP,
			StartTime:        time.Now(),
		}

		h.activeSessions[targetClient] = session
		h.activeSessions[msg.From] = session
		h.activeSessions[strings.ToLower(targetClient)] = session
		h.activeSessions[strings.ToLower(msg.From)] = session
		if targetTicketID != "" {
			h.activeSessions[targetTicketID] = session
		}
		h.mu.Unlock()

		log.Printf("Operador %s (%s) aceitou chamado %s do cliente %s (%s) (IP: %s)\n", msg.From, operatorName, targetTicketID, targetClient, clientName, targetIP)

		// Atualiza a fila para todos os operadores e notifica o avanço de posição aos clientes na fila
		h.BroadcastQueueAndNotifyClients()

		// Notifica o cliente com o destinatário (To) correto para iniciar a oferta SDP do WebRTC
		h.sendToUserFiltered(targetClient, models.WsMessage{
			Type: "ticket_accepted",
			From: msg.From,     // Operador
			To:   targetClient, // Cliente
			Payload: map[string]interface{}{
				"ticket_id":     targetTicketID,
				"operator":      msg.From,
				"operator_name": operatorName,
			},
		}, func(c *Client) bool {
			return !c.IsOperator
		})

		// Notifica o operador da confirmação de associação com o IP do cliente para conexão direta
		opConfirmMsg := models.WsMessage{
			Type: "ticket_accepted",
			From: "server",
			To:   msg.From,
			Payload: map[string]interface{}{
				"ticket_id":   targetTicketID,
				"client_user": targetClient,
				"client_name": clientName,
				"client_ip":   targetIP,
			},
		}
		if sender != nil {
			sender.Send(opConfirmMsg)
		} else {
			h.sendToUserFiltered(msg.From, opConfirmMsg, func(c *Client) bool {
				return c.IsOperator
			})
		}
		return

	case "session_closed":
		// Encerramento de sessão com gravação exclusiva de auditoria no PostgreSQL
		var ticketIDPayload string
		if pMap, ok := msg.Payload.(map[string]interface{}); ok {
			if tID, ok := pMap["ticket_id"].(string); ok {
				ticketIDPayload = tID
			}
		}

		h.mu.Lock()
		var session *ActiveSession
		if ticketIDPayload != "" {
			session = h.getActiveSessionLocked(ticketIDPayload)
		}
		if session == nil {
			session = h.getActiveSessionLocked(msg.From)
		}
		if session == nil && msg.To != "" {
			session = h.getActiveSessionLocked(msg.To)
		}
		if session != nil {
			delete(h.activeSessions, session.ClientUsername)
			delete(h.activeSessions, session.OperatorUsername)
			delete(h.activeSessions, strings.ToLower(session.ClientUsername))
			delete(h.activeSessions, strings.ToLower(session.OperatorUsername))
			if session.TicketID != "" {
				delete(h.activeSessions, session.TicketID)
			}
		}
		h.mu.Unlock()

		endTime := time.Now()
		record := &db.SupportSession{
			ID:      uuid.New(),
			EndTime: endTime,
		}

		if session != nil {
			record.TicketID = session.TicketID
			record.ClientUsername = session.ClientUsername
			record.ClientDisplayName = session.ClientName
			record.OperatorUsername = session.OperatorUsername
			record.OperatorDisplayName = session.OperatorName
			record.ClientIP = session.ClientIP
			record.StartTime = session.StartTime
			record.DurationSeconds = int64(endTime.Sub(session.StartTime).Seconds())
		}

		// Se o payload contiver informações adicionais para o registro de auditoria
		if pMap, ok := msg.Payload.(map[string]interface{}); ok {
			if tID, ok := pMap["ticket_id"].(string); ok && tID != "" {
				record.TicketID = tID
			}
			if cUser, ok := pMap["client_username"].(string); ok && cUser != "" {
				record.ClientUsername = cUser
			}
			if opUser, ok := pMap["operator_username"].(string); ok && opUser != "" {
				record.OperatorUsername = opUser
			}
		}

		if record.ClientUsername == "" && msg.From != "" {
			record.ClientUsername = msg.From
		}
		if record.OperatorUsername == "" && msg.To != "" {
			record.OperatorUsername = msg.To
		}
		if record.StartTime.IsZero() {
			record.StartTime = endTime
		}

		// Gravação no PostgreSQL conforme requisito 2
		if err := db.SaveSessionRecord(record); err != nil {
			log.Printf("Erro ao gravar registro de auditoria da sessão no PostgreSQL: %v\n", err)
		} else {
			log.Printf("Sessão finalizada e auditada no Postgres (ID: %s, Ticket: %s, Duração: %ds)\n",
				record.ID, record.TicketID, record.DurationSeconds)
		}

		// Encaminha notificação de fechamento para a outra ponta
		partner := msg.To
		if partner == "" && session != nil {
			partner = session.OperatorUsername
			if msg.From == session.OperatorUsername {
				partner = session.ClientUsername
			}
		}
		if partner != "" {
			if partner == msg.From && sender != nil {
				// Caso de teste com mesmo usuário para ambos
				h.sendToUserFiltered(partner, msg, func(c *Client) bool {
					return c.IsOperator != sender.IsOperator
				})
			} else {
				h.sendToUser(partner, msg)
			}
		}
		return

	case "get_queue":
		// Operador requisita fila atual
		queue, err := db.GetQueue(context.Background())
		if err == nil {
			qMsg := models.WsMessage{
				Type:    "queue_update",
				From:    "server",
				To:      msg.From,
				Payload: queue,
			}
			if sender != nil {
				sender.Send(qMsg)
			} else {
				h.sendToUser(msg.From, qMsg)
			}
		}
		return

	case "cancel_ticket", "cancel_support":
		// Cliente cancela seu pedido de suporte voluntariamente
		removed, err := db.RemoveTicketByUsername(context.Background(), msg.From)
		if err != nil {
			log.Printf("Erro ao cancelar chamado para %s: %v\n", msg.From, err)
			return
		}
		if removed != nil {
			log.Printf("Chamado %s cancelado voluntariamente por %s\n", removed.ID, msg.From)
			h.BroadcastQueueAndNotifyClients()
			cMsg := models.WsMessage{
				Type:    "ticket_cancelled",
				From:    "server",
				To:      msg.From,
				Payload: removed,
			}
			if sender != nil {
				sender.Send(cMsg)
			} else {
				h.sendToUserFiltered(msg.From, cMsg, func(c *Client) bool {
					return !c.IsOperator
				})
			}
		}
		return

	case "get_queue_position", "get_position":
		// Cliente consulta sua posição atual na fila
		pos, ticket, err := db.GetQueuePosition(context.Background(), msg.From)
		if err != nil {
			log.Printf("Erro ao consultar posição na fila para %s: %v\n", msg.From, err)
			return
		}
		var posMsg models.WsMessage
		if ticket != nil {
			ticket.Position = pos
			ticket.QueuePosition = pos
			if ticket.TicketID == "" {
				ticket.TicketID = ticket.ID
			}
			posMsg = models.WsMessage{
				Type:    "queue_position",
				From:    "server",
				To:      msg.From,
				Payload: ticket,
			}
		} else {
			posMsg = models.WsMessage{
				Type:    "queue_position",
				From:    "server",
				To:      msg.From,
				Payload: map[string]interface{}{
					"position":       0,
					"queue_position": 0,
					"status":         "not_in_queue",
				},
			}
		}
		if sender != nil {
			sender.Send(posMsg)
		} else {
			h.sendToUserFiltered(msg.From, posMsg, func(c *Client) bool {
				return !c.IsOperator
			})
		}
		return

	case "rate_session", "session_rating":
		// Avaliação do atendimento enviada pelo cliente
		ticketID := ""
		rating := 5
		feedback := ""
		if pMap, ok := msg.Payload.(map[string]interface{}); ok {
			if tID, ok := pMap["ticket_id"].(string); ok {
				ticketID = tID
			}
			if rVal, ok := pMap["rating"].(float64); ok {
				rating = int(rVal)
			} else if rVal, ok := pMap["rating"].(int); ok {
				rating = rVal
			}
			if fText, ok := pMap["feedback"].(string); ok {
				feedback = fText
			}
		}
		if err := db.SaveSessionRating(ticketID, msg.From, rating, feedback); err != nil {
			log.Printf("Erro ao salvar avaliação do atendimento (Ticket: %s, User: %s): %v\n", ticketID, msg.From, err)
		} else {
			log.Printf("Avaliação de atendimento registrada (Ticket: %s, User: %s, Nota: %d, Feedback: %s)\n", ticketID, msg.From, rating, feedback)
		}
		return

	case "ping":
		// Heartbeat a nível de aplicação (JSON)
		_ = db.RenewPresence(context.Background(), msg.From)
		pongMsg := models.WsMessage{
			Type: "pong",
			From: "server",
			To:   msg.From,
		}
		if sender != nil {
			sender.Send(pongMsg)
		} else {
			h.sendToUser(msg.From, pongMsg)
		}
		return
	}

	// Roteamento ponto a ponto (Sinalização WebRTC: offer, answer, ice-candidate, etc.)
	if msg.To != "" {
		h.mu.RLock()
		var targets []*Client
		if clientsMap := h.getUserClientsLocked(msg.To); clientsMap != nil {
			for _, client := range clientsMap {
				// Se a mensagem veio de uma conexão específica, não reflete de volta para ela
				if sender != nil && client.ID == sender.ID {
					continue
				}
				// Se remetente e destinatário têm o mesmo username (teste local/dev)
				// envia do Operador para o Cliente e vice-versa
				if sender != nil && strings.EqualFold(msg.From, msg.To) {
					if sender.IsOperator != client.IsOperator {
						targets = append(targets, client)
					}
					continue
				}
				targets = append(targets, client)
			}
		}
		h.mu.RUnlock()

		if len(targets) > 0 {
			for _, target := range targets {
				target.Send(msg)
			}
		} else {
			log.Printf("Destinatário %s não encontrado para mensagem %s\n", msg.To, msg.Type)
		}
		return
	}
}

// RealtimeStats agrega os dados em tempo real da fila, operadores e chamadas ativas.
type RealtimeStats struct {
	QueueLength           int                 `json:"queue_length"`
	QueueTickets          []models.Ticket     `json:"queue_tickets"`
	AvailableOperators    []OperatorInfo      `json:"available_operators"`
	BusyOperators         []OperatorInfo      `json:"busy_operators"`
	ActiveSessions        []ActiveSessionInfo `json:"active_sessions"`
	TotalConnectedClients int                 `json:"total_connected_clients"`
}

type OperatorInfo struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsBusy      bool   `json:"is_busy"`
	ActiveWith  string `json:"active_with,omitempty"`
}

type ActiveSessionInfo struct {
	TicketID            string `json:"ticket_id"`
	ClientUsername      string `json:"client_username"`
	ClientDisplayName   string `json:"client_display_name"`
	OperatorUsername    string `json:"operator_username"`
	OperatorDisplayName string `json:"operator_display_name"`
	ClientIP            string `json:"client_ip"`
	StartTime           string `json:"start_time"`
	DurationSeconds     int64  `json:"duration_seconds"`
}

// GetRealtimeManagerStats compila os dados ao vivo do hub e fila Redis para o gerenciador.
func (h *Hub) GetRealtimeManagerStats() (*RealtimeStats, error) {
	queue, err := db.GetQueue(context.Background())
	if err != nil {
		queue = []models.Ticket{}
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	availableOps := []OperatorInfo{}
	busyOps := []OperatorInfo{}
	seenOperators := make(map[string]bool)

	// Localiza operadores conectados
	for _, client := range h.clients {
		if client.IsOperator && !seenOperators[client.Username] {
			seenOperators[client.Username] = true
			isBusy := false
			activeWith := ""

			// Verifica se este operador está em sessão ativa
			if session := h.getActiveSessionLocked(client.Username); session != nil {
				isBusy = true
				activeWith = session.ClientName
				if activeWith == "" {
					activeWith = session.ClientUsername
				}
			}

			opInfo := OperatorInfo{
				Username:    client.Username,
				DisplayName: client.Username,
				IsBusy:      isBusy,
				ActiveWith:  activeWith,
			}

			if isBusy {
				busyOps = append(busyOps, opInfo)
			} else {
				availableOps = append(availableOps, opInfo)
			}
		}
	}

	// Compila sessões ativas únicas
	activeList := []ActiveSessionInfo{}
	seenTickets := make(map[string]bool)
	now := time.Now()

	for _, session := range h.activeSessions {
		if session != nil && !seenTickets[session.TicketID] {
			seenTickets[session.TicketID] = true
			duration := int64(now.Sub(session.StartTime).Seconds())
			if duration < 0 {
				duration = 0
			}

			cName := session.ClientName
			if cName == "" {
				cName = session.ClientUsername
			}
			opName := session.OperatorName
			if opName == "" {
				opName = session.OperatorUsername
			}

			activeList = append(activeList, ActiveSessionInfo{
				TicketID:            session.TicketID,
				ClientUsername:      session.ClientUsername,
				ClientDisplayName:   cName,
				OperatorUsername:    session.OperatorUsername,
				OperatorDisplayName: opName,
				ClientIP:            session.ClientIP,
				StartTime:           session.StartTime.Format(time.RFC3339),
				DurationSeconds:     duration,
			})
		}
	}

	return &RealtimeStats{
		QueueLength:           len(queue),
		QueueTickets:          queue,
		AvailableOperators:    availableOps,
		BusyOperators:         busyOps,
		ActiveSessions:        activeList,
		TotalConnectedClients: len(h.clients),
	}, nil
}
