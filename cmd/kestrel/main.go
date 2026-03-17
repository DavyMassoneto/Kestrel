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

	"github.com/go-chi/chi/v5"

	"github.com/DavyMassoneto/Kestrel/internal/adapter/claude"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/crypto"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/handler"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/middleware"
	"github.com/DavyMassoneto/Kestrel/internal/adapter/sqlite"
	"github.com/DavyMassoneto/Kestrel/internal/domain/entity"
	"github.com/DavyMassoneto/Kestrel/internal/domain/vo"
	"github.com/DavyMassoneto/Kestrel/internal/infra/cfg"
	"github.com/DavyMassoneto/Kestrel/internal/infra/logger"
	"github.com/DavyMassoneto/Kestrel/internal/usecase"
)

func main() {
	// --- Config ---
	appCfg, err := cfg.Load()
	if err != nil {
		slog.Error("failed to load config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Logger ---
	log := logger.New(appCfg.LogLevel, appCfg.LogFormat)

	// --- Crypto ---
	encryptor, err := crypto.NewAESEncryptor(appCfg.EncryptionKey)
	if err != nil {
		log.Error("failed to create encryptor", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// --- Database ---
	db, err := sqlite.NewDB(appCfg.DBPath)
	if err != nil {
		log.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// --- Migrations ---
	if err := sqlite.RunMigrations(db.Writer()); err != nil {
		log.Error("failed to run migrations", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("migrations applied")

	// --- Repositories ---
	accountRepo := sqlite.NewAccountRepo(db, encryptor)
	apiKeyRepo := sqlite.NewAPIKeyRepo(db)

	// --- Use Cases (Admin) ---
	adminAccountUC := usecase.NewAdminAccountUseCase(accountRepo)
	adminAPIKeyUC := usecase.NewAdminAPIKeyUseCase(apiKeyRepo)

	// --- Use Cases (Auth) ---
	authenticateUC := usecase.NewAuthenticateUseCase(apiKeyRepo)

	// --- Use Cases (Proxy — Phase 2 single-account) ---
	defaultAccount, err := entity.NewAccount(
		vo.NewAccountID(),
		"default",
		vo.NewSensitiveString(appCfg.ClaudeAPIKey),
		appCfg.ClaudeBaseURL,
		0,
	)
	if err != nil {
		log.Error("failed to create default account", slog.String("error", err.Error()))
		os.Exit(1)
	}

	claudeClient := claude.NewClient(http.DefaultClient)
	proxyChatUC := usecase.NewProxyChatUseCase(claudeClient, defaultAccount)
	proxyStreamUC := usecase.NewProxyStreamUseCase(claudeClient, defaultAccount)

	// --- Handlers ---
	startTime := time.Now()
	healthHandler := handler.NewHealth(startTime)
	adminHandler := handler.NewAdminHandler(adminAccountUC, adminAPIKeyUC, appCfg.AdminKey)
	chatHandler := handler.NewChatHandler(proxyChatUC, proxyStreamUC)
	modelsHandler := handler.ModelsHandler()

	// --- Router ---
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recovery)

	r.Get("/health", healthHandler.ServeHTTP)

	adminHandler.RegisterRoutes(r)

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.NewLogging(nil))
		r.Use(middleware.Auth(authenticateUC))
		r.Post("/chat/completions", chatHandler.ServeHTTP)
		r.Get("/models", modelsHandler)
	})

	// --- Server ---
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", appCfg.Port),
		Handler: r,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Info("server starting", slog.Int("port", appCfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sig := <-done
	log.Info("shutting down", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("server stopped")
}
