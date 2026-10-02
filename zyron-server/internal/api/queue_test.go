package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"zyron-server/internal/api"
	"zyron-server/internal/db"
	"zyron-server/internal/ws"
)

func TestQueueHTTPHandlers(t *testing.T) {
	if err := db.InitRedis(); err != nil {
		t.Fatalf("Erro ao conectar no Redis: %v", err)
	}

	// Limpa fila antes do teste
	for {
		item, _ := db.PopTicket(context.Background())
		if item == nil {
			break
		}
	}

	hub := ws.NewHub()
	reqHandler := api.RequestSupportHandler(hub)
	statusHandler := api.GetQueueStatusHandler()
	cancelHandler := api.CancelSupportHandler(hub)

	testUser := "DOMAIN\\test.client"

	// 1. Consulta inicial: deve constar que não está na fila
	{
		req := httptest.NewRequest(http.MethodGet, "/api/queue/status?username="+testUser, nil)
		w := httptest.NewRecorder()
		statusHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperava status 200, obteve %d", w.Code)
		}
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["in_queue"] != false {
			t.Fatalf("Esperava in_queue=false, obteve %v", resp["in_queue"])
		}
	}

	// 2. Faz solicitação HTTP única para entrar na fila
	{
		body, _ := json.Marshal(api.SupportRequestBody{
			Username:   testUser,
			ClientName: "Test Client",
			Hostname:   "WS-TEST-01",
			IP:         "192.168.1.100",
		})
		req := httptest.NewRequest(http.MethodPost, "/api/queue/request", bytes.NewReader(body))
		w := httptest.NewRecorder()
		reqHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperava status 200, obteve %d (%s)", w.Code, w.Body.String())
		}
		var resp api.SupportResponseBody
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Erro ao decodificar JSON: %v", err)
		}
		if !resp.Success {
			t.Fatalf("Esperava success=true")
		}
		if resp.Position != 1 {
			t.Fatalf("Esperava posição 1 na fila vazia, obteve %d", resp.Position)
		}
		if resp.Ticket == nil || resp.Ticket.ClientUser != testUser {
			t.Fatalf("Dados do ticket incorretos: %+v", resp.Ticket)
		}
	}

	// 3. Consulta status após entrada na fila
	{
		req := httptest.NewRequest(http.MethodGet, "/api/queue/status?username="+testUser, nil)
		w := httptest.NewRecorder()
		statusHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperava status 200, obteve %d", w.Code)
		}
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["in_queue"] != true {
			t.Fatalf("Esperava in_queue=true, obteve %v", resp["in_queue"])
		}
		if int(resp["position"].(float64)) != 1 {
			t.Fatalf("Esperava position=1, obteve %v", resp["position"])
		}
	}

	// 4. Cancela chamado via HTTP
	{
		body, _ := json.Marshal(api.CancelRequestBody{Username: testUser})
		req := httptest.NewRequest(http.MethodPost, "/api/queue/cancel", bytes.NewReader(body))
		w := httptest.NewRecorder()
		cancelHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Esperava status 200, obteve %d", w.Code)
		}
		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["success"] != true {
			t.Fatalf("Esperava success=true no cancelamento")
		}
	}

	// 5. Consulta status final: deve constar que saiu da fila
	{
		req := httptest.NewRequest(http.MethodGet, "/api/queue/status?username="+testUser, nil)
		w := httptest.NewRecorder()
		statusHandler(w, req)

		var resp map[string]interface{}
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp["in_queue"] != false {
			t.Fatalf("Esperava in_queue=false após cancelamento, obteve %v", resp["in_queue"])
		}
	}
}
