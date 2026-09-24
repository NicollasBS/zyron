Contexto do Sistema: Servidor de Sinalização (Suporte Remoto P2P)
Este repositório contém o backend em Golang para um sistema de suporte remoto P2P. O servidor atua como um roteador de sinalização WebRTC (SDP/ICE) e gerencia o estado da fila de atendimentos, permitindo que a conexão de mídia (tela/áudio) ocorra diretamente entre o ClientAPP (usuário) e o ClientAPP (operador).

A Fase 1 (Esqueleto WebSocket e Roteamento) foi concluída. Sua tarefa, como agente de IA, é implementar a Fase 2: Integração com Redis e PostgreSQL.

📂 Estrutura Atual do Projeto
Plaintext
suporte-p2p-server/
├── cmd/
│   └── server/
│       └── main.go           # Ponto de entrada (HTTP e WS)
├── internal/
│   ├── api/
│   │   └── auth.go           # Handler REST /api/auth/check-profile (Mockado)
│   ├── db/
│   │   ├── postgres.go       # [A IMPLEMENTAR] Conexão GORM e Migrations
│   │   └── redis.go          # [A IMPLEMENTAR] Conexão go-redis e funções de Fila/Presença
│   ├── models/
│   │   └── messages.go       # Structs WsMessage e Ticket
│   └── ws/
│       ├── client.go         # Leitura/Escrita do WebSocket e Pings
│       └── hub.go            # Registro de clientes e roteamento de sinalização WebRTC
├── go.mod
└── go.sum
🛠 Dependências Instaladas
[github.com/gorilla/websocket](https://github.com/gorilla/websocket) (Roteamento P2P)

[github.com/redis/go-redis/v9](https://github.com/redis/go-redis/v9) (Estado e Fila)

gorm.io/gorm e gorm.io/driver/postgres (Auditoria e Histórico)

🎯 Instruções para a IA (Próximas Tarefas)
Ao assumir o desenvolvimento deste código, você deve focar nas integrações de banco de dados, respeitando o ciclo de vida do chamado P2P. Siga os passos abaixo:

1. Implementar o Redis (internal/db/redis.go)
O Redis funcionará como a Máquina de Estados em memória.

Presença (TTL): Crie funções para registrar um usuário online com uma chave temporária (ex: presence:DOMINIO\usuario) que expira em 10 segundos. O heartbeat no arquivo internal/ws/client.go deverá chamar a função de renovar esse TTL no handler de Pong.

Fila de Chamados (List/Sorted Set): Crie funções para adicionar (PushTicket), listar (GetQueue) e remover (PopTicket) chamados.

Eventos: Quando um cliente entrar na fila, todos os operadores logados (via WS Hub) devem receber um broadcast de atualização da fila.

2. Implementar o PostgreSQL com GORM (internal/db/postgres.go)
O Postgres é exclusivo para auditoria. Ele não deve ser consultado para saber quem está online.

Modelo GORM: Crie a struct SupportSession contendo: ID (UUID), TicketID, ClientUsername, OperatorUsername, StartTime, EndTime, DurationSeconds.

Conexão: Implemente a função de inicialização e o AutoMigrate da tabela.

Gravação: Crie a função SaveSessionRecord que será chamada exclusivamente quando o servidor WebSocket receber uma mensagem do tipo session_closed.

3. Conectar os Módulos no Ciclo de Vida (ws/client.go e ws/hub.go)
Desconexões abruptas: Atualize o hub.Unregister para remover o usuário da fila do Redis caso a conexão WebSocket caia.

Roteamento aprimorado: Trate mensagens WebSocket específicas (ex: Type: "accept_ticket") para associar o operador ao cliente, remover da fila do Redis e notificar o cliente via WsMessage com o destinatário (To) correto para iniciar a oferta SDP do WebRTC.

4. Inicialização (cmd/server/main.go)
Adicione a inicialização dos pacotes db.InitRedis() e db.InitPostgres() antes de levantar os endpoints.

Regra Arquitetural Crítica: Não armazene ou trafegue os pacotes de áudio/vídeo pelo servidor em Go. O servidor existe única e exclusivamente para a troca inicial de sinalização P2P (JSON com SDP Offer/Answer e ICE Candidates). O fluxo binário ocorrerá de forma descentralizada na rede LAN após o handshake.