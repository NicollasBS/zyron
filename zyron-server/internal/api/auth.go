package api

import (
	"encoding/json"
	"log"
	"net/http"
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

var DefaultADService auth.ADService

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

	// Consulta o Active Directory para obter perfil completo e nome
	profile, err := DefaultADService.GetUserProfile(req.Username)
	if err != nil {
		log.Printf("Aviso na consulta ao Active Directory para '%s': %v", req.Username, err)
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
