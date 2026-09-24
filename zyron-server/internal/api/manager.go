package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"zyron-server/internal/db"
	"zyron-server/internal/ws"
)

// ManagerSession armazena a sessão ativa de um gerente autenticado.
type ManagerSession struct {
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	ExpiresAt   time.Time `json:"expires_at"`
}

var (
	managerSessions   = make(map[string]*ManagerSession)
	managerSessionsMu sync.RWMutex
)

func createManagerToken(username, displayName string) string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)

	managerSessionsMu.Lock()
	managerSessions[token] = &ManagerSession{
		Username:    username,
		DisplayName: displayName,
		ExpiresAt:   time.Now().Add(12 * time.Hour),
	}
	managerSessionsMu.Unlock()

	return token
}

func validateManagerToken(token string) *ManagerSession {
	if token == "" {
		return nil
	}
	managerSessionsMu.RLock()
	session, exists := managerSessions[token]
	managerSessionsMu.RUnlock()

	if !exists {
		return nil
	}
	if time.Now().After(session.ExpiresAt) {
		managerSessionsMu.Lock()
		delete(managerSessions, token)
		managerSessionsMu.Unlock()
		return nil
	}
	return session
}

func getSessionFromRequest(r *http.Request) *ManagerSession {
	// 1. Tenta por cookie
	if cookie, err := r.Cookie("zyron_manager_session"); err == nil && cookie.Value != "" {
		if s := validateManagerToken(cookie.Value); s != nil {
			return s
		}
	}
	// 2. Tenta por Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if s := validateManagerToken(token); s != nil {
			return s
		}
	}
	return nil
}

// ManagerLoginRequest payload para login no painel gerencial
type ManagerLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ManagerLoginHandler autentica o usuário no AD e valida se pertence a SG_GERENCIA_ATENDIMENTO
func ManagerLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req ManagerLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Requisição inválida", http.StatusBadRequest)
		return
	}

	profile, err := DefaultADService.AuthenticateUser(req.Username, req.Password)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	token := createManagerToken(profile.Username, profile.DisplayName)

	http.SetCookie(w, &http.Cookie{
		Name:     "zyron_manager_session",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(12 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"user": map[string]interface{}{
			"username":     profile.Username,
			"display_name": profile.DisplayName,
			"is_manager":   true,
		},
	})
}

// ManagerLogoutHandler encerra a sessão do gerente
func ManagerLogoutHandler(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("zyron_manager_session"); err == nil && cookie.Value != "" {
		managerSessionsMu.Lock()
		delete(managerSessions, cookie.Value)
		managerSessionsMu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "zyron_manager_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

// ManagerCheckAuthHandler verifica se a sessão atual é válida
func ManagerCheckAuthHandler(w http.ResponseWriter, r *http.Request) {
	session := getSessionFromRequest(r)
	if session == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"authenticated": false})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"user": map[string]interface{}{
			"username":     session.Username,
			"display_name": session.DisplayName,
		},
	})
}

// ManagerRealtimeHandler retorna os dados em tempo real da fila, operadores e chamadas ativas
func ManagerRealtimeHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := getSessionFromRequest(r)
		if session == nil {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		stats, err := hub.GetRealtimeManagerStats()
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao obter métricas em tempo real: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(stats)
	}
}

// ManagerSessionsHandler retorna histórico paginado e filtrado de chamados
func ManagerSessionsHandler(w http.ResponseWriter, r *http.Request) {
	session := getSessionFromRequest(r)
	if session == nil {
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	q := r.URL.Query()
	filter := db.SessionFilter{
		Search:   q.Get("search"),
		Operator: q.Get("operator"),
		Client:   q.Get("client"),
	}

	if rStr := q.Get("rating"); rStr != "" {
		if rVal, err := strconv.Atoi(rStr); err == nil && rVal >= 1 && rVal <= 5 {
			filter.Rating = rVal
		}
	}

	if lStr := q.Get("limit"); lStr != "" {
		if lVal, err := strconv.Atoi(lStr); err == nil && lVal > 0 {
			filter.Limit = lVal
		}
	}
	if filter.Limit == 0 {
		filter.Limit = 50
	}

	if oStr := q.Get("offset"); oStr != "" {
		if oVal, err := strconv.Atoi(oStr); err == nil && oVal >= 0 {
			filter.Offset = oVal
		}
	}

	if sDate := q.Get("start_date"); sDate != "" {
		if t, err := time.Parse("2006-01-02", sDate); err == nil {
			filter.StartDate = t
		}
	}
	if eDate := q.Get("end_date"); eDate != "" {
		if t, err := time.Parse("2006-01-02", eDate); err == nil {
			// Inclui o dia todo
			filter.EndDate = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}

	sessions, total, err := db.GetAllSessions(filter)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao consultar chamados: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": sessions,
		"total":    total,
		"limit":    filter.Limit,
		"offset":   filter.Offset,
	})
}

// ManagerAnalyticsHandler retorna estatísticas comparativas e dados para gráficos com suporte a cache Redis
func ManagerAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	session := getSessionFromRequest(r)
	if session == nil {
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	forceRefresh := r.URL.Query().Get("refresh") == "true" || r.URL.Query().Get("nocache") == "true"
	analytics, err := db.GetDashboardAnalytics(forceRefresh)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao compilar analytics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(analytics)
}

// ManagerExportCSVHandler exporta todos os chamados em formato CSV transmitindo linhas em streaming para alta performance
func ManagerExportCSVHandler(w http.ResponseWriter, r *http.Request) {
	session := getSessionFromRequest(r)
	if session == nil {
		http.Error(w, "Não autorizado", http.StatusUnauthorized)
		return
	}

	if db.DB == nil {
		http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
		return
	}

	rows, err := db.DB.Model(&db.SupportSession{}).Order("start_time DESC").Rows()
	if err != nil {
		http.Error(w, fmt.Sprintf("Erro ao consultar chamados para exportação: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	fileName := fmt.Sprintf("zyron_relatorio_chamados_%s.csv", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))

	// UTF-8 BOM para garantir correta abertura no Excel no Windows
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	// Cabeçalho CSV
	_, _ = fmt.Fprintln(w, "Ticket ID;Data Início;Data Fim;Duração (Segundos);Duração Formatada;Usuário Solicitante;Nome Solicitante;IP Solicitante;Usuário Operador;Nome Operador;Avaliação (Estrelas);Comentário")

	for rows.Next() {
		var s db.SupportSession
		if err := db.DB.ScanRows(rows, &s); err != nil {
			continue
		}

		durationFmt := fmt.Sprintf("%02d:%02d", s.DurationSeconds/60, s.DurationSeconds%60)
		feedbackClean := strings.ReplaceAll(s.Feedback, "\n", " ")
		feedbackClean = strings.ReplaceAll(feedbackClean, ";", ",")

		cName := s.ClientDisplayName
		if cName == "" {
			cName = s.ClientUsername
		}
		opName := s.OperatorDisplayName
		if opName == "" {
			opName = s.OperatorUsername
		}

		_, _ = fmt.Fprintf(w, "%s;%s;%s;%d;%s;%s;%s;%s;%s;%s;%d;%s\n",
			s.TicketID,
			s.StartTime.Format("02/01/2006 15:04:05"),
			s.EndTime.Format("02/01/2006 15:04:05"),
			s.DurationSeconds,
			durationFmt,
			s.ClientUsername,
			cName,
			s.ClientIP,
			s.OperatorUsername,
			opName,
			s.Rating,
			feedbackClean,
		)
	}
}
