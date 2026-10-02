package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	_ "zyron-server/docs"
	"zyron-server/internal/api"
	"zyron-server/internal/db"
	"zyron-server/internal/ws"
	"zyron-server/web"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title Zyron P2P Remote Support API
// @version 1.0
// @description Backend do servidor de sinalização WebRTC e gerenciamento de fila de suporte remoto P2P (Zyron Server).
// @termsOfService http://swagger.io/terms/

// @contact.name Suporte Zyron
// @contact.email suporte@zyron.local

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /

// healthzHandler godoc
// @Summary Checagem de integridade (Healthcheck)
// @Description Retorna o estado do servidor para orquestradores como Docker e Kubernetes.
// @Tags Sistema
// @Produce json
// @Success 200 {object} map[string]string "Servidor saudável"
// @Router /healthz [get]
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// wsHandler godoc
// @Summary Endpoint de Sinalização WebRTC e Fila P2P (WebSocket)
// @Description Abre o canal WebSocket para controle de presença, gerenciamento de fila e troca de sinalização WebRTC (SDP Offer/Answer e ICE Candidates).
// @Tags WebSocket
// @Param username query string true "Nome de usuário do cliente ou operador (ex: DOMINIO\\usuario)"
// @Param is_operator query bool false "Indica se a conexão é de um operador de suporte (true/false)"
// @Success 101 {string} string "Switching Protocols para WebSocket"
// @Failure 400 {string} string "Username required"
// @Router /ws [get]
func wsHandler(hub *ws.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, w, r)
	}
}

func main() {
	// Inicialização das conexões com Redis e PostgreSQL
	if err := db.InitRedis(); err != nil {
		log.Fatalf("Erro ao inicializar conexão com Redis: %v", err)
	}

	if err := db.InitPostgres(); err != nil {
		log.Fatalf("Erro ao inicializar conexão com PostgreSQL: %v", err)
	}

	hub := ws.NewHub()

	// Endpoints REST
	http.HandleFunc("/healthz", healthzHandler)
	http.HandleFunc("/api/auth/check-profile", api.CheckProfileHandler)
	http.HandleFunc("/api/queue/request", api.RequestSupportHandler(hub))
	http.HandleFunc("/api/queue/cancel", api.CancelSupportHandler(hub))
	http.HandleFunc("/api/queue/status", api.GetQueueStatusHandler())
	http.HandleFunc("/api/session/rate", api.RateSessionHandler)
	http.HandleFunc("/api/update/version", api.CheckUpdateHandler)
	http.HandleFunc("/api/update/download", api.DownloadUpdateHandler)
	http.HandleFunc("/downloads/zyron-client.exe", api.DownloadUpdateHandler)
	http.HandleFunc("/downloads/config.json", func(w http.ResponseWriter, r *http.Request) {
		cfgPath := filepath.Join(api.GetUpdatesDir(), "config.json")
		if _, err := os.Stat(cfgPath); err == nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			http.ServeFile(w, r, cfgPath)
			return
		}
		http.NotFound(w, r)
	})

	// Endpoint WebSocket
	http.HandleFunc("/ws", wsHandler(hub))

	// Endpoint Swagger UI
	http.HandleFunc("/swagger/", httpSwagger.WrapHandler)
	http.HandleFunc("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

	// Web Gerenciador
	http.HandleFunc("/manager", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(web.ManagerHTML)
	})
	http.HandleFunc("/manager/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/manager", http.StatusMovedPermanently)
	})

	// Endpoints REST do Gerenciador
	http.HandleFunc("/api/manager/login", api.ManagerLoginHandler)
	http.HandleFunc("/api/manager/logout", api.ManagerLogoutHandler)
	http.HandleFunc("/api/manager/check-auth", api.ManagerCheckAuthHandler)
	http.HandleFunc("/api/manager/realtime", api.ManagerRealtimeHandler(hub))
	http.HandleFunc("/api/manager/sessions", api.ManagerSessionsHandler)
	http.HandleFunc("/api/manager/analytics", api.ManagerAnalyticsHandler)
	http.HandleFunc("/api/manager/export", api.ManagerExportCSVHandler)

	// Redireciona a raiz para o Gerenciador
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/manager", http.StatusTemporaryRedirect)
			return
		}
		http.NotFound(w, r)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	} else if port[0] != ':' {
		port = ":" + port
	}

	log.Printf("Servidor Central P2P rodando na porta %s", port)
	log.Printf("Documentação Swagger disponível em: http://localhost%s/swagger/index.html", port)

	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.DefaultServeMux.ServeHTTP(w, r)
	})

	if err := http.ListenAndServe(port, corsHandler); err != nil {
		log.Fatal("Erro ao iniciar o servidor: ", err)
	}
}
