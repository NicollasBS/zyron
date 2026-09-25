package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// WsMessage representa a estrutura de mensagem de sinalização do Zyron
type WsMessage struct {
	Type    string      `json:"type"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Payload interface{} `json:"payload"`
}

// Configuração do teste
type LoadTestConfig struct {
	ServerAddr     string
	NumClients     int
	NumOperators   int
	Duration       time.Duration
	RequestRatio   float64
	SimulateMedia  bool
	PingInterval   time.Duration
	RampUpDuration time.Duration
	RateSessions   bool
}

// Métricas agregadas
type Metrics struct {
	connectedClients   int64
	connectedOperators int64
	connectionErrors   int64
	pingsSent          int64
	pongsReceived      int64
	ticketsRequested   int64
	ticketsCreated     int64
	ticketsAccepted    int64
	sessionsClosed     int64
	offersSent         int64
	answersSent        int64
	candidatesSent     int64
	messagesReceived   int64
	messagesSent       int64

	mu            sync.Mutex
	handshakeRTTs []time.Duration
	pingRTTs      []time.Duration
	queueWaitRTTs []time.Duration
}

func (m *Metrics) recordHandshake(d time.Duration) {
	m.mu.Lock()
	m.handshakeRTTs = append(m.handshakeRTTs, d)
	m.mu.Unlock()
}

func (m *Metrics) recordPingRTT(d time.Duration) {
	m.mu.Lock()
	m.pingRTTs = append(m.pingRTTs, d)
	m.mu.Unlock()
}

func (m *Metrics) recordQueueWait(d time.Duration) {
	m.mu.Lock()
	m.queueWaitRTTs = append(m.queueWaitRTTs, d)
	m.mu.Unlock()
}

func main() {
	cfg := LoadTestConfig{}

	flag.StringVar(&cfg.ServerAddr, "server", "localhost:8080", "Endereço host:port do zyron-server (ex: 127.0.0.1:8080 ou 192.168.1.100:8080)")
	flag.IntVar(&cfg.NumClients, "clients", 100, "Número de conexões simultâneas de clientes")
	flag.IntVar(&cfg.NumOperators, "ops", 5, "Número de conexões simultâneas de operadores")
	flag.DurationVar(&cfg.Duration, "duration", 60*time.Second, "Duração total do teste (ex: 30s, 2m, 10m)")
	flag.Float64Var(&cfg.RequestRatio, "req-ratio", 0.6, "Fração dos clientes que solicitarão atendimento (0.0 a 1.0)")
	flag.BoolVar(&cfg.SimulateMedia, "simulate-signaling", true, "Simular troca de sinalização WebRTC (Offer/Answer/Candidate) e fechamento de sessão")
	flag.DurationVar(&cfg.PingInterval, "ping-interval", 5*time.Second, "Intervalo de envio de ping (presença Redis)")
	flag.DurationVar(&cfg.RampUpDuration, "ramp-up", 5*time.Second, "Tempo de ramp-up gradual para estabelecer as conexões")
	flag.BoolVar(&cfg.RateSessions, "rate-sessions", true, "Enviar avaliação (1 a 5 estrelas) após fechar sessões atendidas")
	flag.Parse()

	cfg.ServerAddr = strings.TrimPrefix(cfg.ServerAddr, "http://")
	cfg.ServerAddr = strings.TrimPrefix(cfg.ServerAddr, "ws://")
	cfg.ServerAddr = strings.TrimRight(cfg.ServerAddr, "/")
	if !strings.Contains(cfg.ServerAddr, ":") {
		cfg.ServerAddr = cfg.ServerAddr + ":8080"
	}

	fmt.Println("================================================================================")
	fmt.Println("                 ZYRON SERVER - TESTE DE CARGA WEBSOCKET                        ")
	fmt.Println("================================================================================")
	fmt.Printf(" Servidor Alvo          : ws://%s/ws\n", cfg.ServerAddr)
	fmt.Printf(" Clientes Simultâneos   : %d\n", cfg.NumClients)
	fmt.Printf(" Operadores Simultâneos : %d\n", cfg.NumOperators)
	fmt.Printf(" Duração do Teste       : %v\n", cfg.Duration)
	fmt.Printf(" Taxa de Chamados       : %.0f%%\n", cfg.RequestRatio*100)
	fmt.Printf(" Simulação Sinalização  : %v\n", cfg.SimulateMedia)
	fmt.Printf(" Intervalo Heartbeat    : %v\n", cfg.PingInterval)
	fmt.Printf(" Ramp-up das Conexões   : %v\n", cfg.RampUpDuration)
	fmt.Println("================================================================================")

	metrics := &Metrics{}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	// Captura interrupção manual (Ctrl + C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n[!] Interrupção manual recebida (Ctrl+C). Encerrando conexões com elegância...")
		cancel()
	}()

	var wg sync.WaitGroup

	// Cálculo do intervalo de ramp-up entre novas conexões
	totalPeers := cfg.NumOperators + cfg.NumClients
	var delayPerConn time.Duration
	if cfg.RampUpDuration > 0 && totalPeers > 0 {
		delayPerConn = cfg.RampUpDuration / time.Duration(totalPeers)
	}

	startTime := time.Now()

	// 1. Conecta os Operadores
	for i := 1; i <= cfg.NumOperators; i++ {
		wg.Add(1)
		opID := i
		go func() {
			defer wg.Done()
			runVirtualOperator(ctx, opID, cfg, metrics)
		}()
		if delayPerConn > 0 {
			time.Sleep(delayPerConn)
		}
	}

	// 2. Conecta os Clientes
	for i := 1; i <= cfg.NumClients; i++ {
		wg.Add(1)
		clientID := i
		go func() {
			defer wg.Done()
			runVirtualClient(ctx, clientID, cfg, metrics)
		}()
		if delayPerConn > 0 {
			time.Sleep(delayPerConn)
		}
	}

	// 3. Monitor em tempo real no console
	go runDashboard(ctx, startTime, metrics)

	// Aguarda conclusão do teste
	<-ctx.Done()
	fmt.Println("\n[*] Finalizando conexões e coletando relatório final...")
	wg.Wait()

	printFinalSummary(time.Since(startTime), metrics)
}

func runVirtualClient(ctx context.Context, id int, cfg LoadTestConfig, m *Metrics) {
	username := fmt.Sprintf("BENCH_CLI_%05d", id)
	u := url.URL{
		Scheme:   "ws",
		Host:     cfg.ServerAddr,
		Path:     "/ws",
		RawQuery: fmt.Sprintf("username=%s&is_operator=false", url.QueryEscape(username)),
	}

	connectStart := time.Now()
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		atomic.AddInt64(&m.connectionErrors, 1)
		return
	}
	defer conn.Close()
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	m.recordHandshake(time.Since(connectStart))
	atomic.AddInt64(&m.connectedClients, 1)
	defer atomic.AddInt64(&m.connectedClients, -1)

	// Canais de controle locais
	sendChan := make(chan WsMessage, 32)
	var requestTime time.Time
	var activeTicketID string
	var currentOperator string

	// Rotina de escrita no WebSocket
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sendChan:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
				atomic.AddInt64(&m.messagesSent, 1)
			}
		}
	}()

	// Goroutine dedicada para envio periódico de Pings (não bloqueia com ReadJSON)
	var pingMu sync.Mutex
	var lastPingTime time.Time

	go func() {
		ticker := time.NewTicker(cfg.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingMu.Lock()
				lastPingTime = time.Now()
				pingMu.Unlock()
				atomic.AddInt64(&m.pingsSent, 1)
				select {
				case sendChan <- WsMessage{Type: "ping", From: username}:
				default:
				}
			}
		}
	}()

	// Decide se este cliente solicitará atendimento
	shouldRequest := rand.Float64() < cfg.RequestRatio
	if shouldRequest {
		// Atraso randômico pequeno para simular comportamento humano após conectar
		jitterWait := time.Duration(100+rand.Intn(2000)) * time.Millisecond
		go func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(jitterWait):
				requestTime = time.Now()
				atomic.AddInt64(&m.ticketsRequested, 1)
				sendChan <- WsMessage{
					Type: "request_support",
					From: username,
					Payload: map[string]interface{}{
						"hostname":    fmt.Sprintf("BENCH-HOST-%05d", id),
						"ip":          fmt.Sprintf("192.168.%d.%d", (id/250)%250+1, (id%250)+1),
						"client_name": fmt.Sprintf("Estação de Teste #%d", id),
					},
				}
			}
		}()
	}

	// Loop de Leitura contínuo
	for {
		var inMsg WsMessage
		// Deadline renovado com margem de segurança (4x intervalo de ping)
		_ = conn.SetReadDeadline(time.Now().Add(cfg.PingInterval * 4))
		if err := conn.ReadJSON(&inMsg); err != nil {
			if ctx.Err() == nil {
				atomic.AddInt64(&m.connectionErrors, 1)
			}
			return
		}

		atomic.AddInt64(&m.messagesReceived, 1)

		switch inMsg.Type {
		case "pong":
			atomic.AddInt64(&m.pongsReceived, 1)
			pingMu.Lock()
			if !lastPingTime.IsZero() {
				m.recordPingRTT(time.Since(lastPingTime))
			}
			pingMu.Unlock()

		case "ticket_created":
			atomic.AddInt64(&m.ticketsCreated, 1)
			if pMap, ok := inMsg.Payload.(map[string]interface{}); ok {
				if tID, ok := pMap["id"].(string); ok {
					activeTicketID = tID
				}
			}

		case "ticket_accepted":
			atomic.AddInt64(&m.ticketsAccepted, 1)
			if !requestTime.IsZero() {
				m.recordQueueWait(time.Since(requestTime))
			}
			if pMap, ok := inMsg.Payload.(map[string]interface{}); ok {
				if op, ok := pMap["operator"].(string); ok {
					currentOperator = op
				}
				if tID, ok := pMap["ticket_id"].(string); ok && activeTicketID == "" {
					activeTicketID = tID
				}
			}

			// Simulação de fluxo WebRTC: Cliente envia Oferta SDP ao Operador
			if cfg.SimulateMedia && currentOperator != "" {
				go func(opUser, tID string) {
					time.Sleep(150 * time.Millisecond)
					atomic.AddInt64(&m.offersSent, 1)
					sendChan <- WsMessage{
						Type: "offer",
						From: username,
						To:   opUser,
						Payload: map[string]interface{}{
							"type": "offer",
							"sdp":  "v=0\r\no=- 123456 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=sendonly\r\n",
						},
					}

					// Envia ICE Candidate
					time.Sleep(100 * time.Millisecond)
					atomic.AddInt64(&m.candidatesSent, 1)
					sendChan <- WsMessage{
						Type: "candidate",
						From: username,
						To:   opUser,
						Payload: map[string]interface{}{
							"candidate": "candidate:1 1 UDP 2130706431 192.168.1.100 50000 typ host",
						},
					}

					// Simula duração de chamada (ex: 1.5 a 3 segundos) e encerra sessão
					callDuration := time.Duration(1500+rand.Intn(1500)) * time.Millisecond
					select {
					case <-ctx.Done():
						return
					case <-time.After(callDuration):
						atomic.AddInt64(&m.sessionsClosed, 1)
						sendChan <- WsMessage{
							Type: "session_closed",
							From: username,
							To:   opUser,
							Payload: map[string]interface{}{
								"ticket_id": tID,
							},
						}

						// Envia avaliação opcional
						if cfg.RateSessions {
							time.Sleep(100 * time.Millisecond)
							sendChan <- WsMessage{
								Type: "rate_session",
								From: username,
								Payload: map[string]interface{}{
									"ticket_id": tID,
									"rating":    5,
									"feedback":  "Excelente atendimento automatizado",
								},
							}
						}
					}
				}(currentOperator, activeTicketID)
			}

		case "answer":
			// Resposta SDP recebida do operador
		case "candidate":
			// ICE Candidate recebido do operador
		case "peer_disconnected":
			// Conexão encerrada pela outra ponta
		}
	}
}

func runVirtualOperator(ctx context.Context, id int, cfg LoadTestConfig, m *Metrics) {
	username := fmt.Sprintf("BENCH_OP_%03d", id)
	u := url.URL{
		Scheme:   "ws",
		Host:     cfg.ServerAddr,
		Path:     "/ws",
		RawQuery: fmt.Sprintf("username=%s&is_operator=true", url.QueryEscape(username)),
	}

	connectStart := time.Now()
	conn, resp, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		atomic.AddInt64(&m.connectionErrors, 1)
		return
	}
	defer conn.Close()
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	m.recordHandshake(time.Since(connectStart))
	atomic.AddInt64(&m.connectedOperators, 1)
	defer atomic.AddInt64(&m.connectedOperators, -1)

	sendChan := make(chan WsMessage, 32)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sendChan:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := conn.WriteJSON(msg); err != nil {
					return
				}
				atomic.AddInt64(&m.messagesSent, 1)
			}
		}
	}()

	// Goroutine dedicada para Ping Heartbeat do Operador
	go func() {
		ticker := time.NewTicker(cfg.PingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				atomic.AddInt64(&m.pingsSent, 1)
				select {
				case sendChan <- WsMessage{Type: "ping", From: username}:
				default:
				}
			}
		}
	}()

	var isBusy bool
	var busyMu sync.Mutex

	for {
		var inMsg WsMessage
		// Deadline renovado com margem de segurança (4x intervalo de ping)
		_ = conn.SetReadDeadline(time.Now().Add(cfg.PingInterval * 4))
		if err := conn.ReadJSON(&inMsg); err != nil {
			if ctx.Err() == nil {
				atomic.AddInt64(&m.connectionErrors, 1)
			}
			return
		}

		atomic.AddInt64(&m.messagesReceived, 1)

		switch inMsg.Type {
		case "pong":
			atomic.AddInt64(&m.pongsReceived, 1)

		case "queue_update":
			// Se o operador não estiver ocupado, pega o próximo chamado da fila
			busyMu.Lock()
			if !isBusy {
				if queue, ok := inMsg.Payload.([]interface{}); ok && len(queue) > 0 {
					if firstTicket, ok := queue[0].(map[string]interface{}); ok {
						ticketID, _ := firstTicket["id"].(string)
						clientUser, _ := firstTicket["client_user"].(string)
						if ticketID != "" && clientUser != "" {
							isBusy = true
							sendChan <- WsMessage{
								Type: "accept_ticket",
								From: username,
								Payload: map[string]interface{}{
									"ticket_id":   ticketID,
									"client_user": clientUser,
								},
							}
						}
					}
				}
			}
			busyMu.Unlock()

		case "ticket_accept_failed":
			// Outro operador foi mais rápido no Redis, libera o lock
			busyMu.Lock()
			isBusy = false
			busyMu.Unlock()

		case "offer":
			// Recebeu oferta do cliente, responde com Answer SDP
			if cfg.SimulateMedia {
				clientTarget := inMsg.From
				go func() {
					time.Sleep(100 * time.Millisecond)
					atomic.AddInt64(&m.answersSent, 1)
					sendChan <- WsMessage{
						Type: "answer",
						From: username,
						To:   clientTarget,
						Payload: map[string]interface{}{
							"type": "answer",
							"sdp":  "v=0\r\no=- 654321 2 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=recvonly\r\n",
						},
					}
				}()
			}

		case "session_closed", "peer_disconnected":
			// Chamada finalizada, operador volta a ficar disponível
			busyMu.Lock()
			isBusy = false
			busyMu.Unlock()
		}
	}
}

func runDashboard(ctx context.Context, startTime time.Time, m *Metrics) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(startTime).Round(time.Second)
			clients := atomic.LoadInt64(&m.connectedClients)
			ops := atomic.LoadInt64(&m.connectedOperators)
			pongs := atomic.LoadInt64(&m.pongsReceived)
			created := atomic.LoadInt64(&m.ticketsCreated)
			accepted := atomic.LoadInt64(&m.ticketsAccepted)
			closed := atomic.LoadInt64(&m.sessionsClosed)
			errs := atomic.LoadInt64(&m.connectionErrors)
			totalSent := atomic.LoadInt64(&m.messagesSent)
			totalRecv := atomic.LoadInt64(&m.messagesReceived)

			fmt.Printf("\r[%02ds] Clientes: %d | Ops: %d | Chamados: Solicitados=%d, Aceitos=%d, Fechados=%d | Pongs: %d | Msgs/s: %d | Falhas: %d",
				int(elapsed.Seconds()),
				clients, ops,
				created, accepted, closed,
				pongs,
				(totalSent+totalRecv)/int64(elapsed.Seconds()+1),
				errs,
			)
		}
	}
}

func printFinalSummary(elapsed time.Duration, m *Metrics) {
	fmt.Println("\n")
	fmt.Println("================================================================================")
	fmt.Println("                          RELATÓRIO FINAL DO TESTE DE CARGA                     ")
	fmt.Println("================================================================================")
	fmt.Printf(" Tempo Total de Execução    : %v\n", elapsed.Round(time.Millisecond))
	fmt.Printf(" Conexões Ativas (Clientes) : %d\n", atomic.LoadInt64(&m.connectedClients))
	fmt.Printf(" Conexões Ativas (Operador) : %d\n", atomic.LoadInt64(&m.connectedOperators))
	fmt.Printf(" Erros de Conexão/Quedas    : %d\n", atomic.LoadInt64(&m.connectionErrors))
	fmt.Printf(" Pings Enviados / Pongs OK  : %d / %d\n", atomic.LoadInt64(&m.pingsSent), atomic.LoadInt64(&m.pongsReceived))
	fmt.Printf(" Chamados Solicitados       : %d\n", atomic.LoadInt64(&m.ticketsRequested))
	fmt.Printf(" Chamados Criados na Fila   : %d\n", atomic.LoadInt64(&m.ticketsCreated))
	fmt.Printf(" Chamados Aceitos (Ops)     : %d\n", atomic.LoadInt64(&m.ticketsAccepted))
	fmt.Printf(" Sessões Auditadas/Fechadas : %d\n", atomic.LoadInt64(&m.sessionsClosed))
	fmt.Printf(" Ofertas / Respostas SDP    : %d / %d\n", atomic.LoadInt64(&m.offersSent), atomic.LoadInt64(&m.answersSent))
	fmt.Printf(" Total de Mensagens (Sent)  : %d\n", atomic.LoadInt64(&m.messagesSent))
	fmt.Printf(" Total de Mensagens (Recv)  : %d\n", atomic.LoadInt64(&m.messagesReceived))
	fmt.Println("--------------------------------------------------------------------------------")

	printStats("Latência Handshake WebSocket (TCP + Upgrade)", m.handshakeRTTs)
	printStats("Latência RTT Heartbeat (Ping/Pong Redis)", m.pingRTTs)
	printStats("Tempo de Fila (Solicitação até Aceite)", m.queueWaitRTTs)

	fmt.Println("================================================================================")
}

func printStats(title string, samples []time.Duration) {
	if len(samples) == 0 {
		fmt.Printf(" %-44s: Sem dados\n", title)
		return
	}

	sort.Slice(samples, func(i, j int) bool {
		return samples[i] < samples[j]
	})

	var total time.Duration
	for _, d := range samples {
		total += d
	}
	avg := total / time.Duration(len(samples))
	min := samples[0]
	max := samples[len(samples)-1]
	p50 := samples[len(samples)*50/100]
	p95 := samples[len(samples)*95/100]
	p99 := samples[len(samples)*99/100]

	fmt.Printf(" %s:\n", title)
	fmt.Printf("   Amostras: %-6d | Mín: %-8v | Média: %-8v | Máx: %-8v\n", len(samples), min.Round(time.Millisecond), avg.Round(time.Millisecond), max.Round(time.Millisecond))
	fmt.Printf("   Percentis: p50=%-8v | p95=%-8v | p99=%-8v\n", p50.Round(time.Millisecond), p95.Round(time.Millisecond), p99.Round(time.Millisecond))
}
