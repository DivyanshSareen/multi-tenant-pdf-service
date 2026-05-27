// Package models defines the core data structures shared across the service.
package models

import (
	"time"
)

// TenantStatus represents the lifecycle state of a tenant.
type TenantStatus string

const (
	TenantStatusProvisioning TenantStatus = "provisioning"
	TenantStatusActive       TenantStatus = "active"
	TenantStatusDeleted      TenantStatus = "deleted"
)

// Tenant represents a registered tenant in the master PostgreSQL registry.
// Each tenant maps to one MongoDB database and one MinIO bucket.
type Tenant struct {
	ID          string       `db:"id"           json:"id"`
	Name        string       `db:"name"         json:"name"`
	MongoDBName string       `db:"mongodb_name" json:"mongodb_name"`
	BucketName  string       `db:"bucket_name"  json:"bucket_name"`
	Status      TenantStatus `db:"status"       json:"status"`
	CreatedAt   time.Time    `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"   json:"updated_at"`
}

// Document represents a PDF document stored in a tenant's MongoDB database.
// FileReference is the MinIO object path (bucket/objectName).
type Document struct {
	ID            string    `bson:"_id,omitempty" json:"id"`
	FileName      string    `bson:"file_name"     json:"file_name"`
	FullText      string    `bson:"full_text"     json:"full_text,omitempty"`
	Summary       string    `bson:"summary"       json:"summary"`
	FileReference string    `bson:"file_reference" json:"file_reference"`
	FileSize      int64     `bson:"file_size"     json:"file_size"`
	PageCount     int       `bson:"page_count"    json:"page_count"`
	UploadedAt    time.Time `bson:"uploaded_at"   json:"uploaded_at"`
}

// UploadResponse is returned after a successful PDF upload and summarization.
type UploadResponse struct {
	DocumentID    string `json:"document_id"`
	TenantName    string `json:"tenant_name"`
	FileName      string `json:"file_name"`
	Summary       string `json:"summary"`
	FileReference string `json:"file_reference"`
	PageCount     int    `json:"page_count"`
	IsNewTenant   bool   `json:"is_new_tenant"`
}

// TenantListResponse wraps a list of tenants for the list endpoint.
type TenantListResponse struct {
	Tenants []Tenant `json:"tenants"`
	Total   int      `json:"total"`
}

// ErrorResponse is the standard error envelope returned on failures.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// HealthResponse is returned by GET /health.
type HealthResponse struct {
	Status string `json:"status"`
}

// ReadyResponse is returned by GET /ready — reports per-dependency status.
type ReadyResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components"`
}
