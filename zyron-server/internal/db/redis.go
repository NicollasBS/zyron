package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"zyron-server/internal/models"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	PresencePrefix = "presence:"
	PresenceTTL    = 90 * time.Second
)

var (
	QueueKey = "support:queue"
	RDB      *redis.Client

	// Script Lua para remoção 100% atômica por ID de chamado, garantindo que apenas um operador retire o ticket
	removeTicketScript = redis.NewScript(`
		local queueKey = KEYS[1]
		local targetID = ARGV[1]
		local items = redis.call('LRANGE', queueKey, 0, -1)
		for i, item in ipairs(items) do
			local ok, data = pcall(cjson.decode, item)
			if ok and data then
				if data.id == targetID or data.ticket_id == targetID then
					local removed = redis.call('LREM', queueKey, 1, item)
					if removed > 0 then
						return {item, i}
					end
				end
			end
		end
		return nil
	`)

	// Script Lua para remoção 100% atômica por username de cliente
	removeTicketByUserScript = redis.NewScript(`
		local queueKey = KEYS[1]
		local targetUser = ARGV[1]
		local items = redis.call('LRANGE', queueKey, 0, -1)
		for i, item in ipairs(items) do
			local ok, data = pcall(cjson.decode, item)
			if ok and data and data.client_user then
				if data.client_user == targetUser or string.lower(data.client_user) == string.lower(targetUser) then
					local removed = redis.call('LREM', queueKey, 1, item)
					if removed > 0 then
						return {item, i}
					end
				end
			end
		end
		return nil
	`)
)

// InitRedis inicializa o cliente Redis e testa a conexão via Ping.
func InitRedis() error {
	if RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		if err := RDB.Ping(ctx).Err(); err == nil {
			return nil
		}
	}

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	dbNum := 0
	if dbStr := os.Getenv("REDIS_DB"); dbStr != "" {
		if n, err := strconv.Atoi(dbStr); err == nil {
			dbNum = n
		}
	}

	passwordsToTry := []string{}
	if envPass, set := os.LookupEnv("REDIS_PASSWORD"); set {
		passwordsToTry = append(passwordsToTry, envPass)
	} else {
		// Tenta zyron_local_redis_password primeiro, depois postgres, depois sem senha
		passwordsToTry = append(passwordsToTry, "zyron_local_redis_password", "postgres", "")
	}

	var lastErr error
	for _, pass := range passwordsToTry {
		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: pass,
			DB:       dbNum,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := client.Ping(ctx).Result()
		cancel()

		if err == nil {
			RDB = client
			log.Printf("Conectado ao Redis em %s (DB: %d)", addr, dbNum)
			return nil
		}
		lastErr = err
		client.Close()
	}

	return fmt.Errorf("falha ao conectar ao Redis em %s: %w", addr, lastErr)
}

// SetPresence registra ou atualiza o status online de um usuário com TTL de 10 segundos.
func SetPresence(ctx context.Context, username string) error {
	if RDB == nil {
		return errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := PresencePrefix + username
	return RDB.Set(ctx, key, "online", PresenceTTL).Err()
}

// RenewPresence renova o TTL da chave de presença do usuário para 10 segundos.
func RenewPresence(ctx context.Context, username string) error {
	return SetPresence(ctx, username)
}

// RemovePresence remove a chave de presença do usuário no Redis.
func RemovePresence(ctx context.Context, username string) error {
	if RDB == nil {
		return errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := PresencePrefix + username
	return RDB.Del(ctx, key).Err()
}

// IsUserOnline verifica se a chave de presença do usuário existe no Redis.
func IsUserOnline(ctx context.Context, username string) (bool, error) {
	if RDB == nil {
		return false, errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	key := PresencePrefix + username
	count, err := RDB.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// PushTicket adiciona o chamado à fila somente se o usuário ainda não tiver um chamado pendente.
// Se já existir um chamado para o usuário na fila:
// - NÃO cria outro chamado
// - NÃO altera a posição do usuário na fila
// - Atualiza IP e Hostname caso informados na nova requisição
// - Retorna o chamado existente com sua posição atual (1-indexada) e created=false.
// Se não existir:
// - Adiciona o novo chamado no fim da fila com sua posição e created=true.
func PushTicket(ctx context.Context, ticket *models.Ticket) (*models.Ticket, int, bool, error) {
	if RDB == nil {
		return nil, 0, false, errors.New("redis client não inicializado")
	}
	if ticket == nil {
		return nil, 0, false, errors.New("ticket não pode ser nulo")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	items, err := RDB.LRange(ctx, QueueKey, 0, -1).Result()
	if err != nil {
		return nil, 0, false, err
	}

	// 1. Verifica se já existe um chamado ativo deste usuário na fila
	for i, item := range items {
		var existing models.Ticket
		if err := json.Unmarshal([]byte(item), &existing); err == nil {
			if existing.ClientUser == ticket.ClientUser {
				// Chamado já existe! Mantém exatamente a mesma posição na fila
				pos := i + 1
				existing.Position = pos
				existing.QueuePosition = pos
				updated := false
				if existing.TicketID == "" {
					existing.TicketID = existing.ID
					updated = true
				}
				if existing.CreatedAt.IsZero() {
					existing.CreatedAt = time.Now()
					updated = true
				}

				// Atualiza hostname ou IP se fornecidos na nova requisição
				if ticket.Hostname != "" && ticket.Hostname != existing.Hostname {
					existing.Hostname = ticket.Hostname
					updated = true
				}
				if ticket.IP != "" && ticket.IP != existing.IP {
					existing.IP = ticket.IP
					updated = true
				}
				if ticket.ClientName != "" && ticket.ClientName != existing.ClientName {
					existing.ClientName = ticket.ClientName
					updated = true
				}

				if updated {
					if updatedData, err := json.Marshal(existing); err == nil {
						_ = RDB.LSet(ctx, QueueKey, int64(i), string(updatedData)).Err()
					}
				}

				return &existing, pos, false, nil
			}
		}
	}

	// 2. Não possui chamado prévio: adiciona no final da fila
	if ticket.ID == "" {
		ticket.ID = uuid.New().String()
	}
	if ticket.TicketID == "" {
		ticket.TicketID = ticket.ID
	}
	if ticket.Status == "" {
		ticket.Status = "waiting"
	}
	if ticket.CreatedAt.IsZero() {
		ticket.CreatedAt = time.Now()
	}

	pos := len(items) + 1
	ticket.Position = pos
	ticket.QueuePosition = pos

	data, err := json.Marshal(ticket)
	if err != nil {
		return nil, 0, false, fmt.Errorf("erro ao serializar ticket: %w", err)
	}

	if err := RDB.RPush(ctx, QueueKey, string(data)).Err(); err != nil {
		return nil, 0, false, err
	}

	return ticket, pos, true, nil
}

// GetQueue lista todos os chamados presentes na fila do Redis com a posição de cada um calculada (1-indexada).
func GetQueue(ctx context.Context) ([]models.Ticket, error) {
	if RDB == nil {
		return nil, errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	items, err := RDB.LRange(ctx, QueueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	tickets := make([]models.Ticket, 0, len(items))
	for i, item := range items {
		var t models.Ticket
		if err := json.Unmarshal([]byte(item), &t); err == nil {
			t.Position = i + 1
			t.QueuePosition = i + 1
			if t.TicketID == "" {
				t.TicketID = t.ID
			}
			tickets = append(tickets, t)
		}
	}

	return tickets, nil
}

// PopTicket remove e retorna o primeiro chamado da fila do Redis.
func PopTicket(ctx context.Context) (*models.Ticket, error) {
	if RDB == nil {
		return nil, errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	val, err := RDB.LPop(ctx, QueueKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Fila vazia
		}
		return nil, err
	}

	var ticket models.Ticket
	if err := json.Unmarshal([]byte(val), &ticket); err != nil {
		return nil, fmt.Errorf("erro ao desserializar ticket: %w", err)
	}
	ticket.Position = 1
	ticket.QueuePosition = 1
	if ticket.TicketID == "" {
		ticket.TicketID = ticket.ID
	}

	return &ticket, nil
}

// RemoveTicket remove um chamado da fila pelo seu ID de forma 100% atômica no Redis.
func RemoveTicket(ctx context.Context, ticketID string) (*models.Ticket, error) {
	if RDB == nil {
		return nil, errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Tenta remoção atômica via Lua script
	res, err := removeTicketScript.Run(ctx, RDB, []string{QueueKey}, ticketID).Result()
	if err == nil {
		if arr, ok := res.([]interface{}); ok && len(arr) > 0 {
			if itemStr, ok := arr[0].(string); ok {
				var t models.Ticket
				if err := json.Unmarshal([]byte(itemStr), &t); err == nil {
					if len(arr) > 1 {
						if pos, ok := arr[1].(int64); ok {
							t.Position = int(pos)
							t.QueuePosition = int(pos)
						}
					}
					if t.TicketID == "" {
						t.TicketID = t.ID
					}
					return &t, nil
				}
			}
		}
		return nil, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil // Não encontrado ou já retirado por outro operador
	}

	// 2. Fallback caso scripts Lua não estejam habilitados no ambiente
	items, err := RDB.LRange(ctx, QueueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		var t models.Ticket
		if err := json.Unmarshal([]byte(item), &t); err == nil {
			if t.ID == ticketID || t.TicketID == ticketID {
				remCount, err := RDB.LRem(ctx, QueueKey, 1, item).Result()
				if err != nil {
					return nil, err
				}
				if remCount == 0 {
					// Já foi retirado concorrentemente por outro operador
					return nil, nil
				}
				t.Position = i + 1
				t.QueuePosition = i + 1
				if t.TicketID == "" {
					t.TicketID = t.ID
				}
				return &t, nil
			}
		}
	}

	return nil, nil
}

// RemoveTicketByUsername remove qualquer chamado de determinado usuário da fila do Redis de forma atômica.
func RemoveTicketByUsername(ctx context.Context, username string) (*models.Ticket, error) {
	if RDB == nil {
		return nil, errors.New("redis client não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	// 1. Tenta remoção atômica via Lua script
	res, err := removeTicketByUserScript.Run(ctx, RDB, []string{QueueKey}, username).Result()
	if err == nil {
		if arr, ok := res.([]interface{}); ok && len(arr) > 0 {
			if itemStr, ok := arr[0].(string); ok {
				var t models.Ticket
				if err := json.Unmarshal([]byte(itemStr), &t); err == nil {
					if len(arr) > 1 {
						if pos, ok := arr[1].(int64); ok {
							t.Position = int(pos)
							t.QueuePosition = int(pos)
						}
					}
					if t.TicketID == "" {
						t.TicketID = t.ID
					}
					return &t, nil
				}
			}
		}
		return nil, nil
	}
	if errors.Is(err, redis.Nil) {
		return nil, nil // Não encontrado ou já retirado
	}

	// 2. Fallback caso scripts Lua não estejam habilitados
	items, err := RDB.LRange(ctx, QueueKey, 0, -1).Result()
	if err != nil {
		return nil, err
	}

	for i, item := range items {
		var t models.Ticket
		if err := json.Unmarshal([]byte(item), &t); err == nil {
			if t.ClientUser == username {
				remCount, err := RDB.LRem(ctx, QueueKey, 1, item).Result()
				if err != nil {
					return nil, err
				}
				if remCount == 0 {
					// Já foi retirado concorrentemente por outro operador
					return nil, nil
				}
				t.Position = i + 1
				t.QueuePosition = i + 1
				if t.TicketID == "" {
					t.TicketID = t.ID
				}
				return &t, nil
			}
		}
	}

	return nil, nil
}

// GetQueuePosition retorna a posição atual (1-indexada) e o ticket do usuário na fila.
func GetQueuePosition(ctx context.Context, username string) (int, *models.Ticket, error) {
	tickets, err := GetQueue(ctx)
	if err != nil {
		return 0, nil, err
	}
	for _, t := range tickets {
		if t.ClientUser == username {
			return t.Position, &t, nil
		}
	}
	return 0, nil, nil
}

const (
	// AnalyticsCacheKey chave de armazenamento no Redis para o dashboard analítico
	AnalyticsCacheKey = "zyron:cache:dashboard_analytics"
	// AnalyticsCacheTTL tempo de vida do cache (60 segundos)
	AnalyticsCacheTTL = 60 * time.Second
)

// GetCachedAnalytics recupera o dashboard compilado do cache no Redis.
func GetCachedAnalytics(ctx context.Context) (*DashboardAnalytics, error) {
	if RDB == nil {
		return nil, errors.New("redis não inicializado")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	val, err := RDB.Get(ctx, AnalyticsCacheKey).Result()
	if err != nil {
		return nil, err
	}

	var analytics DashboardAnalytics
	if err := json.Unmarshal([]byte(val), &analytics); err != nil {
		return nil, err
	}
	return &analytics, nil
}

// SetCachedAnalytics armazena o dashboard analítico no Redis com TTL configurado.
func SetCachedAnalytics(ctx context.Context, analytics *DashboardAnalytics, ttl time.Duration) error {
	if RDB == nil || analytics == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ttl <= 0 {
		ttl = AnalyticsCacheTTL
	}

	data, err := json.Marshal(analytics)
	if err != nil {
		return err
	}

	return RDB.Set(ctx, AnalyticsCacheKey, string(data), ttl).Err()
}

// InvalidateAnalyticsCache invalida a chave de cache de métricas e analytics no Redis.
func InvalidateAnalyticsCache(ctx context.Context) error {
	if RDB == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return RDB.Del(ctx, AnalyticsCacheKey).Err()
}

