package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// UpdateCheckResult representa o resultado da verificação de atualização no servidor
type UpdateCheckResult struct {
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	DownloadURL     string `json:"download_url"`
	ReleaseNotes    string `json:"release_notes"`
	Size            int64  `json:"size"`
	SHA256          string `json:"sha256"`
	Error           string `json:"error,omitempty"`
}

// UpdateApplyResult representa o resultado do download e aplicação da atualização
type UpdateApplyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type serverUpdateResponse struct {
	Available    bool   `json:"available"`
	Version      string `json:"version"`
	Filename     string `json:"filename"`
	DownloadURL  string `json:"download_url"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	ReleaseNotes string `json:"release_notes"`
}

// IsNewerVersion compara duas versões semânticas (ex: "1.0.1" > "1.0.0").
// Retorna true se a versão remota for estritamente mais recente que a versão local atual.
func IsNewerVersion(remote, local string) bool {
	remote = strings.TrimPrefix(strings.TrimSpace(remote), "v")
	remote = strings.TrimPrefix(remote, "V")
	local = strings.TrimPrefix(strings.TrimSpace(local), "v")
	local = strings.TrimPrefix(local, "V")

	if remote == "" || local == "" {
		return false
	}

	remoteParts := strings.Split(remote, ".")
	localParts := strings.Split(local, ".")

	maxLen := len(remoteParts)
	if len(localParts) > maxLen {
		maxLen = len(localParts)
	}

	for i := 0; i < maxLen; i++ {
		rNum := 0
		if i < len(remoteParts) {
			rNum, _ = strconv.Atoi(remoteParts[i])
		}

		lNum := 0
		if i < len(localParts) {
			lNum, _ = strconv.Atoi(localParts[i])
		}

		if rNum > lNum {
			return true
		}
		if rNum < lNum {
			return false
		}
	}

	return false
}

// CheckForUpdate consulta o servidor central para verificar se há uma versão mais nova disponível
func CheckForUpdate(serverURL, currentVersion string) (*UpdateCheckResult, error) {
	serverURL = strings.TrimRight(strings.TrimSpace(serverURL), "/")
	if serverURL == "" {
		return &UpdateCheckResult{
			CurrentVersion: currentVersion,
			Error:          "URL do servidor não configurada",
		}, fmt.Errorf("URL do servidor não configurada")
	}

	endpoint := fmt.Sprintf("%s/api/update/version", serverURL)

	client := &http.Client{
		Timeout: 6 * time.Second,
	}

	resp, err := client.Get(endpoint)
	if err != nil {
		return &UpdateCheckResult{
			CurrentVersion: currentVersion,
			Error:          fmt.Sprintf("Falha ao consultar servidor: %v", err),
		}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &UpdateCheckResult{
			CurrentVersion: currentVersion,
			Error:          fmt.Sprintf("Servidor retornou status HTTP %d", resp.StatusCode),
		}, fmt.Errorf("status HTTP %d", resp.StatusCode)
	}

	var sResp serverUpdateResponse
	if err := json.NewDecoder(resp.Body).Decode(&sResp); err != nil {
		return &UpdateCheckResult{
			CurrentVersion: currentVersion,
			Error:          fmt.Sprintf("Erro ao interpretar resposta do servidor: %v", err),
		}, err
	}

	// Monta URL de download absoluta se recebida relativa
	downloadURL := sResp.DownloadURL
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		downloadURL = fmt.Sprintf("%s%s", serverURL, downloadURL)
	}

	isNewer := sResp.Available && IsNewerVersion(sResp.Version, currentVersion)

	return &UpdateCheckResult{
		UpdateAvailable: isNewer,
		CurrentVersion:  currentVersion,
		LatestVersion:   sResp.Version,
		DownloadURL:     downloadURL,
		ReleaseNotes:    sResp.ReleaseNotes,
		Size:            sResp.Size,
		SHA256:          sResp.SHA256,
	}, nil
}

// DownloadAndApplyUpdate faz o download do novo executável e agenda a substituição atômica no Windows
// CleanupOldVersion remove resquícios de arquivos .old deixados por atualizações anteriores
func CleanupOldVersion() {
	if currentExe, err := os.Executable(); err == nil {
		currentExe, _ = filepath.EvalSymlinks(currentExe)
		_ = os.Remove(currentExe + ".old")
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// DownloadAndApplyUpdate faz o download do novo executável e aplica substituição atômica no Windows sem scripts
func DownloadAndApplyUpdate(serverURL, downloadURL, expectedSHA256 string) (bool, string, error) {
	if strings.TrimSpace(downloadURL) == "" {
		downloadURL = fmt.Sprintf("%s/api/update/download", strings.TrimRight(serverURL, "/"))
	} else if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		downloadURL = fmt.Sprintf("%s%s", strings.TrimRight(serverURL, "/"), downloadURL)
	}

	// 1. Identifica o caminho do executável atual em execução
	currentExe, err := os.Executable()
	if err != nil {
		return false, "", fmt.Errorf("não foi possível identificar o caminho do executável atual: %w", err)
	}
	currentExe, _ = filepath.EvalSymlinks(currentExe)
	exeDir := filepath.Dir(currentExe)

	// 2. Prepara o arquivo para onde a nova versão será baixada
	newExe := filepath.Join(exeDir, fmt.Sprintf(".zyron-update-%d.new", time.Now().UnixNano()))
	oldExe := currentExe + ".old"

	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return false, "", fmt.Errorf("falha ao iniciar download da atualização: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("servidor retornou HTTP %d ao baixar atualização", resp.StatusCode)
	}

	out, err := os.OpenFile(newExe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		// Se não conseguir gravar na pasta do executável, tenta pasta temporária do sistema
		newExe = filepath.Join(os.TempDir(), fmt.Sprintf("zyron-update-%d.new", time.Now().UnixNano()))
		out, err = os.OpenFile(newExe, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return false, "", fmt.Errorf("não foi possível criar arquivo de atualização: %w", err)
		}
	}

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	copiedBytes, err := io.Copy(writer, resp.Body)
	_ = out.Close()

	if err != nil {
		_ = os.Remove(newExe)
		return false, "", fmt.Errorf("erro durante a gravação do download: %w", err)
	}

	if copiedBytes == 0 {
		_ = os.Remove(newExe)
		return false, "", fmt.Errorf("arquivo baixado está vazio (0 bytes)")
	}

	// 3. Validação de integridade SHA-256 se fornecido pelo servidor
	if expectedSHA256 != "" {
		calculatedHash := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(calculatedHash, expectedSHA256) {
			_ = os.Remove(newExe)
			return false, "", fmt.Errorf("falha de integridade: hash SHA-256 divergente (esperado %s, obtido %s)", expectedSHA256, calculatedHash)
		}
	}

	// 4. Substituição atômica 100% nativa sem scripts externos/bat (Compatível com Windows Defender):
	// O Windows NTFS permite renomear o arquivo do executável em execução (.exe -> .exe.old).
	// Em seguida, o novo executável (.new -> .exe) toma o seu lugar e é iniciado.
	_ = os.Remove(oldExe)

	if err := os.Rename(currentExe, oldExe); err != nil {
		_ = os.Remove(newExe)
		return false, "", fmt.Errorf("não foi possível renomear executável atual: %w", err)
	}

	if err := os.Rename(newExe, currentExe); err != nil {
		if copyErr := copyFile(newExe, currentExe); copyErr != nil {
			_ = os.Rename(oldExe, currentExe)
			_ = os.Remove(newExe)
			return false, "", fmt.Errorf("não foi possível aplicar a nova versão: %w", copyErr)
		}
		_ = os.Remove(newExe)
	}

	// 5. Inicia a nova versão diretamente pelo caminho original
	cmd := exec.Command(currentExe)
	cmd.Dir = exeDir
	if err := cmd.Start(); err != nil {
		_ = os.Remove(currentExe)
		_ = os.Rename(oldExe, currentExe)
		return false, "", fmt.Errorf("falha ao iniciar a nova versão: %w", err)
	}

	return true, "Download concluído com sucesso. Reiniciando com a nova versão...", nil
}
