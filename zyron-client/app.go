package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"zyron-client/internal/config"
	"zyron-client/internal/laser"
	"zyron-client/internal/system"
	"zyron-client/internal/updater"
	"zyron-client/internal/version"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct representa a instância da aplicação Wails
type App struct {
	ctx        context.Context
	isFloating bool
}

// NewApp cria uma nova instância do App
func NewApp() *App {
	return &App{}
}

// startup é chamado na inicialização do aplicativo Wails
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	updater.CleanupOldVersion()
	laser.GetLaserController() // Inicializa o controlador do ponteiro laser em background
}

// CheckProfileResponse representa a resposta da checagem de perfil
type CheckProfileResponse struct {
	IsOperator  bool   `json:"is_operator"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
}

// GetSystemInfo retorna as informações da estação de trabalho
func (a *App) GetSystemInfo() system.SystemInfo {
	return system.GetSystemInfo()
}

// GetConfig retorna a configuração salva (ex: URL do servidor)
func (a *App) GetConfig() config.Config {
	return config.GetConfig()
}

// SetServerURL atualiza e salva a URL do servidor
func (a *App) SetServerURL(serverURL string) (bool, string) {
	err := config.SetServerURL(serverURL)
	if err != nil {
		return false, err.Error()
	}
	return true, ""
}

// GetAppVersion retorna a versão atual compilada desta instância do Zyron Client
func (a *App) GetAppVersion() string {
	return version.CurrentVersion
}

// CheckForUpdate consulta o servidor para verificar se há versão mais recente disponível
func (a *App) CheckForUpdate(serverURL string) updater.UpdateCheckResult {
	if strings.TrimSpace(serverURL) == "" {
		serverURL = config.GetConfig().ServerURL
	}
	res, err := updater.CheckForUpdate(serverURL, version.CurrentVersion)
	if err != nil && res == nil {
		return updater.UpdateCheckResult{
			CurrentVersion: version.CurrentVersion,
			Error:          err.Error(),
		}
	}
	return *res
}

// ApplyUpdate baixa o novo executável do servidor, agenda a substituição atômica no Windows e reinicia
func (a *App) ApplyUpdate(serverURL string, downloadURL string, expectedSHA256 string) updater.UpdateApplyResult {
	if strings.TrimSpace(serverURL) == "" {
		serverURL = config.GetConfig().ServerURL
	}
	success, msg, err := updater.DownloadAndApplyUpdate(serverURL, downloadURL, expectedSHA256)
	if err != nil {
		return updater.UpdateApplyResult{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Aguarda 300ms para a resposta HTTP retornar ao frontend antes de encerrar o processo atual
	go func() {
		time.Sleep(300 * time.Millisecond)
		if a.ctx != nil {
			runtime.Quit(a.ctx)
		}
	}()

	return updater.UpdateApplyResult{
		Success: success,
		Message: msg,
	}
}

// CheckProfile consulta o servidor central para verificar se o usuário é Operador ou Cliente Comum
func (a *App) CheckProfile(serverURL string, username string) CheckProfileResponse {
	if strings.TrimSpace(serverURL) == "" {
		serverURL = config.GetConfig().ServerURL
	}
	serverURL = strings.TrimRight(serverURL, "/")

	if strings.TrimSpace(username) == "" {
		username = system.GetCurrentUser()
	}

	endpoint := fmt.Sprintf("%s/api/auth/check-profile", serverURL)

	reqPayload := map[string]string{
		"username": username,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return CheckProfileResponse{
			Username: username,
			Success:  false,
			Error:    fmt.Sprintf("Erro ao serializar requisição: %v", err),
		}
	}

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return CheckProfileResponse{
			Username: username,
			Success:  false,
			Error:    fmt.Sprintf("Falha ao conectar com servidor (%s): %v", endpoint, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return CheckProfileResponse{
			Username: username,
			Success:  false,
			Error:    fmt.Sprintf("Servidor retornou status %d: %s", resp.StatusCode, string(bodyBytes)),
		}
	}

	var authResp struct {
		IsOperator  bool   `json:"is_operator"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return CheckProfileResponse{
			Username: username,
			Success:  false,
			Error:    fmt.Sprintf("Erro ao decodificar resposta do servidor: %v", err),
		}
	}

	return CheckProfileResponse{
		IsOperator:  authResp.IsOperator,
		Username:    authResp.Username,
		DisplayName: authResp.DisplayName,
		Success:     true,
	}
}

// MinimizeWindow minimiza a janela da aplicação
func (a *App) MinimizeWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowMinimise(a.ctx)
}

// ToggleMaximizeWindow alterna entre maximizar e restaurar a janela
func (a *App) ToggleMaximizeWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowToggleMaximise(a.ctx)
}

// MaximizeWindow maximiza a janela da aplicação (tela cheia) de forma estável e suave
func (a *App) MaximizeWindow() {
	if a.ctx == nil {
		return
	}
	a.isFloating = false
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.WindowSetMinSize(a.ctx, 340, 220)
	if runtime.WindowIsMinimised(a.ctx) {
		runtime.WindowUnminimise(a.ctx)
	}
	runtime.WindowShow(a.ctx)
	if !runtime.WindowIsMaximised(a.ctx) {
		runtime.WindowMaximise(a.ctx)
	}
}

// RestoreFromFloatingToMaximized transita suavemente do modo flutuante direto para a tela cheia (maximizada)
func (a *App) RestoreFromFloatingToMaximized() {
	if a.ctx == nil {
		return
	}
	a.isFloating = false
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	runtime.WindowSetMinSize(a.ctx, 340, 220)
	if runtime.WindowIsMinimised(a.ctx) {
		runtime.WindowUnminimise(a.ctx)
	}
	runtime.WindowShow(a.ctx)
	if !runtime.WindowIsMaximised(a.ctx) {
		runtime.WindowMaximise(a.ctx)
	}
}

// IsWindowMaximised verifica se a janela está atualmente maximizada
func (a *App) IsWindowMaximised() bool {
	if a.ctx == nil {
		return false
	}
	return runtime.WindowIsMaximised(a.ctx)
}

// IsWindowMinimised verifica se a janela está atualmente minimizada
func (a *App) IsWindowMinimised() bool {
	if a.ctx == nil {
		return false
	}
	return runtime.WindowIsMinimised(a.ctx)
}

// CloseWindow encerra o aplicativo
func (a *App) CloseWindow() {
	if a.ctx == nil {
		return
	}
	runtime.Quit(a.ctx)
}

// RestoreAndFocusWindow restaura a janela caso minimizada e a traz para o primeiro plano temporariamente sem travar AlwaysOnTop
func (a *App) RestoreAndFocusWindow() {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "restore-window")
	if runtime.WindowIsMinimised(a.ctx) {
		runtime.WindowUnminimise(a.ctx)
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	go func() {
		time.Sleep(150 * time.Millisecond)
		if !a.isFloating {
			runtime.WindowSetAlwaysOnTop(a.ctx, false)
		}
	}()
}

// SetFloatingMode redimensiona e fixa a janela sobre as demais telas (estilo Google Meet)
func (a *App) SetFloatingMode(enable bool) {
	if a.ctx == nil {
		return
	}
	a.isFloating = enable
	if enable {
		runtime.WindowUnminimise(a.ctx)
		runtime.WindowUnmaximise(a.ctx)
		runtime.WindowShow(a.ctx)
		// Define o tamanho mínimo para permitir o widget compacto
		runtime.WindowSetMinSize(a.ctx, 320, 200)
		runtime.WindowSetSize(a.ctx, 380, 250)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)

		// Posiciona no canto inferior direito da tela principal
		screens, err := runtime.ScreenGetAll(a.ctx)
		if err == nil && len(screens) > 0 {
			primary := screens[0]
			for _, s := range screens {
				if s.IsPrimary {
					primary = s
					break
				}
			}
			w := primary.Size.Width
			if w == 0 {
				w = primary.Width
			}
			h := primary.Size.Height
			if h == 0 {
				h = primary.Height
			}
			if w == 0 {
				w = 1920
			}
			if h == 0 {
				h = 1080
			}
			targetX := w - 410
			targetY := h - 310
			if targetX < 0 {
				targetX = 50
			}
			if targetY < 0 {
				targetY = 50
			}
			runtime.WindowSetPosition(a.ctx, targetX, targetY)
		}
	} else {
		// Restaura tamanho normal e centraliza
		runtime.WindowSetAlwaysOnTop(a.ctx, false)
		runtime.WindowSetMinSize(a.ctx, 340, 220)
		runtime.WindowSetSize(a.ctx, 1100, 780)
		runtime.WindowCenter(a.ctx)
		runtime.WindowShow(a.ctx)
	}
}

// UpdateLaserPointer atualiza a posição e estado do ponteiro laser na tela real do cliente
func (a *App) UpdateLaserPointer(xPercent, yPercent float64, active, click bool) {
	laser.GetLaserController().UpdateLaser(xPercent, yPercent, active, click)
}

// HideLaserPointer oculta imediatamente o ponteiro laser
func (a *App) HideLaserPointer() {
	laser.GetLaserController().HideLaser()
}

