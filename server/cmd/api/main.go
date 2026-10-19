package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ruhuang/ink/server/internal/ai"
	"github.com/ruhuang/ink/server/internal/auth"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/platform/clock"
	"github.com/ruhuang/ink/server/internal/platform/config"
	"github.com/ruhuang/ink/server/internal/platform/httpapi"
	"github.com/ruhuang/ink/server/internal/platform/idgen"
	"github.com/ruhuang/ink/server/internal/platform/observability"
	"github.com/ruhuang/ink/server/internal/platform/password"
	"github.com/ruhuang/ink/server/internal/platform/secret"
	"github.com/ruhuang/ink/server/internal/platform/store/postgres"
	"github.com/ruhuang/ink/server/internal/platform/token"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/scheduler"
	"github.com/ruhuang/ink/server/internal/workspace"
)

const (
	httpReadHeaderTimeout = 5 * time.Second
	httpReadTimeout       = 15 * time.Second
	httpWriteTimeout      = 2 * time.Minute
	httpIdleTimeout       = 60 * time.Second
	httpMaxHeaderBytes    = 1 << 20
	shutdownTimeout       = 10 * time.Second
)

func main() {
	if err := config.LoadDotEnv(); err != nil {
		panic(err)
	}

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, cancelRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelRoot()
	shutdownTracing, err := observability.ConfigureTracing(ctx, cfg.AppName, logger)
	if err != nil {
		panic(err)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flushCtx); err != nil {
			logger.Error("trace shutdown failed")
		}
	}()

	db, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		panic(err)
	}

	store := postgres.New(db)
	tokenManager, err := token.NewJWTAccessManager(cfg.JWTSecret, cfg.AppName, cfg.AccessTokenTTL)
	if err != nil {
		panic(err)
	}

	service := auth.NewService(
		store,
		store,
		store,
		password.BcryptHasher{},
		tokenManager,
		clock.SystemClock{},
		idgen.Generator{},
		cfg.RefreshTokenTTL,
	)
	workspaceService := workspace.NewService(store, service, clock.SystemClock{})
	var encryptor *secret.Box
	if cfg.AIConfigEncryptionKey != "" {
		encryptor, err = secret.NewBox(cfg.AIConfigEncryptionKey)
		if err != nil {
			panic(err)
		}
	}
	aiService := ai.NewService(
		store,
		service,
		ai.NewOpenAIClient(cfg.AIProviderTimeout, cfg.AIAllowInsecurePrivateURL),
		encryptor,
		clock.SystemClock{},
		cfg.AIAllowInsecurePrivateURL,
	)
	printerService := printer.NewService(
		store,
		service,
		idgen.Generator{},
		clock.SystemClock{},
		cfg.MemobirdAccessKey,
		cfg.MemobirdBaseURL,
		cfg.MemobirdTimeout,
	)
	feedbackService := feedback.NewService(
		service,
		store,
		store,
		store,
		printerService,
		clock.SystemClock{},
	)
	var metrics *observability.Metrics
	var backlogDone <-chan struct{}
	if cfg.MetricsAddr != "" {
		metrics = observability.NewMetrics()
		metrics.RegisterPool(db)
		backlogDone = metrics.StartBacklog(ctx, store)
	}
	if metrics != nil || observability.TracingEnabled() {
		printerService.SubmissionObserver = metrics.Operation("print_submit")
		printerService.StatusObserver = metrics.Operation("print_status")
		printerService.RenderObserver = metrics.Operation("png_render")
	}
	workerDone := make(map[string]<-chan struct{})
	startWorker(cfg.WorkersEnabled && cfg.PrintStatusSyncEnabled, "print_status_sync", workerDone, func() <-chan struct{} {
		statusSync := printer.NewStatusSynchronizerWithPollingAndAttempts(printerService, cfg.PrintStatusTimeout, cfg.PrintStatusRecheckInterval, cfg.PrintStatusMaxRecheckInterval, cfg.PrintStatusMaxAttempts)
		statusRunner := scheduler.NewPrintStatusRunner(statusSync, logger, cfg.PrintStatusPollInterval, cfg.PrintStatusBatchSize)
		if metrics != nil {
			statusRunner.Observer = metrics.Worker("print_status_sync", cfg.PrintStatusPollInterval)
		}
		return statusRunner.Start(ctx)
	})

	readiness := observability.NewReadiness(ctx, db.Ping, workerDone)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: httpapi.NewServer(
			service,
			workspaceService,
			aiService,
			printerService,
			feedbackService,
			logger,
			cfg.RateLimitWindow,
			cfg.RateLimitMax,
			cfg.RateLimitMaxEntries,
			cfg.TrustedProxyCIDRs,
			cfg.TrustedProxyHeader,
			httpapi.WithObservability(metrics, readiness.Check),
		).Handler(),
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    httpMaxHeaderBytes,
	}
	var metricsServer *http.Server
	if cfg.MetricsAddr != "" {
		metricsServer = &http.Server{
			Addr: cfg.MetricsAddr, Handler: metrics.Handler(),
			ReadHeaderTimeout: httpReadHeaderTimeout, ReadTimeout: httpReadTimeout,
			WriteTimeout: 10 * time.Second, IdleTimeout: httpIdleTimeout,
			MaxHeaderBytes: httpMaxHeaderBytes,
		}
	}

	logger.Info("starting auth api", "port", cfg.Port)

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	metricsErrors := make(chan error, 1)
	if metricsServer != nil {
		go func() { metricsErrors <- metricsServer.ListenAndServe() }()
	}

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
			logger.Error("server stopped unexpectedly", "error", err)
		}
		cancelRoot()
	case err := <-metricsErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
			logger.Error("metrics server stopped unexpectedly", "error", err)
		}
		cancelRoot()
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		if serveErr == nil {
			serveErr = err
		}
	}
	if metricsServer != nil {
		if err := metricsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("metrics server shutdown failed", "error", err)
			if serveErr == nil {
				serveErr = err
			}
		}
	}

	workers := make([]<-chan struct{}, 0, len(workerDone)+1)
	for _, done := range workerDone {
		workers = append(workers, done)
	}
	if backlogDone != nil {
		workers = append(workers, backlogDone)
	}
	for _, worker := range workers {
		select {
		case <-worker:
		case <-shutdownCtx.Done():
			logger.Error("worker shutdown timed out", "error", shutdownCtx.Err())
			if serveErr == nil {
				serveErr = shutdownCtx.Err()
			}
		}
	}

	if serveErr != nil {
		panic(serveErr)
	}
}

// startWorker registers only enabled workers for readiness and shutdown checks.
func startWorker(enabled bool, name string, workers map[string]<-chan struct{}, start func() <-chan struct{}) {
	if enabled {
		workers[name] = start()
	}
}
