package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"zyron-server/internal/api"
)

func TestCheckProfileHandler(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		expectedIsOp bool
	}{
		{
			name:         "Usuário comum",
			username:     "DOMAIN\\joao.silva",
			expectedIsOp: false,
		},
		{
			name:         "Operador suporte",
			username:     "DOMAIN\\maria.suporte",
			expectedIsOp: true,
		},
		{
			name:         "Operador admin",
			username:     "DOMAIN\\admin_ti",
			expectedIsOp: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(api.AuthRequest{Username: tt.username})
			req := httptest.NewRequest(http.MethodPost, "/api/auth/check-profile", bytes.NewReader(body))
			w := httptest.NewRecorder()

			api.CheckProfileHandler(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("Esperava status 200, obteve %d", w.Code)
			}

			var resp api.AuthResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("Erro ao decodificar resposta JSON: %v", err)
			}

			if resp.Username != tt.username {
				t.Errorf("Esperava username %s, obteve %s", tt.username, resp.Username)
			}
			if resp.IsOperator != tt.expectedIsOp {
				t.Errorf("Esperava IsOperator=%v, obteve %v", tt.expectedIsOp, resp.IsOperator)
			}
		})
	}
}
