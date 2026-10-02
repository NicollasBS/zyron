package db

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SupportSession representa o modelo de auditoria de sessões de suporte P2P.
type SupportSession struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	TicketID            string    `gorm:"type:varchar(100);index" json:"ticket_id"`
	ClientUsername      string    `gorm:"type:varchar(100);index" json:"client_username"`
	ClientDisplayName   string    `gorm:"type:varchar(150)" json:"client_display_name,omitempty"`
	OperatorUsername    string    `gorm:"type:varchar(100);index" json:"operator_username"`
	OperatorDisplayName string    `gorm:"type:varchar(150)" json:"operator_display_name,omitempty"`
	ClientIP            string    `gorm:"type:varchar(50)" json:"client_ip,omitempty"`
	StartTime           time.Time `gorm:"index" json:"start_time"`
	EndTime             time.Time `json:"end_time"`
	DurationSeconds     int64     `json:"duration_seconds"`
	WaitDurationSeconds int64     `json:"wait_duration_seconds"`
	Rating              int       `gorm:"index" json:"rating,omitempty"`
	Feedback            string    `gorm:"type:text" json:"feedback,omitempty"`
}

// BeforeCreate garante o preenchimento de UUID e duração se não definidos.
func (s *SupportSession) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.DurationSeconds == 0 && !s.EndTime.IsZero() && !s.StartTime.IsZero() {
		s.DurationSeconds = int64(s.EndTime.Sub(s.StartTime).Seconds())
	}
	return nil
}

var (
	DB          *gorm.DB
	migrateOnce sync.Once
)

// InitPostgres inicializa a conexão com o PostgreSQL e executa o AutoMigrate.
func InitPostgres() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil && sqlDB.Ping() == nil {
			return nil
		}
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "zyron_user"
		}
		password := os.Getenv("DB_PASSWORD")
		if password == "" {
			password = "zyron_local_password"
		}
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "zyron_db"
		}
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}
		tz := os.Getenv("DB_TIMEZONE")
		if tz == "" {
			tz = os.Getenv("TZ")
		}
		if tz == "" {
			tz = "America/Sao_Paulo"
		}
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
			host, user, password, dbname, port, sslmode, tz)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		tz := os.Getenv("DB_TIMEZONE")
		if tz == "" {
			tz = os.Getenv("TZ")
		}
		if tz == "" {
			tz = "America/Sao_Paulo"
		}
		// Se DB_NAME não foi definido explicitamente, tenta fallbacks locais (zyron, portal_db)
		if os.Getenv("DB_NAME") == "" && os.Getenv("DATABASE_URL") == "" {
			for _, fallbackDB := range []string{"zyron", "portal_db"} {
				fallbackDSN := fmt.Sprintf("host=localhost user=postgres password=postgres dbname=%s port=5432 sslmode=disable TimeZone=%s", fallbackDB, tz)
				DB, err = gorm.Open(postgres.Open(fallbackDSN), &gorm.Config{
					Logger: logger.Default.LogMode(logger.Warn),
				})
				if err == nil {
					break
				}
			}
		}
		if err != nil {
			return fmt.Errorf("falha ao conectar ao PostgreSQL: %w", err)
		}
	}

	// AutoMigrate da tabela de auditoria SupportSession (executado uma única vez)
	var migrateErr error
	migrateOnce.Do(func() {
		migrateErr = DB.AutoMigrate(&SupportSession{})
		if migrateErr != nil && (strings.Contains(migrateErr.Error(), "already exists") || strings.Contains(migrateErr.Error(), "42701")) {
			log.Printf("Aviso no AutoMigrate (coluna/relação já existe): %v", migrateErr)
			migrateErr = nil
		}
	})
	if migrateErr != nil {
		return fmt.Errorf("falha no AutoMigrate de SupportSession: %w", migrateErr)
	}

	// Otimização do pool de conexões para alta concorrência
	if sqlDB, err := DB.DB(); err == nil {
		sqlDB.SetMaxIdleConns(15)
		sqlDB.SetMaxOpenConns(50)
		sqlDB.SetConnMaxLifetime(1 * time.Hour)
	}

	// Índice composto para acelerar relatórios por operador e período
	_ = DB.Exec("CREATE INDEX IF NOT EXISTS idx_sessions_op_starttime ON support_sessions (operator_username, start_time DESC)").Error

	log.Println("PostgreSQL conectado e tabela de auditoria migrada com índices otimizados.")
	return nil
}

// SaveSessionRecord grava o registro de auditoria da sessão no banco de dados.
// É chamada exclusivamente quando o servidor WebSocket recebe a mensagem "session_closed".
func SaveSessionRecord(session *SupportSession) error {
	if DB == nil {
		return errors.New("banco de dados PostgreSQL não inicializado")
	}
	if session == nil {
		return errors.New("registro de sessão não pode ser nulo")
	}

	if session.ID == uuid.Nil {
		session.ID = uuid.New()
	}
	if session.EndTime.IsZero() {
		session.EndTime = time.Now()
	}
	if session.DurationSeconds == 0 && !session.StartTime.IsZero() {
		session.DurationSeconds = int64(session.EndTime.Sub(session.StartTime).Seconds())
	}

	err := DB.Create(session).Error
	if err == nil {
		_ = InvalidateAnalyticsCache(context.Background())
	}
	return err
}

// SaveSessionRating atualiza a avaliação (1 a 5 estrelas) e feedback da sessão no PostgreSQL.
func SaveSessionRating(ticketID string, clientUsername string, rating int, feedback string) error {
	if DB == nil {
		return errors.New("banco de dados PostgreSQL não inicializado")
	}

	var session SupportSession
	query := DB.Model(&SupportSession{})
	if ticketID != "" {
		query = query.Where("ticket_id = ?", ticketID)
	} else if clientUsername != "" {
		query = query.Where("client_username = ?", clientUsername).Order("end_time DESC")
	} else {
		return errors.New("ticket_id ou client_username é obrigatório para registrar avaliação")
	}

	err := query.First(&session).Error
	if err != nil {
		return err
	}

	err = DB.Model(&session).Updates(map[string]interface{}{
		"rating":   rating,
		"feedback": feedback,
	}).Error
	if err == nil {
		_ = InvalidateAnalyticsCache(context.Background())
	}
	return err
}

// SessionFilter define os parâmetros de busca e paginação para o gerenciador.
type SessionFilter struct {
	Search    string    `json:"search"`
	Operator  string    `json:"operator"`
	Client    string    `json:"client"`
	Rating    int       `json:"rating"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
}

// OperatorPerformance agrega métricas de produtividade por atendente.
type OperatorPerformance struct {
	OperatorUsername     string  `json:"operator_username"`
	OperatorDisplayName  string  `json:"operator_display_name"`
	TotalCalls           int64   `json:"total_calls"`
	AvgDurationSeconds   int64   `json:"avg_duration_seconds"`
	AvgWaitSeconds       int64   `json:"avg_wait_seconds"`
	TotalDurationSeconds int64   `json:"total_duration_seconds"`
	AvgRating            float64 `json:"avg_rating"`
	RatedCallsCount      int64   `json:"rated_calls_count"`
}

// DailyStat agrega volume de chamados por data.
type DailyStat struct {
	Date               string `json:"date"`
	TotalCalls         int64  `json:"total_calls"`
	AvgDurationSeconds int64  `json:"avg_duration_seconds"`
	AvgWaitSeconds     int64  `json:"avg_wait_seconds"`
}

// HourlyStat agrega volume de chamados por hora do dia (0 a 23).
type HourlyStat struct {
	Hour       int   `json:"hour"`
	TotalCalls int64 `json:"total_calls"`
}

// DashboardAnalytics compila os comparativos executivos da central de suporte.
type DashboardAnalytics struct {
	TotalSessions      int64                 `json:"total_sessions"`
	TodaySessions      int64                 `json:"today_sessions"`
	AvgDurationSeconds int64                 `json:"avg_duration_seconds"`
	AvgWaitSeconds     int64                 `json:"avg_wait_seconds"`
	AvgRating          float64               `json:"avg_rating"`
	RatedCallsCount    int64                 `json:"rated_calls_count"`
	RatingCounts       map[int]int64         `json:"rating_counts"`
	Operators          []OperatorPerformance `json:"operators"`
	DailyStats         []DailyStat           `json:"daily_stats"`
	HourlyDistribution []HourlyStat          `json:"hourly_distribution"`
}

// GetAllSessions retorna a lista paginada e filtrada de atendimentos para o gerenciador.
func GetAllSessions(filter SessionFilter) ([]SupportSession, int64, error) {
	if DB == nil {
		return nil, 0, errors.New("banco de dados PostgreSQL não inicializado")
	}

	query := DB.Model(&SupportSession{})

	if filter.Search != "" {
		s := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where(
			"LOWER(ticket_id) LIKE ? OR LOWER(client_username) LIKE ? OR LOWER(operator_username) LIKE ? OR LOWER(client_display_name) LIKE ? OR LOWER(operator_display_name) LIKE ? OR LOWER(client_ip) LIKE ?",
			s, s, s, s, s, s,
		)
	}

	if filter.Operator != "" {
		query = query.Where("operator_username = ? OR LOWER(operator_display_name) = ?", filter.Operator, strings.ToLower(filter.Operator))
	}

	if filter.Client != "" {
		query = query.Where("client_username = ? OR LOWER(client_display_name) = ?", filter.Client, strings.ToLower(filter.Client))
	}

	if filter.Rating > 0 {
		query = query.Where("rating = ?", filter.Rating)
	}

	if !filter.StartDate.IsZero() {
		query = query.Where("start_time >= ?", filter.StartDate)
	}

	if !filter.EndDate.IsZero() {
		query = query.Where("start_time <= ?", filter.EndDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	var sessions []SupportSession
	err := query.Order("start_time DESC").Limit(limit).Offset(filter.Offset).Find(&sessions).Error
	return sessions, total, err
}

// computeDashboardAnalytics executa as consultas analíticas de forma paralela no PostgreSQL.
func computeDashboardAnalytics() (*DashboardAnalytics, error) {
	analytics := &DashboardAnalytics{
		RatingCounts:       make(map[int]int64),
		Operators:          []OperatorPerformance{},
		DailyStats:         []DailyStat{},
		HourlyDistribution: make([]HourlyStat, 24),
	}
	for h := 0; h < 24; h++ {
		analytics.HourlyDistribution[h] = HourlyStat{Hour: h, TotalCalls: 0}
	}
	for r := 1; r <= 5; r++ {
		analytics.RatingCounts[r] = 0
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	now := time.Now()
	if loc, err := time.LoadLocation("America/Sao_Paulo"); err == nil {
		now = now.In(loc)
	}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	fourteenDaysAgo := now.AddDate(0, 0, -14)

	// 1. Total Geral e Chamados de Hoje (em paralelo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		var total, today int64
		DB.Model(&SupportSession{}).Count(&total)
		DB.Model(&SupportSession{}).Where("start_time >= ?", todayStart).Count(&today)
		mu.Lock()
		analytics.TotalSessions = total
		analytics.TodaySessions = today
		mu.Unlock()
	}()

	// 2. TMA e Média de Avaliação (em paralelo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		type SummaryResult struct {
			AvgDuration float64
			AvgWait     float64
			AvgRating   float64
			RatedCount  int64
		}
		var summary SummaryResult
		DB.Model(&SupportSession{}).Select(
			"COALESCE(AVG(duration_seconds), 0) as avg_duration, COALESCE(AVG(wait_duration_seconds), 0) as avg_wait, COALESCE(AVG(CASE WHEN rating > 0 THEN rating ELSE NULL END), 0) as avg_rating, COUNT(CASE WHEN rating > 0 THEN 1 ELSE NULL END) as rated_count",
		).Scan(&summary)
		mu.Lock()
		analytics.AvgDurationSeconds = int64(summary.AvgDuration)
		analytics.AvgWaitSeconds = int64(summary.AvgWait)
		analytics.AvgRating = math.Round(summary.AvgRating*10) / 10
		analytics.RatedCallsCount = summary.RatedCount
		mu.Unlock()
	}()

	// 3. Contagem por Estrelas (1 a 5)
	wg.Add(1)
	go func() {
		defer wg.Done()
		type RatingGroup struct {
			Rating int
			Count  int64
		}
		var ratingGroups []RatingGroup
		DB.Model(&SupportSession{}).Where("rating > 0").Select("rating, count(*) as count").Group("rating").Scan(&ratingGroups)
		mu.Lock()
		for _, rg := range ratingGroups {
			analytics.RatingCounts[rg.Rating] = rg.Count
		}
		mu.Unlock()
	}()

	// 4. Comparativo por Operador (em paralelo)
	wg.Add(1)
	go func() {
		defer wg.Done()
		type OpGroup struct {
			OperatorUsername    string
			OperatorDisplayName string
			TotalCalls          int64
			AvgDuration         float64
			AvgWait             float64
			TotalDuration       int64
			AvgRating           float64
			RatedCount          int64
		}
		var opGroups []OpGroup
		DB.Model(&SupportSession{}).Select(
			`operator_username,
			 COALESCE(MAX(operator_display_name), operator_username) as operator_display_name,
			 count(*) as total_calls,
			 COALESCE(AVG(duration_seconds), 0) as avg_duration,
			 COALESCE(AVG(wait_duration_seconds), 0) as avg_wait,
			 COALESCE(SUM(duration_seconds), 0) as total_duration,
			 COALESCE(AVG(CASE WHEN rating > 0 THEN rating ELSE NULL END), 0) as avg_rating,
			 COUNT(CASE WHEN rating > 0 THEN 1 ELSE NULL END) as rated_count`,
		).Group("operator_username").Order("total_calls DESC").Scan(&opGroups)

		var ops []OperatorPerformance
		for _, og := range opGroups {
			displayName := og.OperatorDisplayName
			if displayName == "" {
				displayName = og.OperatorUsername
			}
			ops = append(ops, OperatorPerformance{
				OperatorUsername:     og.OperatorUsername,
				OperatorDisplayName:  displayName,
				TotalCalls:           og.TotalCalls,
				AvgDurationSeconds:   int64(og.AvgDuration),
				AvgWaitSeconds:       int64(og.AvgWait),
				TotalDurationSeconds: og.TotalDuration,
				AvgRating:            math.Round(og.AvgRating*10) / 10,
				RatedCallsCount:      og.RatedCount,
			})
		}
		mu.Lock()
		analytics.Operators = ops
		mu.Unlock()
	}()

	// 5. Chamados por Dia (Últimos 14 dias)
	wg.Add(1)
	go func() {
		defer wg.Done()
		type DayGroup struct {
			DateStr     string
			Count       int64
			AvgDuration float64
			AvgWait     float64
		}
		var dayGroups []DayGroup
		DB.Model(&SupportSession{}).Where("start_time >= ?", fourteenDaysAgo).
			Select("TO_CHAR(start_time, 'YYYY-MM-DD') as date_str, count(*) as count, COALESCE(AVG(duration_seconds), 0) as avg_duration, COALESCE(AVG(wait_duration_seconds), 0) as avg_wait").
			Group("date_str").Order("date_str ASC").Scan(&dayGroups)

		var daily []DailyStat
		for _, dg := range dayGroups {
			daily = append(daily, DailyStat{
				Date:               dg.DateStr,
				TotalCalls:         dg.Count,
				AvgDurationSeconds: int64(dg.AvgDuration),
				AvgWaitSeconds:     int64(dg.AvgWait),
			})
		}
		mu.Lock()
		analytics.DailyStats = daily
		mu.Unlock()
	}()

	// 6. Distribuição por Hora do Dia
	wg.Add(1)
	go func() {
		defer wg.Done()
		type HourGroup struct {
			Hour  int
			Count int64
		}
		var hourGroups []HourGroup
		DB.Model(&SupportSession{}).
			Select("EXTRACT(HOUR FROM start_time)::int as hour, count(*) as count").
			Group("hour").Scan(&hourGroups)

		mu.Lock()
		for _, hg := range hourGroups {
			if hg.Hour >= 0 && hg.Hour < 24 {
				analytics.HourlyDistribution[hg.Hour].TotalCalls = hg.Count
			}
		}
		mu.Unlock()
	}()

	wg.Wait()
	return analytics, nil
}

// GetDashboardAnalytics compila estatísticas analíticas e comparativos de atendimento com caching Redis.
func GetDashboardAnalytics(forceRefresh ...bool) (*DashboardAnalytics, error) {
	if DB == nil {
		return nil, errors.New("banco de dados PostgreSQL não inicializado")
	}

	force := len(forceRefresh) > 0 && forceRefresh[0]
	ctx := context.Background()

	// 1. Tenta recuperar do cache Redis se não for forçado
	if !force && RDB != nil {
		if cached, err := GetCachedAnalytics(ctx); err == nil && cached != nil {
			return cached, nil
		}
	}

	// 2. Se for miss de cache ou forçado, calcula no banco
	analytics, err := computeDashboardAnalytics()
	if err != nil {
		return nil, err
	}

	// 3. Salva no cache do Redis de forma assíncrona
	if RDB != nil {
		go func(a *DashboardAnalytics) {
			_ = SetCachedAnalytics(context.Background(), a, AnalyticsCacheTTL)
		}(analytics)
	}

	return analytics, nil
}
