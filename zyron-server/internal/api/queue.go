package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
	"zyron-server/internal/db"
	"zyron-server/internal/models"
	"zyron-server/internal/ws"

	"github.com/google/uuid"
)

// SupportRequestBody define o payload de requisição HTTP para entrar na fila de suporte
type SupportRequestBody struct {
	Username   string `json:"username"`
	ClientName string `json:"client_name,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	IP         string `json:"ip,omitempty"`
}

// SupportResponseBody define a resposta enviada ao cliente com seu chamado e posição
type SupportResponseBody struct {
	Success  bool           `json:"success"`
	Ticket   *models.Ticket `json:"ticket,omitempty"`
	Position int            `json:"position,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// CancelRequestBody define o payload para cancelar um chamado pendente via HTTP
type CancelRequestBody struct {
	Username string `json:"username"`
}

// ExtractClientIP extrai o endereço IP real da requisição (considerando proxies reversos)
func ExtractClientIP(r *http.Request) string {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP != "" {
		parts := strings.Split(clientIP, ",")
		clientIP = strings.TrimSpace(parts[0])
	}
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	if host, _, err := net.SplitHostPort(clientIP); err == nil {
		clientIP = host
	}
	return clientIP
}

// RequestSupportHandler godoc
// @Summary Solicitação HTTP de entrada na fila de atendimento
// @Description Permite ao cliente comum solicitar suporte com apenas 1 requisição HTTP, sem precisar manter conexão WebSocket ociosa aberta.
// @Tags Fila
// @Accept json
// @Produce json
// @Param request body SupportRequestBody true "Dados do chamado"
// @Success 200 {object} SupportResponseBody
// @Failure 400 {object} SupportResponseBody
// @Failure 405 {string} string "Método não permitido"
// @Failure 500 {object} SupportResponseBody
// @Router /api/queue/request [post]
func RequestSupportHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(SupportResponseBody{
				Success: false,
				Error:   "Método não permitido. Utilize POST.",
			})
			return
		}

		var req SupportRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(SupportResponseBody{
				Success: false,
				Error:   "Corpo da requisição inválido: " + err.Error(),
			})
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(SupportResponseBody{
				Success: false,
				Error:   "O campo 'username' é obrigatório.",
			})
			return
		}

		// Preenche IP se não informado no corpo
		clientIP := strings.TrimSpace(req.IP)
		if clientIP == "" {
			clientIP = ExtractClientIP(r)
		}

		// Preenche nome de exibição caso não informado
		clientName := strings.TrimSpace(req.ClientName)
		if clientName == "" {
			if prof, err := DefaultADService.GetUserProfile(req.Username); err == nil && prof != nil && prof.DisplayName != "" {
				clientName = prof.DisplayName
			} else {
				clientName = req.Username
			}
		}

		ticketToProcess := &models.Ticket{
			ID:         uuid.New().String(),
			ClientUser: req.Username,
			ClientName: clientName,
			Hostname:   strings.TrimSpace(req.Hostname),
			IP:         clientIP,
			Status:     "waiting",
			CreatedAt:  time.Now(),
		}

		actualTicket, position, created, err := db.PushTicket(r.Context(), ticketToProcess)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(SupportResponseBody{
				Success: false,
				Error:   "Erro ao registrar chamado na fila: " + err.Error(),
			})
			return
		}

		// Se um novo chamado foi criado, notifica imediatamente os operadores conectados no WebSocket
		if created && hub != nil {
			hub.BroadcastQueue()
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(SupportResponseBody{
			Success:  true,
			Ticket:   actualTicket,
			Position: position,
		})
	}
}

// CancelSupportHandler godoc
// @Summary Cancelamento HTTP de chamado pendente na fila
// @Description Permite ao cliente cancelar seu pedido de suporte voluntariamente via HTTP.
// @Tags Fila
// @Accept json
// @Produce json
// @Param request body CancelRequestBody true "Dados do usuário"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 405 {string} string "Método não permitido"
// @Router /api/queue/cancel [post]
func CancelSupportHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Método não permitido. Utilize POST.",
			})
			return
		}

		var req CancelRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Corpo da requisição inválido: " + err.Error(),
			})
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "O campo 'username' é obrigatório.",
			})
			return
		}

		removed, err := db.RemoveTicketByUsername(r.Context(), req.Username)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Erro ao cancelar chamado: " + err.Error(),
			})
			return
		}

		if removed != nil && hub != nil {
			hub.BroadcastQueueAndNotifyClients()
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"ticket":  removed,
		})
	}
}

// GetQueueStatusHandler godoc
// @Summary Consulta HTTP da posição atual na fila
// @Description Permite consultar o status e a posição atual de um usuário na fila sem manter conexão persistente.
// @Tags Fila
// @Produce json
// @Param username query string true "Nome de usuário do cliente"
// @Success 200 {object} map[string]interface{}
// @Router /api/queue/status [get]
func GetQueueStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		username := strings.TrimSpace(r.URL.Query().Get("username"))
		if username == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Parâmetro 'username' é obrigatório.",
			})
			return
		}

		pos, ticket, err := db.GetQueuePosition(r.Context(), username)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Erro ao consultar fila: " + err.Error(),
			})
			return
		}

		if ticket == nil || pos == 0 {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success":  true,
				"in_queue": false,
				"position": 0,
			})
			return
		}

		ticket.Position = pos
		ticket.QueuePosition = pos
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success":  true,
			"in_queue": true,
			"position": pos,
			"ticket":   ticket,
		})
	}
}
