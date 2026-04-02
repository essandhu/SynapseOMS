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
	_ "github.com/synapse-oms/gateway/internal/adapter/alpaca"
	_ "github.com/synapse-oms/gateway/internal/adapter/binance"
	_ "github.com/synapse-oms/gateway/internal/adapter/simulated"
	"github.com/synapse-oms/gateway/internal/credential"
	"github.com/synapse-oms/gateway/internal/crossing"
	"github.com/synapse-oms/gateway/internal/domain"
	riskgrpc "github.com/synapse-oms/gateway/internal/grpc"
	"github.com/synapse-oms/gateway/internal/kafka"
	"github.com/synapse-oms/gateway/internal/logging"
	"github.com/synapse-oms/gateway/internal/pipeline"
	"github.com/synapse-oms/gateway/internal/rest"
	"github.com/synapse-oms/gateway/internal/router"
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
	viper.SetDefault("KAFKA_BROKERS", "")
	viper.SetDefault("RISK_ENGINE_GRPC", "")
	viper.SetDefault("SYNAPSE_MASTER_PASSPHRASE", "")
	viper.SetDefault("ML_SCORER_URL", "http://localhost:8090")
	viper.AutomaticEnv()

	port := viper.GetString("PORT")
	postgresURL := viper.GetString("POSTGRES_URL")
	redisURL := viper.GetString("REDIS_URL")
	kafkaBrokers := viper.GetString("KAFKA_BROKERS")
	riskEngineAddr := viper.GetString("RISK_ENGINE_GRPC")
	masterPassphrase := viper.GetString("SYNAPSE_MASTER_PASSPHRASE")
	mlScorerURL := viper.GetString("ML_SCORER_URL")

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
	// 6. Initialize all registered adapters
	// -------------------------------------------------------
	logger.Info("initializing venue adapters")
	var venues []adapter.LiquidityProvider

	for venueType, factory := range adapter.All() {
		logger.Info("creating adapter instance", slog.String("venue_type", venueType))
		venue := factory(nil)
		adapter.RegisterInstance(venue.VenueID(), venue)
		venues = append(venues, venue)
		logger.Info("adapter instance registered",
			slog.String("venue_id", venue.VenueID()),
			slog.String("venue_name", venue.VenueName()),
		)
	}

	if len(venues) == 0 {
		logger.Error("no venue adapters registered")
		os.Exit(1)
	}

	// Auto-connect the simulated adapter (no credentials needed)
	if simVenue, ok := adapter.GetInstance("sim-exchange"); ok {
		if err := simVenue.Connect(ctx, domain.VenueCredential{}); err != nil {
			logger.Error("failed to connect to simulated exchange",
				slog.String("error", err.Error()))
			os.Exit(1)
		}
		logger.Info("simulated exchange connected",
			slog.String("venue_id", simVenue.VenueID()))
	}

	defer func() {
		logger.Info("disconnecting venue adapters")
		for _, v := range venues {
			if v.Status() == adapter.Connected {
				_ = v.Disconnect(context.Background())
			}
		}
	}()

	logger.Info("venue adapters initialized",
		slog.Int("count", len(venues)),
	)

	// -------------------------------------------------------
	// 7. Initialize Kafka producer (optional)
	// -------------------------------------------------------
	var kafkaProducer *kafka.Producer
	var pipelineOpts []pipeline.Option

	if kafkaBrokers != "" {
		logger.Info("initializing Kafka producer", slog.String("brokers", kafkaBrokers))
		kafkaLogger := logging.New(os.Stdout, "gateway", "kafka")
		kp, err := kafka.NewProducer(kafkaBrokers, kafkaLogger)
		if err != nil {
			logger.Error("failed to create Kafka producer", slog.String("error", err.Error()))
			os.Exit(1)
		}
		kafkaProducer = kp
		defer kafkaProducer.Close()
		pipelineOpts = append(pipelineOpts, pipeline.WithKafkaPublisher(kafkaProducer))
		logger.Info("Kafka producer initialized")
	} else {
		logger.Info("KAFKA_BROKERS not set, running without Kafka")
	}

	// -------------------------------------------------------
	// 8. Initialize gRPC risk client
	// -------------------------------------------------------
	var riskClient riskgrpc.RiskClient

	if riskEngineAddr != "" {
		logger.Info("initializing risk engine client", slog.String("address", riskEngineAddr))
		rc, err := riskgrpc.NewRiskClient(riskEngineAddr)
		if err != nil {
			logger.Error("failed to create risk client", slog.String("error", err.Error()))
			os.Exit(1)
		}
		riskClient = rc
		defer riskClient.Close()
		logger.Info("risk engine client initialized")
	} else {
		logger.Info("RISK_ENGINE_GRPC not set, using fail-open risk client")
		riskClient = riskgrpc.NewFailOpenRiskClient()
	}

	// -------------------------------------------------------
	// 9. Initialize credential manager (optional)
	// -------------------------------------------------------
	var credMgr *credential.CredentialManager

	if masterPassphrase != "" {
		logger.Info("initializing credential manager")
		cm, err := credential.NewCredentialManager(masterPassphrase, pgStore)
		if err != nil {
			logger.Error("failed to create credential manager",
				slog.String("error", err.Error()))
			os.Exit(1)
		}
		credMgr = cm
		logger.Info("credential manager initialized")
	} else {
		logger.Warn("SYNAPSE_MASTER_PASSPHRASE not set, credential management disabled")
	}

	// -------------------------------------------------------
	// 10. Initialize WebSocket hub
	// -------------------------------------------------------
	logger.Info("initializing WebSocket hub")
	hub := ws.NewHub()
	wsSrv := ws.NewServer(hub)
	go hub.Run(ctx)
	logger.Info("WebSocket hub ready")

	// -------------------------------------------------------
	// 10b. Initialize anomaly alert consumer (Kafka -> WebSocket relay)
	// -------------------------------------------------------
	var anomalyConsumer *kafka.AnomalyConsumer
	if kafkaBrokers != "" {
		logger.Info("initializing anomaly alert consumer")
		anomalyConsumer = kafka.NewAnomalyConsumer(kafkaBrokers, func(alert kafka.AnomalyAlert) {
			hub.NotifyAnomalyAlert(ws.AnomalyAlertEvent{
				ID:           alert.ID,
				InstrumentID: alert.InstrumentID,
				VenueID:      alert.VenueID,
				AnomalyScore: alert.AnomalyScore,
				Severity:     alert.Severity,
				Features:     alert.Features,
				Description:  alert.Description,
				Timestamp:    alert.Timestamp,
				Acknowledged: alert.Acknowledged,
			})
		})
		anomalyConsumer.Start(ctx)
		defer anomalyConsumer.Stop()
		logger.Info("anomaly alert consumer started")
	}

	// -------------------------------------------------------
	// 11a. Initialize smart order router
	// -------------------------------------------------------
	logger.Info("initializing smart order router")
	bestPrice := router.NewBestPriceStrategy()
	venuePref := router.NewVenuePreferenceStrategy(bestPrice)

	smartRouter := router.New()
	smartRouter.Register(bestPrice) // first registered becomes default
	smartRouter.Register(venuePref)

	if mlScorerURL != "" {
		scorer := router.NewMLScorer(mlScorerURL + "/score")
		mlStrategy := router.NewMLStrategy(scorer, bestPrice)
		smartRouter.Register(mlStrategy)
		logger.Info("ML scoring strategy registered", slog.String("ml_scorer_url", mlScorerURL))
	}

	pipelineOpts = append(pipelineOpts, pipeline.WithRouter(smartRouter))
	logger.Info("smart order router initialized",
		slog.Any("strategies", smartRouter.Strategies()))

	// -------------------------------------------------------
	// 11b. Initialize internal crossing engine
	// -------------------------------------------------------
	logger.Info("initializing internal crossing engine")
	crossingEngine := crossing.NewCrossingEngine()
	pipelineOpts = append(pipelineOpts, pipeline.WithCrossingEngine(crossingEngine))
	logger.Info("internal crossing engine initialized")

	// -------------------------------------------------------
	// 11c. Initialize pipeline
	// -------------------------------------------------------
	logger.Info("initializing order processing pipeline")
	p := pipeline.NewPipeline(pgStore, venues, hub, riskClient, pipelineOpts...)

	// -------------------------------------------------------
	// 12. Start pipeline goroutines
	// -------------------------------------------------------
	logger.Info("starting pipeline goroutines")
	p.Start(ctx)

	// -------------------------------------------------------
	// 13. Initialize REST router and mount WS upgrade endpoints
	// -------------------------------------------------------
	logger.Info("initializing REST router")

	var routerOpts []rest.RouterOption

	if credMgr != nil {
		venueHandler := rest.NewVenueHandler(credMgr, logger)
		credHandler := rest.NewCredentialHandler(credMgr, logger)
		routerOpts = append(routerOpts,
			rest.WithVenueHandler(venueHandler),
			rest.WithCredentialHandler(credHandler),
		)
	}

	restRouter := rest.NewRouter(p, pgStore, routerOpts...)

	// Mount WebSocket upgrade endpoints on the same mux.
	mux := http.NewServeMux()
	mux.Handle("/", restRouter)
	mux.HandleFunc("/ws/orders", wsSrv.HandleOrders)
	mux.HandleFunc("/ws/positions", wsSrv.HandlePositions)
	mux.HandleFunc("/ws/venues", wsSrv.HandleVenues)
	mux.HandleFunc("/ws/anomalies", wsSrv.HandleAnomalies)

	// -------------------------------------------------------
	// 14. Start HTTP server
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
	// 15. Log "Gateway ready"
	// -------------------------------------------------------
	venueIDs := make([]string, 0, len(venues))
	for _, v := range venues {
		venueIDs = append(venueIDs, v.VenueID())
	}
	logger.Info("Gateway ready",
		slog.String("port", port),
		slog.Any("venues", venueIDs),
		slog.Bool("kafka_enabled", kafkaProducer != nil),
		slog.Bool("risk_engine_configured", riskEngineAddr != ""),
		slog.Bool("redis_connected", redisClient != nil),
		slog.Bool("credential_mgr_enabled", credMgr != nil),
		slog.Bool("anomaly_consumer_enabled", anomalyConsumer != nil),
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
			Venues: []string{"simulated", "alpaca"},
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
			Venues: []string{"simulated", "alpaca"},
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
			Venues: []string{"simulated", "alpaca"},
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
			Venues:          []string{"simulated", "binance_testnet"},
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
			Venues:          []string{"simulated", "binance_testnet"},
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
			Venues:          []string{"simulated", "binance_testnet"},
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
