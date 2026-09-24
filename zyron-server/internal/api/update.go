package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// UpdateInfo representa as informações sobre a versão mais recente da aplicação
type UpdateInfo struct {
	Available    bool   `json:"available"`
	Version      string `json:"version"`
	Filename     string `json:"filename"`
	DownloadURL  string `json:"download_url"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256,omitempty"`
	ReleaseNotes string `json:"release_notes,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type versionConfigFile struct {
	Version      string `json:"version"`
	ReleaseNotes string `json:"release_notes"`
}

var (
	updateCacheMu   sync.RWMutex
	cachedSHA256    string
	cachedSize      int64
	cachedModTime   time.Time
	cachedFilePath  string
)

// GetUpdatesDir retorna o diretório onde os executáveis de atualização estão armazenados
func GetUpdatesDir() string {
	dir := os.Getenv("UPDATES_DIR")
	if dir != "" {
		return dir
	}
	// Padrão: pasta ./downloads relativa à raiz do servidor
	return "./downloads"
}

// GetUpdateExePath retorna o caminho completo para o executável do cliente
func GetUpdateExePath() string {
	dir := GetUpdatesDir()
	return filepath.Join(dir, "zyron-client.exe")
}

// computeFileSHA256 calcula o hash SHA-256 do arquivo com cache por data de modificação
func computeFileSHA256(filePath string) (string, int64, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", 0, err
	}

	updateCacheMu.RLock()
	if cachedFilePath == filePath && cachedModTime.Equal(fi.ModTime()) && cachedSize == fi.Size() && cachedSHA256 != "" {
		hash := cachedSHA256
		size := cachedSize
		updateCacheMu.RUnlock()
		return hash, size, nil
	}
	updateCacheMu.RUnlock()

	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", 0, err
	}

	hashStr := hex.EncodeToString(hasher.Sum(nil))

	updateCacheMu.Lock()
	cachedFilePath = filePath
	cachedModTime = fi.ModTime()
	cachedSize = fi.Size()
	cachedSHA256 = hashStr
	updateCacheMu.Unlock()

	return hashStr, fi.Size(), nil
}

// CheckUpdateHandler godoc
// @Summary Consulta a versão mais recente do Zyron Client disponível no servidor
// @Description Retorna os metadados da versão mais recente do executável (versão, tamanho, hash sha256 e URL de download).
// @Tags Atualizações
// @Produce json
// @Success 200 {object} UpdateInfo "Informações da versão disponível"
// @Router /api/update/version [get]
func CheckUpdateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	exePath := GetUpdateExePath()
	updatesDir := GetUpdatesDir()

	info := UpdateInfo{
		Available:    false,
		Version:      "1.0.0",
		Filename:     "zyron-client.exe",
		DownloadURL:  "/api/update/download",
		ReleaseNotes: "Versão oficial de atualização automática do Zyron Client.",
	}

	// 1. Verifica se existe version.json na pasta de downloads
	versionCfgPath := filepath.Join(updatesDir, "version.json")
	if vData, err := os.ReadFile(versionCfgPath); err == nil {
		var cfg versionConfigFile
		if err := json.Unmarshal(vData, &cfg); err == nil {
			if cfg.Version != "" {
				info.Version = strings.TrimSpace(cfg.Version)
			}
			if cfg.ReleaseNotes != "" {
				info.ReleaseNotes = strings.TrimSpace(cfg.ReleaseNotes)
			}
		}
	} else if envVer := os.Getenv("UPDATES_VERSION"); envVer != "" {
		info.Version = strings.TrimSpace(envVer)
	}

	// 2. Verifica se o executável existe no disco
	if fi, err := os.Stat(exePath); err == nil && !fi.IsDir() {
		info.Available = true
		info.Size = fi.Size()
		info.UpdatedAt = fi.ModTime().Format(time.RFC3339)

		if hash, _, err := computeFileSHA256(exePath); err == nil {
			info.SHA256 = hash
		}
	}

	_ = json.NewEncoder(w).Encode(info)
}

// DownloadUpdateHandler godoc
// @Summary Download do executável do Zyron Client para atualização
// @Description Faz o download binário do executável zyron-client.exe mais recente.
// @Tags Atualizações
// @Produce application/octet-stream
// @Success 200 {file} file "Binário zyron-client.exe"
// @Failure 404 {object} map[string]string "Arquivo não encontrado no servidor"
// @Router /api/update/download [get]
func DownloadUpdateHandler(w http.ResponseWriter, r *http.Request) {
	exePath := GetUpdateExePath()

	fi, err := os.Stat(exePath)
	if err != nil || fi.IsDir() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Nenhum executável de atualização encontrado em %s", exePath),
		})
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(exePath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	log.Printf("[Update] Servindo download do executável %s (%d bytes) para %s\n", exePath, fi.Size(), r.RemoteAddr)
	http.ServeFile(w, r, exePath)
}
