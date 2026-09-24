package db_test

import (
	"context"
	"testing"
	"time"
	"zyron-server/internal/db"
	"zyron-server/internal/models"

	"github.com/google/uuid"
)

func setupTestDB(t *testing.T) {
	if err := db.InitRedis(); err != nil {
		t.Fatalf("Erro ao conectar no Redis de teste: %v", err)
	}
	if err := db.InitPostgres(); err != nil {
		t.Fatalf("Erro ao conectar no Postgres de teste: %v", err)
	}
}

func TestRedisPresence(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()
	username := "DOMAIN\\testuser"

	// 1. SetPresence
	if err := db.SetPresence(ctx, username); err != nil {
		t.Fatalf("SetPresence falhou: %v", err)
	}

	online, err := db.IsUserOnline(ctx, username)
	if err != nil {
		t.Fatalf("IsUserOnline falhou: %v", err)
	}
	if !online {
		t.Fatalf("Esperava que o usuário %s estivesse online", username)
	}

	// 2. RenewPresence
	if err := db.RenewPresence(ctx, username); err != nil {
		t.Fatalf("RenewPresence falhou: %v", err)
	}

	// 3. RemovePresence
	if err := db.RemovePresence(ctx, username); err != nil {
		t.Fatalf("RemovePresence falhou: %v", err)
	}

	online, err = db.IsUserOnline(ctx, username)
	if err != nil {
		t.Fatalf("IsUserOnline após remoção falhou: %v", err)
	}
	if online {
		t.Fatalf("Esperava que o usuário %s não estivesse online após RemovePresence", username)
	}
}

func TestRedisQueue(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	oldKey := db.QueueKey
	db.QueueKey = "test:db:queue"
	defer func() {
		db.RDB.Del(ctx, db.QueueKey)
		db.QueueKey = oldKey
	}()

	// Limpa a fila antes do teste
	db.RDB.Del(ctx, db.QueueKey)

	ticket1 := &models.Ticket{
		ID:         uuid.New().String(),
		ClientUser: "DOMAIN\\user1",
		Hostname:   "WS-01",
		IP:         "10.0.0.11",
		Status:     "waiting",
	}

	ticket2 := &models.Ticket{
		ID:         uuid.New().String(),
		ClientUser: "DOMAIN\\user2",
		Hostname:   "WS-02",
		IP:         "10.0.0.12",
		Status:     "waiting",
	}

	// Push tickets
	res1, pos1, created1, err := db.PushTicket(ctx, ticket1)
	if err != nil || !created1 || pos1 != 1 {
		t.Fatalf("PushTicket 1 falhou: err=%v, created=%v, pos=%d", err, created1, pos1)
	}

	_, pos2, created2, err := db.PushTicket(ctx, ticket2)
	if err != nil || !created2 || pos2 != 2 {
		t.Fatalf("PushTicket 2 falhou: err=%v, created=%v, pos=%d", err, created2, pos2)
	}

	// Tentativa de duplicidade: deve manter ticket1 na mesma posição (1) e sem criar outro
	resDuplicate, posDup, createdDup, err := db.PushTicket(ctx, ticket1)
	if err != nil {
		t.Fatalf("PushTicket duplicado falhou: %v", err)
	}
	if createdDup {
		t.Fatalf("Esperava created=false para chamado duplicado")
	}
	if posDup != 1 {
		t.Fatalf("Esperava posição 1 mantida para chamado duplicado, obteve %d", posDup)
	}
	if resDuplicate.ID != res1.ID {
		t.Fatalf("Ticket ID alterado indevidamente na duplicidade: %s vs %s", resDuplicate.ID, res1.ID)
	}

	// GetQueue
	q, err := db.GetQueue(ctx)
	if err != nil {
		t.Fatalf("GetQueue falhou: %v", err)
	}
	if len(q) != 2 {
		t.Fatalf("Esperava 2 chamados na fila, obteve %d", len(q))
	}
	if q[0].ID != ticket1.ID || q[1].ID != ticket2.ID || q[0].IP != ticket1.IP {
		t.Fatalf("Chamados fora de ordem ou incorretos na fila: %+v", q)
	}
	if q[0].Position != 1 || q[1].Position != 2 {
		t.Fatalf("Posições incorretas na fila: pos0=%d, pos1=%d", q[0].Position, q[1].Position)
	}

	// RemoveTicketByUsername
	removed, err := db.RemoveTicketByUsername(ctx, ticket1.ClientUser)
	if err != nil {
		t.Fatalf("RemoveTicketByUsername falhou: %v", err)
	}
	if removed == nil || removed.ID != ticket1.ID {
		t.Fatalf("Esperava remover ticket1, obteve %+v", removed)
	}

	// PopTicket
	popped, err := db.PopTicket(ctx)
	if err != nil {
		t.Fatalf("PopTicket falhou: %v", err)
	}
	if popped == nil || popped.ID != ticket2.ID {
		t.Fatalf("Esperava retirar ticket2, obteve %+v", popped)
	}

	// Fila deve estar vazia
	emptyQueue, err := db.GetQueue(ctx)
	if err != nil {
		t.Fatalf("GetQueue falhou: %v", err)
	}
	if len(emptyQueue) != 0 {
		t.Fatalf("Esperava fila vazia, obteve %d itens", len(emptyQueue))
	}
}

func TestPostgresSessionSave(t *testing.T) {
	setupTestDB(t)

	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	session := &db.SupportSession{
		ID:               uuid.New(),
		TicketID:         "ticket-test-123",
		ClientUsername:   "DOMAIN\\testuser",
		OperatorUsername: "DOMAIN\\operator1",
		StartTime:        startTime,
		EndTime:          endTime,
		DurationSeconds:  300,
	}

	if err := db.SaveSessionRecord(session); err != nil {
		t.Fatalf("SaveSessionRecord falhou: %v", err)
	}

	var found db.SupportSession
	if err := db.DB.First(&found, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("Erro ao consultar sessão gravada no PostgreSQL: %v", err)
	}

	if found.TicketID != "ticket-test-123" || found.OperatorUsername != "DOMAIN\\operator1" {
		t.Fatalf("Dados gravados incorretos no PostgreSQL: %+v", found)
	}
}

func TestAnalyticsCaching(t *testing.T) {
	setupTestDB(t)
	ctx := context.Background()

	// 1. Limpa cache antes do teste
	if err := db.InvalidateAnalyticsCache(ctx); err != nil {
		t.Fatalf("InvalidateAnalyticsCache falhou: %v", err)
	}

	// 2. Chama GetDashboardAnalytics (deve calcular e popular cache)
	analytics1, err := db.GetDashboardAnalytics(false)
	if err != nil {
		t.Fatalf("GetDashboardAnalytics falhou: %v", err)
	}
	if analytics1 == nil {
		t.Fatalf("Analytics não pode ser nulo")
	}

	// Dá um pequeno tempo para a goroutine salvar no Redis
	time.Sleep(50 * time.Millisecond)

	// 3. Testa GetCachedAnalytics diretamente do Redis
	cached, err := db.GetCachedAnalytics(ctx)
	if err != nil || cached == nil {
		t.Fatalf("GetCachedAnalytics falhou: err=%v, cached=%+v", err, cached)
	}
	if cached.TotalSessions != analytics1.TotalSessions {
		t.Fatalf("TotalSessions em cache difere: %d vs %d", cached.TotalSessions, analytics1.TotalSessions)
	}

	// 4. Invalida cache e verifica se retorna erro ou nulo
	if err := db.InvalidateAnalyticsCache(ctx); err != nil {
		t.Fatalf("InvalidateAnalyticsCache falhou: %v", err)
	}

	cachedAfter, err := db.GetCachedAnalytics(ctx)
	if err == nil && cachedAfter != nil {
		t.Fatalf("Esperava erro ou cache nulo após invalidação, obteve: %+v", cachedAfter)
	}
}

