package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"zyron-server/internal/auth"
)

type AuthRequest struct {
	Username string `json:"username"`
}

type AuthResponse struct {
	IsOperator  bool   `json:"is_operator"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
}

type cachedProfile struct {
	profile   *auth.UserProfile
	expiresAt time.Time
}

var (
	DefaultADService auth.ADService
	profileCache     = make(map[string]cachedProfile)
	profileCacheMu   sync.RWMutex
)

func init() {
	DefaultADService = auth.NewLDAPService(nil)
}

// CheckProfileHandler godoc
// @Summary Checa o perfil do usuário
// @Description Valida se o usuário informado possui perfil de Operador de Suporte ou Cliente comum via Active Directory.
// @Tags Autenticação
// @Accept json
// @Produce json
// @Param request body AuthRequest true "Dados do usuário"
// @Success 200 {object} AuthResponse
// @Failure 400 {string} string "Invalid request body"
// @Failure 405 {string} string "Method not allowed"
// @Router /api/auth/check-profile [post]
func CheckProfileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	usernameKey := strings.ToLower(strings.TrimSpace(req.Username))

	// 1. Verifica no cache em memória para evitar chamadas de rede repetidas ao Active Directory
	profileCacheMu.RLock()
	cached, found := profileCache[usernameKey]
	profileCacheMu.RUnlock()

	var profile *auth.UserProfile
	if found && time.Now().Before(cached.expiresAt) {
		profile = cached.profile
	} else {
		// 2. Consulta o Active Directory se não estiver em cache
		var err error
		profile, err = DefaultADService.GetUserProfile(req.Username)
		if err != nil {
			log.Printf("Aviso na consulta ao Active Directory para '%s': %v", req.Username, err)
		}

		if profile != nil {
			profileCacheMu.Lock()
			profileCache[usernameKey] = cachedProfile{
				profile:   profile,
				expiresAt: time.Now().Add(15 * time.Minute),
			}
			profileCacheMu.Unlock()
		}
	}

	var isOp bool
	displayName := req.Username
	if profile != nil {
		isOp = profile.IsOperator
		if profile.DisplayName != "" {
			displayName = profile.DisplayName
		}
	} else {
		isOp, _ = DefaultADService.CheckIsOperator(req.Username)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{
		IsOperator:  isOp,
		Username:    req.Username,
		DisplayName: displayName,
	})
}
