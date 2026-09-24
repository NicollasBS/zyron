package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"zyron-server/internal/api"
	"zyron-server/internal/db"
	"zyron-server/internal/ws"
)

func setupTestEnvironment(t *testing.T) *ws.Hub {
	os.Unsetenv("AD_HOST")
	if err := db.InitRedis(); err != nil {
		t.Fatalf("Erro ao inicializar Redis de teste: %v", err)
	}
	if err := db.InitPostgres(); err != nil {
		t.Fatalf("Erro ao inicializar PostgreSQL de teste: %v", err)
	}
	return ws.NewHub()
}

func TestManagerAuthenticationAndEndpoints(t *testing.T) {
	hub := setupTestEnvironment(t)

	// 1. Login com usuário sem permissão
	unauthBody, _ := json.Marshal(map[string]string{
		"username": "CORP\\usuario.comum",
		"password": "senha",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/manager/login", bytes.NewReader(unauthBody))
	w := httptest.NewRecorder()
	api.ManagerLoginHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Esperava StatusUnauthorized (401), obteve %d", w.Code)
	}

	// 2. Login com usuário do grupo gerência
	authBody, _ := json.Marshal(map[string]string{
		"username": "CORP\\gestor_atendimento",
		"password": "senha",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/manager/login", bytes.NewReader(authBody))
	w = httptest.NewRecorder()
	api.ManagerLoginHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK (200), obteve %d (body: %s)", w.Code, w.Body.String())
	}

	var loginResp struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil || !loginResp.Success || loginResp.Token == "" {
		t.Fatalf("Falha ao obter token de login: %+v", loginResp)
	}

	token := loginResp.Token

	// 3. Teste CheckAuth
	req = httptest.NewRequest(http.MethodGet, "/api/manager/check-auth", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	api.ManagerCheckAuthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em check-auth, obteve %d", w.Code)
	}

	// 4. Teste Realtime Handler
	req = httptest.NewRequest(http.MethodGet, "/api/manager/realtime", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	api.ManagerRealtimeHandler(hub)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em realtime, obteve %d", w.Code)
	}

	// 5. Teste Sessions Handler
	req = httptest.NewRequest(http.MethodGet, "/api/manager/sessions?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	api.ManagerSessionsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em sessions, obteve %d", w.Code)
	}

	// 6. Teste Analytics Handler
	req = httptest.NewRequest(http.MethodGet, "/api/manager/analytics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	api.ManagerAnalyticsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em analytics, obteve %d", w.Code)
	}

	// 7. Teste Export CSV Handler
	req = httptest.NewRequest(http.MethodGet, "/api/manager/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	api.ManagerExportCSVHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em export, obteve %d", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("Ticket ID;Data Início")) {
		t.Fatalf("Esperava cabeçalho CSV no resultado da exportação")
	}

	// 8. Teste Logout Handler
	req = httptest.NewRequest(http.MethodPost, "/api/manager/logout", nil)
	req.AddCookie(&http.Cookie{Name: "zyron_manager_session", Value: token})
	w = httptest.NewRecorder()
	api.ManagerLogoutHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava StatusOK em logout, obteve %d", w.Code)
	}

	// 9. Após logout, requisição autenticada deve falhar
	req = httptest.NewRequest(http.MethodGet, "/api/manager/check-auth", nil)
	req.AddCookie(&http.Cookie{Name: "zyron_manager_session", Value: token})
	w = httptest.NewRecorder()
	api.ManagerCheckAuthHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Esperava StatusUnauthorized após logout, obteve %d", w.Code)
	}
}
