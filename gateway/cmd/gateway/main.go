package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/spf13/viper"

	"github.com/synapse-oms/gateway/internal/adapter"
	_ "github.com/synapse-oms/gateway/internal/adapter/simulated"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
	"github.com/synapse-oms/gateway/internal/pipeline"
	"github.com/synapse-oms/gateway/internal/rest"
	"github.com/synapse-oms/gateway/internal/store"
	"github.com/synapse-oms/gateway/internal/ws"
	"github.com/synapse-oms/gateway/migrations"
)

func main() {
	// -------------------------------------------------------
	// 1. Load config via viper
	// -------------------------------------------------------
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("POSTGRES_URL", "postgres://localhost:5432/synapse")
	viper.SetDefault("REDIS_URL", "redis://localhost:6379")
	viper.AutomaticEnv()

	port := viper.GetString("PORT")
	postgresURL := viper.GetString("POSTGRES_URL")
	redisURL := viper.GetString("REDIS_URL")

	// -------------------------------------------------------
	// 2. Initialize structured JSON logging
	// -------------------------------------------------------
	logger := logging.New(os.Stdout, "gateway", "main")
	slog.SetDefault(logger)
	logger.Info("initializing gateway", slog.String("port", port))

	// Root context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// -------------------------------------------------------
	// 3. Connect to PostgreSQL and run migrations
	// -------------------------------------------------------
	logger.Info("connecting to PostgreSQL", slog.String("url", maskDSN(postgresURL)))

	poolCfg, err := pgxpool.ParseConfig(postgresURL)
	if err != nil {
		logger.Error("failed to parse PostgreSQL URL", slog.String("error", err.Error()))
		os.Exit(1)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		logger.Error("failed to connect to PostgreSQL", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	// Verify connectivity
	if err := pool.Ping(ctx); err != nil {
		logger.Error("PostgreSQL ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("PostgreSQL connected")

	// Run migrations
	logger.Info("running database migrations")
	if err := store.RunMigrations(ctx, pool, migrations.FS); err != nil {
		logger.Error("migration failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("migrations complete")

	// -------------------------------------------------------
	// 4. Connect to Redis (optional for Phase 1)
	// -------------------------------------------------------
	var redisClient *redis.Client
	logger.Info("connecting to Redis", slog.String("url", redisURL))

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Warn("failed to parse Redis URL, continuing without Redis",
			slog.String("error", err.Error()))
	} else {
		redisClient = redis.NewClient(opts)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			logger.Warn("Redis ping failed, continuing without Redis",
				slog.String("error", err.Error()))
			redisClient.Close()
			redisClient = nil
		} else {
			logger.Info("Redis connected")
			defer redisClient.Close()
		}
	}

	// -------------------------------------------------------
	// 5. Seed instruments if table is empty
	// -------------------------------------------------------
	pgStore := store.NewPostgresStore(pool)

	logger.Info("checking instruments table")
	instruments, err := pgStore.ListInstruments(ctx)
	if err != nil {
		logger.Error("failed to list instruments", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if len(instruments) == 0 {
		logger.Info("instruments table empty, seeding default instruments")
		if err := seedInstruments(ctx, pgStore, logger); err != nil {
			logger.Error("failed to seed instruments", slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("instruments seeded successfully")
	} else {
		logger.Info("instruments already exist, skipping seed",
			slog.Int("count", len(instruments)))
	}

	// -------------------------------------------------------
	// 6. Initialize simulated adapter and connect
	// -------------------------------------------------------
	logger.Info("initializing simulated exchange adapter")
	factory, ok := adapter.Get("simulated")
	if !ok {
		logger.Error("simulated adapter not registered")
		os.Exit(1)
	}
	venue := factory(nil)

	if err := venue.Connect(ctx); err != nil {
		logger.Error("failed to connect to simulated exchange",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		logger.Info("disconnecting from simulated exchange")
		_ = venue.Disconnect(context.Background())
	}()
	logger.Info("simulated exchange connected",
		slog.String("venue_id", venue.VenueID()))

	// -------------------------------------------------------
	// 7. Initialize WebSocket hub
	// -------------------------------------------------------
	logger.Info("initializing WebSocket hub")
	hub := ws.NewHub()
	wsSrv := ws.NewServer(hub)
	logger.Info("WebSocket hub ready")

	// -------------------------------------------------------
	// 8. Initialize pipeline
	// -------------------------------------------------------
	logger.Info("initializing order processing pipeline")
	p := pipeline.NewPipeline(pgStore, venue, hub)

	// -------------------------------------------------------
	// 9. Start pipeline goroutines
	// -------------------------------------------------------
	logger.Info("starting pipeline goroutines")
	p.Start(ctx)

	// -------------------------------------------------------
	// 10. Initialize REST router and mount WS upgrade endpoints
	// -------------------------------------------------------
	logger.Info("initializing REST router")
	router := rest.NewRouter(p, pgStore)

	// Mount WebSocket upgrade endpoints on the same mux.
	// rest.NewRouter returns an http.Handler (chi.Mux), so we wrap.
	mux := http.NewServeMux()
	mux.Handle("/", router)
	mux.HandleFunc("/ws/orders", wsSrv.HandleOrders)
	mux.HandleFunc("/ws/positions", wsSrv.HandlePositions)

	// -------------------------------------------------------
	// 11. Start HTTP server
	// -------------------------------------------------------
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("starting HTTP server", slog.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// -------------------------------------------------------
	// 12. Log "Gateway ready"
	// -------------------------------------------------------
	logger.Info("Gateway ready",
		slog.String("port", port),
		slog.String("venue", venue.VenueID()),
		slog.Bool("redis_connected", redisClient != nil),
	)

	// -------------------------------------------------------
	// Graceful shutdown
	// -------------------------------------------------------
	<-ctx.Done()
	logger.Info("shutdown signal received")

	// 1. Stop accepting new HTTP connections (10s timeout)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	logger.Info("shutting down HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP shutdown error", slog.String("error", err.Error()))
	}
	logger.Info("HTTP server stopped")

	// 2. Drain pipeline (wait up to 5s for goroutines)
	logger.Info("draining pipeline")
	pipelineDone := make(chan struct{})
	go func() {
		p.Wait()
		close(pipelineDone)
	}()
	select {
	case <-pipelineDone:
		logger.Info("pipeline drained")
	case <-time.After(5 * time.Second):
		logger.Warn("pipeline drain timed out after 5s")
	}

	// 3. Close DB pool (deferred above)
	logger.Info("closing PostgreSQL pool")
	// pool.Close() is deferred

	// 4. Close Redis (deferred above if connected)
	if redisClient != nil {
		logger.Info("closing Redis connection")
		// redisClient.Close() is deferred
	}

	logger.Info("gateway stopped")
}

// seedInstruments inserts the 6 default instruments into the database.
func seedInstruments(ctx context.Context, s *store.PostgresStore, logger *slog.Logger) error {
	instruments := []domain.Instrument{
		{
			ID:              "AAPL",
			Symbol:          "AAPL",
			Name:            "Apple Inc.",
			AssetClass:      domain.AssetClassEquity,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromInt(1),
			SettlementCycle: domain.SettlementT2,
			TradingHours: domain.TradingSchedule{
				MarketOpen:  "09:30",
				MarketClose: "16:00",
				PreMarket:   "04:00",
				AfterHours:  "20:00",
				Timezone:    "America/New_York",
			},
			Venues: []string{"simulated"},
		},
		{
			ID:              "MSFT",
			Symbol:          "MSFT",
			Name:            "Microsoft Corp.",
			AssetClass:      domain.AssetClassEquity,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromInt(1),
			SettlementCycle: domain.SettlementT2,
			TradingHours: domain.TradingSchedule{
				MarketOpen:  "09:30",
				MarketClose: "16:00",
				PreMarket:   "04:00",
				AfterHours:  "20:00",
				Timezone:    "America/New_York",
			},
			Venues: []string{"simulated"},
		},
		{
			ID:              "GOOG",
			Symbol:          "GOOG",
			Name:            "Alphabet Inc.",
			AssetClass:      domain.AssetClassEquity,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromInt(1),
			SettlementCycle: domain.SettlementT2,
			TradingHours: domain.TradingSchedule{
				MarketOpen:  "09:30",
				MarketClose: "16:00",
				PreMarket:   "04:00",
				AfterHours:  "20:00",
				Timezone:    "America/New_York",
			},
			Venues: []string{"simulated"},
		},
		{
			ID:              "BTC-USD",
			Symbol:          "BTC-USD",
			Name:            "Bitcoin",
			AssetClass:      domain.AssetClassCrypto,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromFloat(0.00001),
			SettlementCycle: domain.SettlementT0,
			TradingHours:    domain.TradingSchedule{Is24x7: true},
			Venues:          []string{"simulated"},
		},
		{
			ID:              "ETH-USD",
			Symbol:          "ETH-USD",
			Name:            "Ethereum",
			AssetClass:      domain.AssetClassCrypto,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromFloat(0.0001),
			SettlementCycle: domain.SettlementT0,
			TradingHours:    domain.TradingSchedule{Is24x7: true},
			Venues:          []string{"simulated"},
		},
		{
			ID:              "SOL-USD",
			Symbol:          "SOL-USD",
			Name:            "Solana",
			AssetClass:      domain.AssetClassCrypto,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromFloat(0.01),
			SettlementCycle: domain.SettlementT0,
			TradingHours:    domain.TradingSchedule{Is24x7: true},
			Venues:          []string{"simulated"},
		},
	}

	for _, inst := range instruments {
		if err := s.UpsertInstrument(ctx, &inst); err != nil {
			return fmt.Errorf("seeding instrument %s: %w", inst.ID, err)
		}
		logger.Info("seeded instrument",
			slog.String("id", inst.ID),
			slog.String("name", inst.Name),
		)
	}

	return nil
}

// maskDSN hides the password portion of a DSN for safe logging.
func maskDSN(dsn string) string {
	// Simple masking: just show it's configured
	if len(dsn) > 20 {
		return dsn[:20] + "..."
	}
	return dsn
}
