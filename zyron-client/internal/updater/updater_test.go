package updater_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"zyron-client/internal/updater"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		remote   string
		local    string
		expected bool
	}{
		{"1.0.1", "1.0.0", true},
		{"1.1.0", "1.0.9", true},
		{"2.0.0", "1.99.99", true},
		{"v1.0.2", "1.0.1", true},
		{"1.0.1", "v1.0.1", false},
		{"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.1", false},
		{"0.9.9", "1.0.0", false},
		{"1.0.0.1", "1.0.0", true},
		{"", "1.0.0", false},
	}

	for _, tt := range tests {
		got := updater.IsNewerVersion(tt.remote, tt.local)
		if got != tt.expected {
			t.Errorf("IsNewerVersion(%q, %q) = %v, esperava %v", tt.remote, tt.local, got, tt.expected)
		}
	}
}

func TestCheckForUpdate(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/update/version" {
			data := "dummy binary content"
			h := sha256.Sum256([]byte(data))
			hashStr := hex.EncodeToString(h[:])

			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"available":     true,
				"version":       "1.0.1",
				"filename":      "zyron-client.exe",
				"download_url":  "/api/update/download",
				"size":          len(data),
				"sha256":        hashStr,
				"release_notes": "Correções e melhorias",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer mockServer.Close()

	// 1. Testa com versão local antiga (1.0.0) -> Deve indicar atualização disponível
	res, err := updater.CheckForUpdate(mockServer.URL, "1.0.0")
	if err != nil {
		t.Fatalf("Erro ao checar atualização: %v", err)
	}
	if !res.UpdateAvailable {
		t.Errorf("Esperava UpdateAvailable=true para versão 1.0.0 vs 1.0.1")
	}
	if res.LatestVersion != "1.0.1" {
		t.Errorf("Esperava LatestVersion 1.0.1, obteve %s", res.LatestVersion)
	}

	// 2. Testa com versão local já atualizada (1.0.1) -> Não deve indicar atualização
	res2, err := updater.CheckForUpdate(mockServer.URL, "1.0.1")
	if err != nil {
		t.Fatalf("Erro ao checar atualização: %v", err)
	}
	if res2.UpdateAvailable {
		t.Errorf("Esperava UpdateAvailable=false para versão 1.0.1 vs 1.0.1")
	}
}
