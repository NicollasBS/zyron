package ws

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"zyron-server/internal/db"
	"zyron-server/internal/models"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// writeWait é o tempo limite permitido para escrita de uma mensagem no websocket.
	writeWait = 10 * time.Second

	// pongWait é o tempo limite de espera para recebimento de um pong do peer.
	pongWait = 60 * time.Second

	// pingPeriod é o intervalo de envio de pings para o peer (deve ser menor que pongWait).
	pingPeriod = 5 * time.Second
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	ID         string
	Hub        *Hub
	Conn       *websocket.Conn
	Username   string
	IsOperator bool
	RemoteIP   string
	send       chan models.WsMessage
	sendMu     sync.Mutex
	isClosed   bool
}

// Send envia uma mensagem para o canal de escrita do cliente de forma thread-safe e não bloqueante.
func (c *Client) Send(msg models.WsMessage) bool {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if c.isClosed {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		log.Printf("Canal de envio lotado para %s (msg tipo: %s)\n", c.Username, msg.Type)
		return false
	}
}

// Close fecha o canal de envio do cliente de maneira segura (idempotente).
func (c *Client) Close() {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if !c.isClosed {
		c.isClosed = true
		close(c.send)
	}
}

// ServeWS lida com upgrades de conexões WebSocket e registro de clientes.
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	username := r.URL.Query().Get("username")
	isOp := r.URL.Query().Get("is_operator") == "true"

	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	// Extrai o IP do cliente (suportando proxies reversos)
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP != "" {
		parts := strings.Split(clientIP, ",")
		clientIP = strings.TrimSpace(parts[0])
	}
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	if host, _, err := net.SplitHostPort(clientIP); err == nil {
		clientIP = host
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WS upgrade error:", err)
		return
	}

	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = uuid.New().String()
	}

	client := &Client{
		ID:         clientID,
		Hub:        hub,
		Conn:       conn,
		Username:   username,
		IsOperator: isOp,
		RemoteIP:   clientIP,
		send:       make(chan models.WsMessage, 256),
	}

	client.Hub.Register(client)

	go client.writePump()
	go client.readPump()
}

// ServerWS mantido para compatibilidade.
func ServerWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	ServeWS(hub, w, r)
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(1024 * 1024) // 1MB limite para mensagens WebRTC / SDP
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		// Renova TTL de 10s no Redis (Presença) a cada resposta de heartbeat Pong
		if err := db.RenewPresence(context.Background(), c.Username); err != nil {
			log.Printf("Erro ao renovar presença no Redis para %s: %v\n", c.Username, err)
		}
		return nil
	})

	for {
		var msg models.WsMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS unexpected close error (%s): %v", c.Username, err)
			}
			break
		}

		// Ao receber com sucesso qualquer mensagem do cliente, renova deadline e presença
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		_ = db.RenewPresence(context.Background(), c.Username)

		// Garante que a origem da mensagem seja o usuário autenticado
		if msg.From == "" {
			msg.From = c.Username
		}
		c.Hub.RouteMessage(msg, c)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteJSON(msg); err != nil {
				log.Printf("Erro ao escrever mensagem WS para %s: %v\n", c.Username, err)
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("Erro ao enviar Ping WS para %s: %v\n", c.Username, err)
				return
			}
		}
	}
}

