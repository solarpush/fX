package storage

import (
	"context"
	"io"
	"time"
)

// Storage interface abstraite pour tous les providers
type Storage interface {
	// Put stocke un fichier
	Put(ctx context.Context, path string, data io.Reader, contentType string) error

	// Get récupère un fichier
	Get(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete supprime un fichier
	Delete(ctx context.Context, path string) error

	// Exists vérifie si un fichier existe
	Exists(ctx context.Context, path string) (bool, error)

	// List liste les fichiers dans un préfixe
	List(ctx context.Context, prefix string) ([]FileInfo, error)

	// GetSignedURL génère une URL signée temporaire
	GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error)

	// GetPublicURL retourne l'URL publique (si applicable)
	GetPublicURL(path string) string
}

// FileInfo informations sur un fichier stocké
type FileInfo struct {
	Path         string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
}

// UploadOptions options pour l'upload
type UploadOptions struct {
	ContentType string
	Metadata    map[string]string
	ACL         string // "private", "public-read", etc.
}
