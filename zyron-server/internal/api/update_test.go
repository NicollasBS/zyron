package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"zyron-server/internal/api"
)

func TestUpdateEndpoints(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "zyron_updates_test_*")
	if err != nil {
		t.Fatalf("Erro ao criar tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	_ = os.Setenv("UPDATES_DIR", tempDir)
	defer os.Unsetenv("UPDATES_DIR")

	// 1. Testa sem arquivo executável disponível
	req := httptest.NewRequest("GET", "/api/update/version", nil)
	w := httptest.NewRecorder()
	api.CheckUpdateHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Esperava status 200, obteve %d", w.Code)
	}

	var info api.UpdateInfo
	if err := json.NewDecoder(w.Body).Decode(&info); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	if info.Available {
		t.Fatalf("Esperava available=false quando executável não existe")
	}

	// 2. Cria executável fictício e version.json
	dummyExe := filepath.Join(tempDir, "zyron-client.exe")
	if err := os.WriteFile(dummyExe, []byte("MZ_DUMMY_EXECUTABLE_BINARY_DATA_TEST_123"), 0755); err != nil {
		t.Fatalf("Erro ao criar dummy exe: %v", err)
	}

	versionJSON := filepath.Join(tempDir, "version.json")
	if err := os.WriteFile(versionJSON, []byte(`{"version":"1.0.1","release_notes":"Correções e melhorias"}`), 0644); err != nil {
		t.Fatalf("Erro ao criar version.json: %v", err)
	}

	// 3. Testa novamente /api/update/version
	req2 := httptest.NewRequest("GET", "/api/update/version", nil)
	w2 := httptest.NewRecorder()
	api.CheckUpdateHandler(w2, req2)

	var info2 api.UpdateInfo
	if err := json.NewDecoder(w2.Body).Decode(&info2); err != nil {
		t.Fatalf("Erro ao decodificar JSON: %v", err)
	}

	if !info2.Available {
		t.Fatalf("Esperava available=true após criar dummy exe")
	}
	if info2.Version != "1.0.1" {
		t.Errorf("Esperava version 1.0.1, obteve %s", info2.Version)
	}
	if info2.Size == 0 {
		t.Errorf("Esperava size > 0")
	}
	if info2.SHA256 == "" {
		t.Errorf("Esperava sha256 não vazio")
	}

	// 4. Testa download via /api/update/download
	reqDl := httptest.NewRequest("GET", "/api/update/download", nil)
	wDl := httptest.NewRecorder()
	api.DownloadUpdateHandler(wDl, reqDl)

	if wDl.Code != http.StatusOK {
		t.Fatalf("Esperava status 200 no download, obteve %d", wDl.Code)
	}
	if wDl.Header().Get("Content-Disposition") != `attachment; filename="zyron-client.exe"` {
		t.Errorf("Header Content-Disposition incorreto: %s", wDl.Header().Get("Content-Disposition"))
	}
	if wDl.Body.String() != "MZ_DUMMY_EXECUTABLE_BINARY_DATA_TEST_123" {
		t.Errorf("Conteúdo baixado incorreto: %s", wDl.Body.String())
	}
}
