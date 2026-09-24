package models

type WsMessage struct {
	Type    string      `json:"type"`
	From    string      `json:"from"`
	To      string      `json:"to"`
	Payload interface{} `json:"payload"`
}

// RequestSupportPayload representa o payload enviado pelo cliente ao solicitar suporte.
type RequestSupportPayload struct {
	ID         string `json:"id,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	IP         string `json:"ip,omitempty"`
	ClientName string `json:"client_name,omitempty"`
}

// Ticket representa um chamado na fila de atendimento com suporte a IP e posição calculada na fila.
type Ticket struct {
	ID            string `json:"id"`
	TicketID      string `json:"ticket_id,omitempty"`
	ClientUser    string `json:"client_user"`
	ClientName    string `json:"client_name,omitempty"`
	Hostname      string `json:"hostname"`
	IP            string `json:"ip"`
	Status        string `json:"status"`
	Position      int    `json:"position"`
	QueuePosition int    `json:"queue_position,omitempty"`
}

