package storage

import (
	"context"
	"fmt"

	"github.com/solarpush/fx/internal/config"
)

// NewStorage crée le storage approprié selon la configuration
func NewStorage(ctx context.Context, cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "local":
		return NewLocalStorage(cfg.LocalPath)

	case "s3":
		return NewS3Storage(ctx, S3Config{
			Endpoint:       cfg.S3Endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			UsePathStyle:   cfg.S3UsePathStyle,
			ForcePathStyle: cfg.S3ForcePathStyle,
		})

	case "gcs":
		// GCS peut utiliser l'interface S3-compatible
		endpoint := cfg.S3Endpoint
		if endpoint == "" {
			endpoint = "https://storage.googleapis.com"
		}
		return NewS3Storage(ctx, S3Config{
			Endpoint:       endpoint,
			Region:         cfg.S3Region,
			Bucket:         cfg.S3Bucket,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			UsePathStyle:   true, // GCS utilise path-style
			ForcePathStyle: true,
		})

	case "azure":
		// TODO: Implémenter Azure Blob Storage
		return nil, fmt.Errorf("azure storage not yet implemented")

	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}
