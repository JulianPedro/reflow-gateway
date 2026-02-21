package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/reflow/gateway/internal/api"
	"github.com/reflow/gateway/internal/auth"
	"github.com/reflow/gateway/internal/config"
	"github.com/reflow/gateway/internal/database"
	"github.com/reflow/gateway/internal/docs"
	"github.com/reflow/gateway/internal/gateway"
	"github.com/reflow/gateway/internal/k8s"
	"github.com/reflow/gateway/internal/observability"
	"github.com/reflow/gateway/internal/stdio"
	"github.com/reflow/gateway/internal/telemetry"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	// Setup logging
	setupLogging(cfg.Logging)

	log.Info().Msg("Starting Reflow Gateway")

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize OpenTelemetry
	otelProvider, err := telemetry.Init(ctx, telemetry.Config{
		Enabled:     cfg.Telemetry.Enabled,
		Endpoint:    cfg.Telemetry.Endpoint,
		ServiceName: cfg.Telemetry.ServiceName,
		Insecure:    cfg.Telemetry.Insecure,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize telemetry")
	}
	if otelProvider != nil {
		log.Info().
			Str("endpoint", cfg.Telemetry.Endpoint).
			Str("service", cfg.Telemetry.ServiceName).
			Msg("OpenTelemetry enabled")
	}
	telemetry.InitMetrics()

	// Connect to database
	db, err := database.New(ctx, cfg.Database.GetDSN())
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(ctx); err != nil {
		log.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Create repository
	repo := database.NewRepository(db)

	// Create JWT manager
	jwtManager := auth.NewJWTManager(cfg.JWT.Secret)

	// Create token encryptor
	encryptor, err := auth.NewTokenEncryptor(cfg.Encryption.Key)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create token encryptor")
	}

	// Create auth middleware
	authMiddleware := auth.NewMiddleware(jwtManager, repo)

	// Create observability hub (real-time dashboard)
	obsHub := observability.NewHub()

	// Create session manager
	sessionManager := gateway.NewSessionManager(repo, cfg.Session.Timeout, cfg.Session.CleanupInterval)
	defer sessionManager.Stop()

	// Create authorizer
	authorizer := gateway.NewAuthorizer(repo)

	// Create STDIO manager
	stdioManager := stdio.NewManager(repo, stdio.ManagerConfig{
		IdleTTL:      cfg.Stdio.IdleTTL,
		MaxLifetime:  cfg.Stdio.MaxLifetime,
		GCInterval:   cfg.Stdio.GCInterval,
		MaxProcesses: cfg.Stdio.MaxProcesses,
	})
	defer stdioManager.Shutdown()

	// Create Kubernetes manager (optional)
	var k8sManager *k8s.Manager
	if cfg.Kubernetes.Enabled {
		var err error
		k8sManager, err = k8s.NewManager(k8s.ManagerConfig{
			Namespace:    cfg.Kubernetes.Namespace,
			Kubeconfig:   cfg.Kubernetes.Kubeconfig,
			IdleTTL:      cfg.Kubernetes.IdleTTL,
			MaxLifetime:  cfg.Kubernetes.MaxLifetime,
			GCInterval:   cfg.Kubernetes.GCInterval,
			MaxInstances: cfg.Kubernetes.MaxInstances,
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create Kubernetes manager")
		}
		defer k8sManager.Shutdown()
		log.Info().Str("namespace", cfg.Kubernetes.Namespace).Msg("Kubernetes transport enabled")
	}

	// Create proxy
	proxy := gateway.NewProxy(repo, encryptor, authorizer, stdioManager, k8sManager, obsHub)

	// Create MCP gateway handler
	mcpHandler := gateway.NewHandler(sessionManager, proxy, repo, obsHub)

	// Create router
	r := chi.NewRouter()

	// Middleware (common to all routes)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// OTel HTTP middleware (wraps all routes with tracing)
	if cfg.Telemetry.Enabled {
		r.Use(func(next http.Handler) http.Handler {
			return otelhttp.NewHandler(next, "reflow-gateway")
		})
	}

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposeHeaders,
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// API documentation (Scalar UI + OpenAPI spec)
	r.Mount("/", docs.Handler())

	// MCP Streamable HTTP endpoint (protected)
	// Supports POST (JSON-RPC requests), GET (SSE notification stream), DELETE (session termination)
	r.Route("/mcp", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.HandleFunc("/*", mcpHandler.HandleMCP)
		r.HandleFunc("/", mcpHandler.HandleMCP)
	})

	// Legacy SSE endpoint alias - redirects to /mcp for clients that look for /sse
	r.Route("/sse", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.HandleFunc("/*", mcpHandler.HandleMCP)
		r.HandleFunc("/", mcpHandler.HandleMCP)
	})

	// REST API (with timeout)
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.Timeout(60 * time.Second))
		var instanceRestarter api.InstanceRestarter
		if k8sManager != nil {
			instanceRestarter = k8sManager
		}
		r.Mount("/", api.Router(repo, jwtManager, encryptor, authMiddleware, sessionManager, instanceRestarter))

		// Observability WebSocket (auth required)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Get("/observability/ws", obsHub.HandleWebSocket)
			r.Get("/observability/snapshot", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				snap := obsHub.GetAggregator().Snapshot()
				data, _ := json.Marshal(snap)
				w.Write(data)
			})
		})
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:        addr,
		Handler:     r,
		ReadTimeout: 30 * time.Second,
		// WriteTimeout must be 0 to support SSE (long-lived GET connections).
		// Per-route timeouts are enforced via middleware on /api routes.
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Info().Msg("Shutting down server...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if otelProvider != nil {
			otelProvider.Shutdown(shutdownCtx)
			log.Info().Msg("Telemetry shut down")
		}

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Server shutdown error")
		}

		cancel()
	}()

	log.Info().Str("addr", addr).Msg("Server listening")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Server error")
	}

	log.Info().Msg("Server stopped")
}

func setupLogging(cfg config.LoggingConfig) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set output format
	if cfg.Format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	// Add timestamp
	zerolog.TimeFieldFormat = time.RFC3339
}
