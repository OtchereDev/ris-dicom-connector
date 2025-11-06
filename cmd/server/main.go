package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/otcheredev/ris-dicom-connector/internal/adapters"
	"github.com/otcheredev/ris-dicom-connector/internal/cache"
	"github.com/otcheredev/ris-dicom-connector/internal/config"
	"github.com/otcheredev/ris-dicom-connector/internal/database"
	"github.com/otcheredev/ris-dicom-connector/internal/handlers"
	"github.com/otcheredev/ris-dicom-connector/internal/middleware"
	"github.com/otcheredev/ris-dicom-connector/internal/repository"
	"github.com/otcheredev/ris-dicom-connector/internal/services"
	"github.com/otcheredev/ris-dicom-connector/pkg/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("Invalid configuration")
	}

	// Initialize logger
	logger.Init(cfg.Log.Level, cfg.Log.Format)
	log.Info().Msg("Starting DICOM Connector")

	// Connect to database
	dbConfig := database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		DBName:   cfg.Database.DBName,
		SSLMode:  cfg.Database.SSLMode,
		LogLevel: cfg.Database.LogLevel,
	}

	if err := database.Connect(dbConfig); err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer database.Close()

	// Initialize cache
	var cacheImpl cache.Cache
	if cfg.Cache.Enabled {
		if cfg.Cache.Type == "redis" {
			addr := fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port)
			cacheImpl, err = cache.NewRedisCache(addr, cfg.Redis.Password, cfg.Redis.DB)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to connect to Redis")
			}
			log.Info().Msg("Redis cache initialized")
		} else {
			cacheImpl = cache.NewMemoryCache()
			log.Info().Msg("Memory cache initialized")
		}
	} else {
		cacheImpl = cache.NewMemoryCache() // Fallback
		log.Info().Msg("Cache disabled, using memory cache as fallback")
	}

	// Initialize repositories
	pacsRepo := repository.NewPACSRepository()
	auditRepo := repository.NewAuditRepository()

	// Initialize adapter factory
	adapterFactory := adapters.NewAdapterFactory()
	defer adapterFactory.CloseAll()

	// Initialize services
	pacsService := services.NewPACSService(pacsRepo, auditRepo, adapterFactory, cacheImpl)

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	dicomwebHandler := handlers.NewDICOMWebHandler(pacsService)
	managementHandler := handlers.NewManagementHandler(pacsService)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recovery)
	r.Use(middleware.Logging)
	r.Use(chimiddleware.Compress(5))

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   []string{"Content-Length", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// Health endpoints (no authentication required)
	r.Get("/health", healthHandler.Health)
	r.Get("/ready", healthHandler.Ready)

	// Metrics endpoint
	if cfg.Metrics.Enabled {
		r.Handle("/metrics", promhttp.Handler())
	}

	// DICOMweb endpoints (require tenant ID)
	r.Route("/dicom-web", func(r chi.Router) {
		r.Use(middleware.TenantID)

		// QIDO-RS (Query)
		r.Get("/studies", dicomwebHandler.SearchStudies)
		r.Get("/studies/{studyUID}/series", dicomwebHandler.SearchSeries)
		r.Get("/studies/{studyUID}/series/{seriesUID}/instances", dicomwebHandler.SearchInstances)

		// WADO-RS (Retrieve)
		r.Get("/studies/{studyUID}/metadata", dicomwebHandler.GetStudyMetadata)
		r.Get("/studies/{studyUID}/series/{seriesUID}/instances/{instanceUID}", dicomwebHandler.RetrieveInstance)
	})

	// Management API
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.TenantID)

		// PACS configuration
		r.Post("/pacs/config", managementHandler.CreatePACSConfig)
		r.Get("/pacs/config", managementHandler.GetPACSConfigs)
		r.Get("/pacs/config/{id}", managementHandler.GetPACSConfig)

		// Connection testing (no tenant ID required)
		r.With(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip tenant middleware for this route
				next.ServeHTTP(w, r)
			})
		}).Post("/pacs/test", managementHandler.TestConnection)
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// Start server in a goroutine
	go func() {
		log.Info().Str("addr", addr).Msg("Server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed to start")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}
