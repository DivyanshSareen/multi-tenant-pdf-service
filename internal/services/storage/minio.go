// Package storage provides a MinIO client for tenant-scoped object storage.
package storage

import (
	"context"
	"fmt"
	"os"

	"github.com/divyansh/multi-tenant-pdf-service/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/sirupsen/logrus"
)

// StorageClient wraps the MinIO client and exposes bucket/object operations.
type StorageClient struct {
	client *minio.Client
	log    *logrus.Logger
}

// NewStorageClient initialises a MinIO client and verifies connectivity.
func NewStorageClient(cfg config.MinIOConfig, log *logrus.Logger) (*StorageClient, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}

	log.WithField("endpoint", cfg.Endpoint).Info("minio client initialised")
	return &StorageClient{client: client, log: log}, nil
}

// Ping verifies the MinIO connection by checking for a sentinel bucket name.
// BucketExists is a lightweight HEAD request that doesn't require the bucket to exist.
func (s *StorageClient) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, "ping-check")
	return err
}

// CreateBucket creates a bucket for the tenant if it does not already exist.
func (s *StorageClient) CreateBucket(ctx context.Context, bucketName string) error {
	exists, err := s.client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("checking bucket %q existence: %w", bucketName, err)
	}
	if exists {
		s.log.WithField("bucket", bucketName).Debug("bucket already exists, skipping creation")
		return nil
	}

	if err := s.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("creating bucket %q: %w", bucketName, err)
	}

	s.log.WithField("bucket", bucketName).Info("created minio bucket")
	return nil
}

// UploadFile uploads a local file to the given bucket and returns the object reference path.
// The reference format is "bucketName/objectName".
func (s *StorageClient) UploadFile(ctx context.Context, bucketName, objectName, filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("opening file %q: %w", filePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file %q: %w", filePath, err)
	}

	_, err = s.client.PutObject(ctx, bucketName, objectName, file, stat.Size(),
		minio.PutObjectOptions{ContentType: "application/pdf"})
	if err != nil {
		return "", fmt.Errorf("uploading %q to bucket %q: %w", objectName, bucketName, err)
	}

	ref := bucketName + "/" + objectName
	s.log.WithFields(logrus.Fields{"bucket": bucketName, "object": objectName}).Info("uploaded file to minio")
	return ref, nil
}

// DeleteBucket empties a bucket by removing all objects, then deletes the bucket itself.
func (s *StorageClient) DeleteBucket(ctx context.Context, bucketName string) error {
	objectsCh := s.client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	for obj := range objectsCh {
		if obj.Err != nil {
			return fmt.Errorf("listing objects in bucket %q: %w", bucketName, obj.Err)
		}
		if err := s.client.RemoveObject(ctx, bucketName, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("removing object %q from bucket %q: %w", obj.Key, bucketName, err)
		}
		s.log.WithFields(logrus.Fields{"bucket": bucketName, "object": obj.Key}).Debug("deleted object")
	}

	if err := s.client.RemoveBucket(ctx, bucketName); err != nil {
		return fmt.Errorf("removing bucket %q: %w", bucketName, err)
	}

	s.log.WithField("bucket", bucketName).Info("deleted minio bucket and all its objects")
	return nil
}
