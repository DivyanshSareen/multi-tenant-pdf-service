// main is the entrypoint for the multi-tenant PDF summary service.
// It loads config, connects to all backends, wires dependencies, and starts the HTTP server
// with graceful shutdown on SIGINT/SIGTERM.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/divyansh/multi-tenant-pdf-service/internal/api"
	"github.com/divyansh/multi-tenant-pdf-service/internal/config"
	"github.com/divyansh/multi-tenant-pdf-service/internal/database"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/ai"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/pdf"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/storage"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/tenant"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)

	cfg := config.Load()
	log.WithField("port", cfg.Server.Port).Info("starting multi-tenant pdf service")

	// --- Connect to PostgreSQL ---
	pgClient, err := database.NewPostgresClient(cfg.Postgres, log)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to postgres")
	}
	defer pgClient.Close()

	// --- Connect to MongoDB ---
	mongoClient, err := database.NewMongoClient(cfg.Mongo.URI, log)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to mongodb")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.WithError(err).Error("disconnecting from mongodb")
		}
	}()

	// --- Connect to MinIO ---
	storageClient, err := storage.NewStorageClient(cfg.MinIO, log)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to minio")
	}

	// --- Build services ---
	tenantMgr := tenant.NewManager(pgClient, mongoClient, storageClient, log)
	extractor := pdf.NewExtractor(log)
	summarizer := ai.NewSummarizer(cfg.LLM, log)

	// --- Build HTTP server ---
	srv := api.NewServer(pgClient, mongoClient, storageClient, tenantMgr, extractor, summarizer, log)
	router := srv.SetupRouter(cfg.Auth.APIKey)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second, // longer for LLM calls
		IdleTimeout:  30 * time.Second,
	}

	// --- Start server in background ---
	go func() {
		log.WithField("addr", httpServer.Addr).Info("http server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server error")
		}
	}()

	// --- Graceful shutdown on SIGINT / SIGTERM ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Error("server forced to shut down")
	}

	log.Info("server exited cleanly")
}
