// Package api wires together the Gin router, all handlers, and middleware.
package api

import (
	"github.com/divyansh/multi-tenant-pdf-service/internal/database"
	"github.com/divyansh/multi-tenant-pdf-service/internal/middleware"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/ai"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/pdf"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/storage"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/tenant"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Server holds all handler dependencies and exposes the HTTP router.
type Server struct {
	pg         *database.PostgresClient
	mongo      *database.MongoClient
	store      *storage.StorageClient
	tenantMgr  *tenant.Manager
	extractor  *pdf.Extractor
	summarizer ai.Summarizer
	log        *logrus.Logger
}

// NewServer wires all dependencies into the Server struct.
func NewServer(
	pg *database.PostgresClient,
	mongo *database.MongoClient,
	store *storage.StorageClient,
	tenantMgr *tenant.Manager,
	extractor *pdf.Extractor,
	summarizer ai.Summarizer,
	log *logrus.Logger,
) *Server {
	return &Server{
		pg:         pg,
		mongo:      mongo,
		store:      store,
		tenantMgr:  tenantMgr,
		extractor:  extractor,
		summarizer: summarizer,
		log:        log,
	}
}

// SetupRouter creates and returns the Gin engine with all routes registered.
func (s *Server) SetupRouter(apiKey string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.BearerAuth(apiKey))

	// Public probe endpoints — auth middleware skips these.
	r.GET("/health", s.healthHandler)
	r.GET("/ready", s.readyHandler)

	// Protected API routes.
	v1 := r.Group("/api/v1")
	{
		v1.POST("/upload", s.uploadHandler)

		v1.GET("/tenants", s.listTenantsHandler)
		v1.GET("/tenants/:name", s.getTenantHandler)
		v1.GET("/tenants/:name/documents", s.getDocumentsHandler)
		v1.DELETE("/tenants/:name", s.deleteTenantHandler)
	}

	return r
}
