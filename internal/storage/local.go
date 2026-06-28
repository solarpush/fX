package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage implémentation locale pour le développement
type LocalStorage struct {
	basePath string
}

// NewLocalStorage crée un nouveau storage local
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &LocalStorage{
		basePath: basePath,
	}, nil
}

// Put stocke un fichier localement
func (s *LocalStorage) Put(ctx context.Context, path string, data io.Reader, contentType string) error {
	fullPath := filepath.Join(s.basePath, path)

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get récupère un fichier
func (s *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete supprime un fichier
func (s *LocalStorage) Delete(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.basePath, path)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists vérifie si un fichier existe
func (s *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	fullPath := filepath.Join(s.basePath, path)

	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// List liste les fichiers dans un préfixe
func (s *LocalStorage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	fullPath := filepath.Join(s.basePath, prefix)

	var files []FileInfo

	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			relPath, _ := filepath.Rel(s.basePath, path)
			files = append(files, FileInfo{
				Path:         relPath,
				Size:         info.Size(),
				LastModified: info.ModTime(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return files, nil
}

// GetSignedURL retourne le chemin local (pas de signature)
func (s *LocalStorage) GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	return s.GetPublicURL(path), nil
}

// GetPublicURL retourne le chemin local
func (s *LocalStorage) GetPublicURL(path string) string {
	return filepath.Join(s.basePath, path)
}
