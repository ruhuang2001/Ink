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
	"github.com/ruhuang/ink/server/internal/dispatch"
	"github.com/ruhuang/ink/server/internal/feedback"
	"github.com/ruhuang/ink/server/internal/inbox"
	"github.com/ruhuang/ink/server/internal/platform/clock"
	"github.com/ruhuang/ink/server/internal/platform/config"
	"github.com/ruhuang/ink/server/internal/platform/httpapi"
	"github.com/ruhuang/ink/server/internal/platform/idgen"
	"github.com/ruhuang/ink/server/internal/platform/password"
	"github.com/ruhuang/ink/server/internal/platform/secret"
	"github.com/ruhuang/ink/server/internal/platform/store/postgres"
	"github.com/ruhuang/ink/server/internal/platform/token"
	"github.com/ruhuang/ink/server/internal/pluginfetch"
	"github.com/ruhuang/ink/server/internal/plugins"
	"github.com/ruhuang/ink/server/internal/printer"
	"github.com/ruhuang/ink/server/internal/schedule"
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
	pluginService := plugins.NewService(
		store,
		service,
		encryptor,
		idgen.Generator{},
		clock.SystemClock{},
		nil,
		cfg.PluginRoot,
		cfg.PluginExecTimeout,
		cfg.PluginInstallTimeout,
		plugins.RuntimeLimits{
			OutputMaxBytes:        cfg.PluginOutputMaxBytes,
			FetchMaxItems:         cfg.PluginFetchMaxItems,
			FetchMaxBlocksPerItem: cfg.PluginFetchMaxBlocksPerItem,
			FetchMaxTextBytes:     cfg.PluginFetchMaxTextBytes,
			FetchMaxURLBytes:      cfg.PluginFetchMaxURLBytes,
			EnvAllowlist:          cfg.PluginEnvAllowlist,
		},
		plugins.GoGitCloner{},
		cfg.PluginGitAllowedHosts,
	)
	inboxService := inbox.NewService(store, idgen.Generator{}, clock.SystemClock{})
	dispatchService := dispatch.NewService(
		store,
		printerService,
		store,
		idgen.Generator{},
		clock.SystemClock{},
	)
	pluginFetchService := pluginfetch.NewService(service, pluginService, inboxService, clock.SystemClock{})
	scheduleService := schedule.NewService(
		store,
		service,
		pluginService,
		store,
		dispatchService,
		idgen.Generator{},
		clock.SystemClock{},
	)
	fetchRunner := scheduler.NewFetchRunner(pluginFetchService, logger, cfg.SchedulerPollInterval, 10)
	fetchDone := fetchRunner.Start(ctx)

	schedulerRunner := scheduler.NewRunner(scheduleService, logger, cfg.SchedulerPollInterval, 10)
	schedulerDone := schedulerRunner.Start(ctx)

	inboxJanitor := scheduler.NewInboxJanitor(inboxService, clock.SystemClock{}, logger, cfg.InboxJanitorInterval, cfg.InboxRetention)
	janitorDone := inboxJanitor.Start(ctx)

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: httpapi.NewServer(
			service,
			workspaceService,
			aiService,
			printerService,
			feedbackService,
			pluginService,
			pluginFetchService,
			scheduleService,
			logger,
			cfg.RateLimitWindow,
			cfg.RateLimitMax,
			cfg.RateLimitMaxEntries,
			cfg.TrustedProxyCIDRs,
			cfg.TrustedProxyHeader,
			cfg.PluginUploadMaxBytes,
		).Handler(),
		ReadHeaderTimeout: httpReadHeaderTimeout,
		ReadTimeout:       httpReadTimeout,
		WriteTimeout:      httpWriteTimeout,
		IdleTimeout:       httpIdleTimeout,
		MaxHeaderBytes:    httpMaxHeaderBytes,
	}

	logger.Info("starting auth api", "port", cfg.Port)

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()

	var serveErr error
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr = err
			logger.Error("server stopped unexpectedly", "error", err)
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

	for _, worker := range []<-chan struct{}{fetchDone, schedulerDone, janitorDone} {
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
