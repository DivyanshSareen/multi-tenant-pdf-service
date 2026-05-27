package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listTenantsHandler returns all active tenants from the PostgreSQL registry.
func (s *Server) listTenantsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	tenants, err := s.pg.ListTenants(ctx)
	if err != nil {
		s.log.WithError(err).Error("listing tenants")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tenants"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenants": tenants,
		"total":   len(tenants),
	})
}

// getTenantHandler returns a single tenant by name.
func (s *Server) getTenantHandler(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	tenant, err := s.pg.GetTenant(ctx, name)
	if err != nil {
		s.log.WithError(err).WithField("tenant", name).Error("getting tenant")
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	c.JSON(http.StatusOK, tenant)
}

// getDocumentsHandler returns all documents stored in a tenant's MongoDB database.
func (s *Server) getDocumentsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	tenant, err := s.pg.GetTenant(ctx, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tenant not found"})
		return
	}

	docs, err := s.mongo.GetDocuments(ctx, tenant.MongoDBName)
	if err != nil {
		s.log.WithError(err).WithField("tenant", name).Error("getting documents")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve documents"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tenant":    name,
		"documents": docs,
		"total":     len(docs),
	})
}

// deleteTenantHandler destroys all tenant resources and soft-deletes the registry record.
func (s *Server) deleteTenantHandler(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if err := s.tenantMgr.DestroyTenant(ctx, name); err != nil {
		s.log.WithError(err).WithField("tenant", name).Error("destroying tenant")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to destroy tenant: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "tenant " + name + " destroyed successfully",
	})
}
