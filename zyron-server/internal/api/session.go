package api

import (
	"encoding/json"
	"net/http"
	"zyron-server/internal/db"
)

type RateSessionRequest struct {
	TicketID       string `json:"ticket_id"`
	ClientUsername string `json:"client_username"`
	Rating         int    `json:"rating"`
	Feedback       string `json:"feedback"`
}

// RateSessionHandler godoc
// @Summary Registrar avaliação do atendimento
// @Description Salva a nota (1 a 5 estrelas) e feedback deixado pelo cliente para o atendimento.
// @Tags Sessão
// @Accept json
// @Produce json
// @Param request body RateSessionRequest true "Dados da avaliação"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /api/session/rate [post]
func RateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		return
	}

	var req RateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Payload inválido: " + err.Error(),
		})
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		req.Rating = 5
	}

	if err := db.SaveSessionRating(req.TicketID, req.ClientUsername, req.Rating, req.Feedback); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Erro ao salvar avaliação: " + err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Avaliação registrada com sucesso",
	})
}
