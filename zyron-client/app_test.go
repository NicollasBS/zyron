package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppCheckProfile(t *testing.T) {
	app := NewApp()

	// Cria servidor HTTP mock
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/check-profile" {
			t.Errorf("Path inesperado: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		var req struct {
			Username string `json:"username"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		w.Header().Set("Content-Type", "application/json")
		if req.Username == "CORP\\operator" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"is_operator": true,
				"username":   req.Username,
			})
		} else {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"is_operator": false,
				"username":   req.Username,
			})
		}
	}))
	defer ts.Close()

	// Testa operador
	respOp := app.CheckProfile(ts.URL, "CORP\\operator")
	if !respOp.Success || !respOp.IsOperator {
		t.Errorf("Esperava operador verdadeiro, obteve: %+v", respOp)
	}

	// Testa cliente comum
	respClient := app.CheckProfile(ts.URL, "CORP\\normaluser")
	if !respClient.Success || respClient.IsOperator {
		t.Errorf("Esperava cliente comum (is_operator false), obteve: %+v", respClient)
	}
}

func TestAppGetSystemInfo(t *testing.T) {
	app := NewApp()
	info := app.GetSystemInfo()
	if info.Username == "" {
		t.Errorf("Username não deve ser vazio")
	}
}

func TestAppConfigMethods(t *testing.T) {
	app := NewApp()
	cfg := app.GetConfig()
	if cfg.ServerURL == "" {
		t.Errorf("ServerURL inicial vazia")
	}
	if cfg.ServerIP == "" {
		t.Errorf("ServerIP inicial vazia")
	}

	// A edição via aplicativo deve ser estritamente bloqueada
	ok, errMsg := app.SetServerURL("http://localhost:9090")
	if ok {
		t.Errorf("SetServerURL deveria ter sido bloqueado, mas retornou sucesso")
	}
	if errMsg == "" {
		t.Errorf("Esperava mensagem de erro indicando que a alteração via app está desabilitada")
	}
}
