// Package tenant provides the TenantManager which orchestrates provisioning and
// destruction of all per-tenant resources: PostgreSQL registry, MongoDB database,
// and MinIO bucket.
package tenant

import (
	"context"
	"fmt"

	"github.com/divyansh/multi-tenant-pdf-service/internal/database"
	"github.com/divyansh/multi-tenant-pdf-service/internal/models"
	"github.com/divyansh/multi-tenant-pdf-service/internal/services/storage"
	"github.com/sirupsen/logrus"
)

// Manager orchestrates tenant lifecycle across all three storage backends.
type Manager struct {
	postgres *database.PostgresClient
	mongo    *database.MongoClient
	storage  *storage.StorageClient
	log      *logrus.Logger
}

// NewManager creates a TenantManager with the required dependencies.
func NewManager(
	pg *database.PostgresClient,
	mongo *database.MongoClient,
	store *storage.StorageClient,
	log *logrus.Logger,
) *Manager {
	return &Manager{
		postgres: pg,
		mongo:    mongo,
		storage:  store,
		log:      log,
	}
}

// GetOrCreateTenant returns an existing active tenant or provisions a new one.
// It is the primary entry point used by the upload handler.
func (m *Manager) GetOrCreateTenant(ctx context.Context, name string) (*models.Tenant, bool, error) {
	exists, err := m.postgres.TenantExists(ctx, name)
	if err != nil {
		return nil, false, fmt.Errorf("checking tenant existence: %w", err)
	}

	if exists {
		tenant, err := m.postgres.GetTenant(ctx, name)
		if err != nil {
			return nil, false, err
		}
		return tenant, false, nil
	}

	tenant, err := m.ProvisionTenant(ctx, name)
	if err != nil {
		return nil, false, err
	}
	return tenant, true, nil
}

// ProvisionTenant creates all resources for a new tenant in order:
//  1. PostgreSQL record (status: provisioning)
//  2. MongoDB database + documents collection
//  3. MinIO bucket
//  4. PostgreSQL status → active
//
// If any step fails after the PG record is created, the record is left in
// "provisioning" state so operators can identify and clean up partial deployments.
func (m *Manager) ProvisionTenant(ctx context.Context, name string) (*models.Tenant, error) {
	log := m.log.WithField("tenant", name)
	log.Info("provisioning tenant")

	// Step 1: create registry record
	tenant, err := m.postgres.CreateTenant(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("creating tenant record: %w", err)
	}

	// Step 2: initialise MongoDB database
	if err := m.mongo.CreateTenantDB(ctx, tenant.MongoDBName); err != nil {
		return nil, fmt.Errorf("creating mongodb database %q: %w", tenant.MongoDBName, err)
	}
	log.WithField("mongo_db", tenant.MongoDBName).Info("mongodb database ready")

	// Step 3: create MinIO bucket
	if err := m.storage.CreateBucket(ctx, tenant.BucketName); err != nil {
		return nil, fmt.Errorf("creating minio bucket %q: %w", tenant.BucketName, err)
	}
	log.WithField("bucket", tenant.BucketName).Info("minio bucket ready")

	// Step 4: mark active
	if err := m.postgres.ActivateTenant(ctx, name); err != nil {
		return nil, fmt.Errorf("activating tenant: %w", err)
	}
	tenant.Status = models.TenantStatusActive

	log.Info("tenant provisioned successfully")
	return tenant, nil
}

// DestroyTenant tears down all resources for a tenant in reverse order:
//  1. Drop MongoDB database
//  2. Empty and delete MinIO bucket
//  3. Soft-delete PostgreSQL record
func (m *Manager) DestroyTenant(ctx context.Context, name string) error {
	log := m.log.WithField("tenant", name)
	log.Info("destroying tenant")

	tenant, err := m.postgres.GetTenant(ctx, name)
	if err != nil {
		return fmt.Errorf("fetching tenant for destruction: %w", err)
	}

	// Step 1: drop MongoDB database
	if err := m.mongo.DropDatabase(ctx, tenant.MongoDBName); err != nil {
		return fmt.Errorf("dropping mongodb database: %w", err)
	}
	log.WithField("mongo_db", tenant.MongoDBName).Info("dropped mongodb database")

	// Step 2: delete MinIO bucket (empties it first)
	if err := m.storage.DeleteBucket(ctx, tenant.BucketName); err != nil {
		return fmt.Errorf("deleting minio bucket: %w", err)
	}
	log.WithField("bucket", tenant.BucketName).Info("deleted minio bucket")

	// Step 3: soft-delete in PostgreSQL
	if err := m.postgres.DeleteTenant(ctx, name); err != nil {
		return fmt.Errorf("soft-deleting tenant: %w", err)
	}

	log.Info("tenant destroyed successfully")
	return nil
}
