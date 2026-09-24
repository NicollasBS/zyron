package ws_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"zyron-server/internal/db"
	"zyron-server/internal/models"
	"zyron-server/internal/ws"

	"github.com/gorilla/websocket"
)

func setupWSTest(t *testing.T) (*ws.Hub, *httptest.Server) {
	if err := db.InitRedis(); err != nil {
		t.Fatalf("Erro ao conectar no Redis: %v", err)
	}
	if err := db.InitPostgres(); err != nil {
		t.Fatalf("Erro ao conectar no Postgres: %v", err)
	}

	// Limpa a fila antes de cada teste
	for {
		item, _ := db.PopTicket(context.Background())
		if item == nil {
			break
		}
	}

	hub := ws.NewHub()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, w, r)
	}))

	return hub, server
}


func connectWS(t *testing.T, serverURL string, username string, isOperator bool) *websocket.Conn {
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "?username=" + username
	if isOperator {
		wsURL += "&is_operator=true"
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Falha ao conectar WebSocket para %s: %v", username, err)
	}
	return conn
}

func TestHubLifecycleAndSignaling(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	// Limpa a fila antes do teste
	for {
		item, _ := db.PopTicket(context.Background())
		if item == nil {
			break
		}
	}

	clientUser := "DOMAIN\\client1"
	opUser := "DOMAIN\\operator1"

	// 1. Conecta o Operador
	opConn := connectWS(t, server.URL, opUser, true)
	defer opConn.Close()

	// Operador deve receber queue_update inicial na conexão
	var initQueueMsg models.WsMessage
	if err := opConn.ReadJSON(&initQueueMsg); err != nil {
		t.Fatalf("Erro ao ler queue_update inicial do operador: %v", err)
	}
	if initQueueMsg.Type != "queue_update" {
		t.Fatalf("Esperava type queue_update, obteve %s", initQueueMsg.Type)
	}

	// 2. Conecta o Cliente
	clientConn := connectWS(t, server.URL, clientUser, false)
	defer clientConn.Close()

	// Aguarda registro no Redis
	var online bool
	var err error
	for i := 0; i < 10; i++ {
		online, err = db.IsUserOnline(context.Background(), clientUser)
		if err == nil && online {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil || !online {
		t.Fatalf("Cliente deveria estar com presença ativa no Redis (online=%v, err=%v)", online, err)
	}

	// 3. Cliente envia "request_support" com IP
	reqMsg := models.WsMessage{
		Type: "request_support",
		From: clientUser,
		Payload: map[string]interface{}{
			"hostname": "FINANCE-PC-01",
			"ip":       "192.168.10.123",
		},
	}
	if err := clientConn.WriteJSON(reqMsg); err != nil {
		t.Fatalf("Erro ao enviar request_support: %v", err)
	}

	// Operador deve receber broadcast com a fila atualizada
	var opQueueUpdate models.WsMessage
	if err := opConn.ReadJSON(&opQueueUpdate); err != nil {
		t.Fatalf("Erro ao receber queue_update no operador: %v", err)
	}
	if opQueueUpdate.Type != "queue_update" {
		t.Fatalf("Esperava broadcast queue_update, obteve %s", opQueueUpdate.Type)
	}

	// Cliente recebe confirmação "ticket_created" com position=1
	var ticketCreatedMsg models.WsMessage
	if err := clientConn.ReadJSON(&ticketCreatedMsg); err != nil {
		t.Fatalf("Erro ao ler confirmação ticket_created: %v", err)
	}
	if ticketCreatedMsg.Type != "ticket_created" {
		t.Fatalf("Esperava ticket_created, obteve %s", ticketCreatedMsg.Type)
	}
	var firstTicketID string
	if pMap, ok := ticketCreatedMsg.Payload.(map[string]interface{}); ok {
		if pos, ok := pMap["position"].(float64); !ok || int(pos) != 1 {
			t.Errorf("Esperava position=1 no ticket_created, obteve %v", pMap["position"])
		}
		firstTicketID = pMap["id"].(string)
	}

	// 3.1 Cliente envia "request_support" novamente (duplicidade)
	if err := clientConn.WriteJSON(reqMsg); err != nil {
		t.Fatalf("Erro ao enviar request_support repetido: %v", err)
	}
	var dupTicketMsg models.WsMessage
	if err := clientConn.ReadJSON(&dupTicketMsg); err != nil {
		t.Fatalf("Erro ao ler resposta para request repetido: %v", err)
	}
	if pMap, ok := dupTicketMsg.Payload.(map[string]interface{}); ok {
		if pMap["id"] != firstTicketID {
			t.Errorf("ID do ticket alterado no pedido duplicado: %v vs %v", pMap["id"], firstTicketID)
		}
		if pos, ok := pMap["position"].(float64); !ok || int(pos) != 1 {
			t.Errorf("Posição alterada indevidamente no pedido duplicado: %v", pMap["position"])
		}
	}

	// 4. Operador aceita o ticket com "accept_ticket"
	acceptMsg := models.WsMessage{
		Type: "accept_ticket",
		From: opUser,
		Payload: map[string]interface{}{
			"client_user": clientUser,
		},
	}
	if err := opConn.WriteJSON(acceptMsg); err != nil {
		t.Fatalf("Erro ao enviar accept_ticket: %v", err)
	}

	// Operador recebe atualização da fila (chamado removido)
	var opQueueAfterAccept models.WsMessage
	if err := opConn.ReadJSON(&opQueueAfterAccept); err != nil {
		t.Fatalf("Erro ao receber fila atualizada após aceite: %v", err)
	}
	if opQueueAfterAccept.Type != "queue_update" {
		t.Fatalf("Esperava queue_update, obteve %s", opQueueAfterAccept.Type)
	}

	// Operador recebe confirmação de ticket_accepted contendo o client_ip
	var opAcceptConfirm models.WsMessage
	if err := opConn.ReadJSON(&opAcceptConfirm); err != nil {
		t.Fatalf("Erro ao receber confirmação de aceite no operador: %v", err)
	}
	if opAcceptConfirm.Type != "ticket_accepted" {
		t.Fatalf("Esperava ticket_accepted no operador, obteve %s", opAcceptConfirm.Type)
	}
	if opPayload, ok := opAcceptConfirm.Payload.(map[string]interface{}); ok {
		if opPayload["client_ip"] != "192.168.10.123" {
			t.Errorf("Esperava client_ip 192.168.10.123 no aceite do operador, obteve %v", opPayload["client_ip"])
		}
	}

	// Cliente recebe notificação ticket_accepted com To/From correto para WebRTC
	var clientAcceptedMsg models.WsMessage
	if err := clientConn.ReadJSON(&clientAcceptedMsg); err != nil {
		t.Fatalf("Erro ao ler ticket_accepted no cliente: %v", err)
	}
	if clientAcceptedMsg.Type != "ticket_accepted" || clientAcceptedMsg.From != opUser {
		t.Fatalf("ticket_accepted no cliente inválido: %+v", clientAcceptedMsg)
	}

	// 5. Teste de Sinalização WebRTC P2P (Offer/Answer/Candidate)
	// Cliente envia Offer SDP para o Operador
	offerMsg := models.WsMessage{
		Type:    "offer",
		From:    clientUser,
		To:      opUser,
		Payload: map[string]string{"sdp": "v=0\r\no=- 461173..."},
	}
	if err := clientConn.WriteJSON(offerMsg); err != nil {
		t.Fatalf("Erro ao enviar offer: %v", err)
	}

	var recOffer models.WsMessage
	if err := opConn.ReadJSON(&recOffer); err != nil {
		t.Fatalf("Erro ao receber offer no operador: %v", err)
	}
	if recOffer.Type != "offer" || recOffer.From != clientUser {
		t.Fatalf("Offer recebido incorreto: %+v", recOffer)
	}

	// Operador responde com Answer SDP
	answerMsg := models.WsMessage{
		Type:    "answer",
		From:    opUser,
		To:      clientUser,
		Payload: map[string]string{"sdp": "v=0\r\no=- 891234..."},
	}
	if err := opConn.WriteJSON(answerMsg); err != nil {
		t.Fatalf("Erro ao enviar answer: %v", err)
	}

	var recAnswer models.WsMessage
	if err := clientConn.ReadJSON(&recAnswer); err != nil {
		t.Fatalf("Erro ao receber answer no cliente: %v", err)
	}
	if recAnswer.Type != "answer" || recAnswer.From != opUser {
		t.Fatalf("Answer recebido incorreto: %+v", recAnswer)
	}

	// 6. Encerramento da Sessão com "session_closed" e Auditoria
	closeMsg := models.WsMessage{
		Type: "session_closed",
		From: opUser,
		To:   clientUser,
	}
	if err := opConn.WriteJSON(closeMsg); err != nil {
		t.Fatalf("Erro ao enviar session_closed: %v", err)
	}

	// Cliente recebe notificação de encerramento
	var recClosed models.WsMessage
	if err := clientConn.ReadJSON(&recClosed); err != nil {
		t.Fatalf("Erro ao receber session_closed no cliente: %v", err)
	}
	if recClosed.Type != "session_closed" {
		t.Fatalf("Esperava session_closed no cliente, obteve %s", recClosed.Type)
	}

	// Aguarda gravação no Postgres
	time.Sleep(100 * time.Millisecond)

	var sessionRecord db.SupportSession
	dbErr := db.DB.Where("client_username = ? AND operator_username = ?", clientUser, opUser).
		Order("end_time desc").First(&sessionRecord).Error
	if dbErr != nil {
		t.Fatalf("Registro de sessão não encontrado no PostgreSQL: %v", dbErr)
	}
	if sessionRecord.ClientUsername != clientUser || sessionRecord.OperatorUsername != opUser {
		t.Fatalf("Registro de auditoria com dados divergentes: %+v", sessionRecord)
	}
}

func TestAbruptDisconnectRemovesFromQueue(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	clientUser := "DOMAIN\\disconnectUser"
	opUser := "DOMAIN\\opQueueWatcher"

	opConn := connectWS(t, server.URL, opUser, true)
	defer opConn.Close()

	// Lê queue inicial
	var msg models.WsMessage
	_ = opConn.ReadJSON(&msg)

	// Conecta cliente e entra na fila
	clientConn := connectWS(t, server.URL, clientUser, false)
	_ = clientConn.WriteJSON(models.WsMessage{
		Type: "request_support",
		From: clientUser,
	})

	// Operador recebe update da entrada
	_ = opConn.ReadJSON(&msg)
	// Cliente recebe ticket_created
	_ = clientConn.ReadJSON(&msg)

	// Cliente desconecta abruptamente
	clientConn.Close()

	// Operador deve receber broadcast com fila atualizada (cliente removido)
	var queueAfterDisconnect models.WsMessage
	if err := opConn.ReadJSON(&queueAfterDisconnect); err != nil {
		t.Fatalf("Erro ao receber queue_update após desconexão: %v", err)
	}

	// Verifica se a fila do Redis não tem mais o cliente
	queue, err := db.GetQueue(context.Background())
	if err != nil {
		t.Fatalf("Erro ao buscar fila: %v", err)
	}
	for _, ticket := range queue {
		if ticket.ClientUser == clientUser {
			t.Fatalf("Chamado do usuário desconectado ainda se encontra na fila do Redis: %+v", ticket)
		}
	}
}

func TestDisconnectionUpdatesSubsequentQueuePositions(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	opUser := "DOMAIN\\opQueueTester"
	u1 := "DOMAIN\\queueUser1"
	u2 := "DOMAIN\\queueUser2"
	u3 := "DOMAIN\\queueUser3"

	// 1. Conecta o operador
	opConn := connectWS(t, server.URL, opUser, true)
	defer opConn.Close()

	var opMsg models.WsMessage
	_ = opConn.ReadJSON(&opMsg) // Drena fila inicial

	// 2. Conecta u1 e entra na fila (Posição 1)
	c1Conn := connectWS(t, server.URL, u1, false)
	defer c1Conn.Close()
	_ = c1Conn.WriteJSON(models.WsMessage{Type: "request_support", From: u1})
	_ = opConn.ReadJSON(&opMsg)

	var c1Created models.WsMessage
	if err := c1Conn.ReadJSON(&c1Created); err != nil {
		t.Fatalf("Erro ao ler ticket_created para u1: %v", err)
	}
	if pMap, ok := c1Created.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 1 {
			t.Fatalf("Esperava posição 1 para u1, obteve %v", pMap["position"])
		}
	}

	// 3. Conecta u2 e entra na fila (Posição 2)
	c2Conn := connectWS(t, server.URL, u2, false)
	defer c2Conn.Close()
	_ = c2Conn.WriteJSON(models.WsMessage{Type: "request_support", From: u2})
	_ = opConn.ReadJSON(&opMsg)

	var c2Created models.WsMessage
	if err := c2Conn.ReadJSON(&c2Created); err != nil {
		t.Fatalf("Erro ao ler ticket_created para u2: %v", err)
	}
	if pMap, ok := c2Created.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 2 {
			t.Fatalf("Esperava posição 2 para u2, obteve %v", pMap["position"])
		}
	}

	// 4. Conecta u3 e entra na fila (Posição 3)
	c3Conn := connectWS(t, server.URL, u3, false)
	defer c3Conn.Close()
	_ = c3Conn.WriteJSON(models.WsMessage{Type: "request_support", From: u3})
	_ = opConn.ReadJSON(&opMsg)

	var c3Created models.WsMessage
	if err := c3Conn.ReadJSON(&c3Created); err != nil {
		t.Fatalf("Erro ao ler ticket_created para u3: %v", err)
	}
	if pMap, ok := c3Created.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 3 {
			t.Fatalf("Esperava posição 3 para u3, obteve %v", pMap["position"])
		}
	}

	// 5. u1 desconecta abruptamente!
	c1Conn.Close()

	// Operador deve receber queue_update com 2 itens
	var opUpdate1 models.WsMessage
	if err := opConn.ReadJSON(&opUpdate1); err != nil {
		t.Fatalf("Erro ao ler queue_update no operador após desconexão de u1: %v", err)
	}

	// u2 deve receber notificação de avanço para a posição 1
	var c2PosUpdate models.WsMessage
	if err := c2Conn.ReadJSON(&c2PosUpdate); err != nil {
		t.Fatalf("Erro ao ler queue_position no u2: %v", err)
	}
	if c2PosUpdate.Type != "queue_position" {
		t.Fatalf("Esperava tipo queue_position para u2, obteve %s", c2PosUpdate.Type)
	}
	if pMap, ok := c2PosUpdate.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 1 {
			t.Fatalf("Esperava nova posição 1 para u2, obteve %v", pMap["position"])
		}
	}

	// u3 deve receber notificação de avanço para a posição 2
	var c3PosUpdate models.WsMessage
	if err := c3Conn.ReadJSON(&c3PosUpdate); err != nil {
		t.Fatalf("Erro ao ler queue_position no u3: %v", err)
	}
	if c3PosUpdate.Type != "queue_position" {
		t.Fatalf("Esperava tipo queue_position para u3, obteve %s", c3PosUpdate.Type)
	}
	if pMap, ok := c3PosUpdate.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 2 {
			t.Fatalf("Esperava nova posição 2 para u3, obteve %v", pMap["position"])
		}
	}

	// Verifica fila no Redis: u1 foi removido, u2 está na pos 1, u3 está na pos 2
	qAfterU1, err := db.GetQueue(context.Background())
	if err != nil {
		t.Fatalf("Erro ao buscar fila no Redis: %v", err)
	}
	if len(qAfterU1) != 2 {
		t.Fatalf("Esperava 2 chamados na fila, obteve %d", len(qAfterU1))
	}
	if qAfterU1[0].ClientUser != u2 || qAfterU1[0].Position != 1 {
		t.Fatalf("Primeiro chamado deveria ser u2 com pos 1, obteve: %+v", qAfterU1[0])
	}
	if qAfterU1[1].ClientUser != u3 || qAfterU1[1].Position != 2 {
		t.Fatalf("Segundo chamado deveria ser u3 com pos 2, obteve: %+v", qAfterU1[1])
	}

	// 6. u2 também desconecta abruptamente!
	c2Conn.Close()

	// Operador recebe queue_update com 1 item
	_ = opConn.ReadJSON(&opUpdate1)

	// u3 deve receber notificação de avanço para a posição 1
	var c3PosFinal models.WsMessage
	if err := c3Conn.ReadJSON(&c3PosFinal); err != nil {
		t.Fatalf("Erro ao ler queue_position final no u3: %v", err)
	}
	if pMap, ok := c3PosFinal.Payload.(map[string]interface{}); ok {
		if int(pMap["position"].(float64)) != 1 {
			t.Fatalf("Esperava nova posição 1 para u3 após queda de u2, obteve %v", pMap["position"])
		}
	}

	// Verifica fila no Redis: apenas u3 na pos 1
	qFinal, err := db.GetQueue(context.Background())
	if err != nil {
		t.Fatalf("Erro ao consultar fila no Redis: %v", err)
	}
	if len(qFinal) != 1 || qFinal[0].ClientUser != u3 || qFinal[0].Position != 1 {
		t.Fatalf("Esperava apenas u3 na pos 1, obteve: %+v", qFinal)
	}
}

func TestWriteDeadlineBugAfterPing(t *testing.T) {
	hub, server := setupWSTest(t)
	defer server.Close()

	user := "DOMAIN\\testDeadlineUser"
	conn := connectWS(t, server.URL, user, false)
	defer conn.Close()

	// Aguarda 6 segundos para que o primeiro Ping dispare (em 5s) e defina o deadline para +2s (7s)
	// Às 7s o deadline do socket expira. Às 8s o socket tem deadline vencido.
	time.Sleep(8 * time.Second)

	// Agora o servidor tenta enviar uma mensagem para esse cliente
	hub.RouteMessage(models.WsMessage{
		Type: "test_msg",
		To:   user,
		Payload: map[string]string{"foo": "bar"},
	})

	// Lê a mensagem no cliente
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg models.WsMessage
	err := conn.ReadJSON(&msg)
	if err != nil {
		t.Fatalf("Falha esperada pelo bug de timeout no WriteJSON: %v", err)
	}
	if msg.Type != "test_msg" {
		t.Fatalf("Esperava tipo test_msg, obteve %s", msg.Type)
	}
}

func TestMultipleConnectionsSameUsername(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	user := "CORP\\185033"

	// 1. Conecta o Operador
	opConn := connectWS(t, server.URL, user, true)
	defer opConn.Close()

	var opMsg models.WsMessage
	if err := opConn.ReadJSON(&opMsg); err != nil {
		t.Fatalf("Erro ao ler queue inicial do operador: %v", err)
	}

	// 2. Conecta o Cliente com o MESMO username (ex: mesma máquina de dev)
	clientConn := connectWS(t, server.URL, user, false)
	defer clientConn.Close()

	// 3. Cliente solicita suporte
	reqMsg := models.WsMessage{
		Type: "request_support",
		From: user,
	}
	if err := clientConn.WriteJSON(reqMsg); err != nil {
		t.Fatalf("Erro ao enviar request_support do cliente: %v", err)
	}

	// 4. Operador DEVE receber a notificação de fila (queue_update)
	_ = opConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var opQueueUpdate models.WsMessage
	if err := opConn.ReadJSON(&opQueueUpdate); err != nil {
		t.Fatalf("Operador perdeu conexão ou não recebeu fila após cliente conectar com mesmo username: %v", err)
	}
	if opQueueUpdate.Type != "queue_update" {
		t.Fatalf("Esperava queue_update no operador, obteve %s", opQueueUpdate.Type)
	}
}

func TestSameUserClientOperatorSignaling(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	user := "CORP\\185033"

	// 1. Operador conecta
	opConn := connectWS(t, server.URL, user, true)
	defer opConn.Close()

	var opInit models.WsMessage
	_ = opConn.ReadJSON(&opInit) // Drena queue_update inicial

	// 2. Cliente conecta
	clientConn := connectWS(t, server.URL, user, false)
	defer clientConn.Close()

	// 3. Cliente solicita suporte
	_ = clientConn.WriteJSON(models.WsMessage{
		Type: "request_support",
		From: user,
		Payload: map[string]interface{}{
			"hostname": "MY-DEV-PC",
			"ip":       "127.0.0.1",
		},
	})

	// Operador recebe queue_update
	var opQueueMsg models.WsMessage
	if err := opConn.ReadJSON(&opQueueMsg); err != nil {
		t.Fatalf("Erro ao ler queue_update no operador: %v", err)
	}

	// Cliente recebe ticket_created
	var clTicketMsg models.WsMessage
	if err := clientConn.ReadJSON(&clTicketMsg); err != nil {
		t.Fatalf("Erro ao ler ticket_created no cliente: %v", err)
	}
	if clTicketMsg.Type != "ticket_created" {
		t.Fatalf("Esperava ticket_created no cliente, obteve %s", clTicketMsg.Type)
	}
	var ticketID string
	if pMap, ok := clTicketMsg.Payload.(map[string]interface{}); ok {
		ticketID = pMap["id"].(string)
	}

	// 4. Operador aceita o ticket
	_ = opConn.WriteJSON(models.WsMessage{
		Type: "accept_ticket",
		From: user,
		Payload: map[string]interface{}{
			"ticket_id":   ticketID,
			"client_user": user,
		},
	})

	// Operador recebe queue_update após aceite
	var opQueueAfter models.WsMessage
	_ = opConn.ReadJSON(&opQueueAfter)

	// Operador recebe confirmação de aceite
	var opAcceptConfirm models.WsMessage
	if err := opConn.ReadJSON(&opAcceptConfirm); err != nil {
		t.Fatalf("Erro ao ler ticket_accepted no operador: %v", err)
	}
	if opAcceptConfirm.Type != "ticket_accepted" {
		t.Fatalf("Esperava ticket_accepted no operador, obteve %s", opAcceptConfirm.Type)
	}

	// Cliente recebe notificação ticket_accepted
	var clAcceptMsg models.WsMessage
	if err := clientConn.ReadJSON(&clAcceptMsg); err != nil {
		t.Fatalf("Erro ao ler ticket_accepted no cliente: %v", err)
	}
	if clAcceptMsg.Type != "ticket_accepted" {
		t.Fatalf("Esperava ticket_accepted no cliente, obteve %s", clAcceptMsg.Type)
	}

	// 5. Cliente envia Offer WebRTC para o operador
	offerSDP := map[string]string{"sdp": "v=0\r\no=- local_offer"}
	_ = clientConn.WriteJSON(models.WsMessage{
		Type:    "offer",
		From:    user,
		To:      user,
		Payload: offerSDP,
	})

	// Operador DEVE receber a Offer
	var recOffer models.WsMessage
	if err := opConn.ReadJSON(&recOffer); err != nil {
		t.Fatalf("Erro ao receber offer no operador: %v", err)
	}
	if recOffer.Type != "offer" {
		t.Fatalf("Esperava offer no operador, obteve %s", recOffer.Type)
	}

	// Operador envia Answer WebRTC para o cliente
	answerSDP := map[string]string{"sdp": "v=0\r\no=- local_answer"}
	_ = opConn.WriteJSON(models.WsMessage{
		Type:    "answer",
		From:    user,
		To:      user,
		Payload: answerSDP,
	})

	// Cliente DEVE receber a Answer
	var recAnswer models.WsMessage
	if err := clientConn.ReadJSON(&recAnswer); err != nil {
		t.Fatalf("Erro ao receber answer no cliente: %v", err)
	}
	if recAnswer.Type != "answer" {
		t.Fatalf("Esperava answer no cliente, obteve %s", recAnswer.Type)
	}

	// 6. Operador encerra sessão
	_ = opConn.WriteJSON(models.WsMessage{
		Type: "session_closed",
		From: user,
		To:   user,
		Payload: map[string]interface{}{
			"ticket_id": ticketID,
		},
	})

	// Cliente recebe encerramento
	var recClosed models.WsMessage
	if err := clientConn.ReadJSON(&recClosed); err != nil {
		t.Fatalf("Erro ao ler session_closed no cliente: %v", err)
	}
	if recClosed.Type != "session_closed" {
		t.Fatalf("Esperava session_closed no cliente, obteve %s", recClosed.Type)
	}
}

func TestConcurrentTicketAcceptance(t *testing.T) {
	_, server := setupWSTest(t)
	defer server.Close()

	clientUser := "DOMAIN\\concurrentClient"
	op1User := "DOMAIN\\op1"
	op2User := "DOMAIN\\op2"

	op1Conn := connectWS(t, server.URL, op1User, true)
	defer op1Conn.Close()
	var initQ1 models.WsMessage
	_ = op1Conn.ReadJSON(&initQ1)

	op2Conn := connectWS(t, server.URL, op2User, true)
	defer op2Conn.Close()
	var initQ2 models.WsMessage
	_ = op2Conn.ReadJSON(&initQ2)

	clientConn := connectWS(t, server.URL, clientUser, false)
	defer clientConn.Close()

	// Cliente solicita suporte
	_ = clientConn.WriteJSON(models.WsMessage{
		Type: "request_support",
		From: clientUser,
		Payload: map[string]interface{}{
			"hostname": "CONCURRENT-PC",
			"ip":       "192.168.1.50",
		},
	})

	// Consome ticket_created no cliente
	var ticketCreated models.WsMessage
	_ = clientConn.ReadJSON(&ticketCreated)

	// Consome queue_update nos dois operadores
	var qUpdate1, qUpdate2 models.WsMessage
	_ = op1Conn.ReadJSON(&qUpdate1)
	_ = op2Conn.ReadJSON(&qUpdate2)

	var ticketID string
	if pMap, ok := ticketCreated.Payload.(map[string]interface{}); ok {
		ticketID, _ = pMap["id"].(string)
		if ticketID == "" {
			ticketID, _ = pMap["ticket_id"].(string)
		}
	}

	// Ambos os operadores tentam aceitar o mesmo ticket simultaneamente
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = op1Conn.WriteJSON(models.WsMessage{
			Type: "accept_ticket",
			From: op1User,
			Payload: map[string]interface{}{
				"ticket_id":   ticketID,
				"client_user": clientUser,
			},
		})
	}()
	go func() {
		defer wg.Done()
		_ = op2Conn.WriteJSON(models.WsMessage{
			Type: "accept_ticket",
			From: op2User,
			Payload: map[string]interface{}{
				"ticket_id":   ticketID,
				"client_user": clientUser,
			},
		})
	}()
	wg.Wait()

	// Coleta as mensagens recebidas por cada operador
	readOperatorResponses := func(conn *websocket.Conn) []string {
		var types []string
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		for {
			var msg models.WsMessage
			err := conn.ReadJSON(&msg)
			if err != nil {
				break
			}
			types = append(types, msg.Type)
		}
		return types
	}

	op1Msgs := readOperatorResponses(op1Conn)
	op2Msgs := readOperatorResponses(op2Conn)

	containsType := func(slice []string, val string) bool {
		for _, item := range slice {
			if item == val {
				return true
			}
		}
		return false
	}

	hasAccepted1 := containsType(op1Msgs, "ticket_accepted")
	hasFailed1 := containsType(op1Msgs, "ticket_accept_failed")

	hasAccepted2 := containsType(op2Msgs, "ticket_accepted")
	hasFailed2 := containsType(op2Msgs, "ticket_accept_failed")

	// Exatamente um deve ter aceito com sucesso e o outro deve ter falhado
	if (hasAccepted1 && hasAccepted2) || (!hasAccepted1 && !hasAccepted2) {
		t.Fatalf("Esperava que exatamente um operador vencesse a corrida. Op1: %v, Op2: %v", op1Msgs, op2Msgs)
	}

	if hasAccepted1 && !hasFailed2 {
		t.Fatalf("Op1 aceitou, mas Op2 não recebeu ticket_accept_failed: %v", op2Msgs)
	}
	if hasAccepted2 && !hasFailed1 {
		t.Fatalf("Op2 aceitou, mas Op1 não recebeu ticket_accept_failed: %v", op1Msgs)
	}
}



