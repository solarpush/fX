package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage implémentation S3-compatible
// Fonctionne avec: AWS S3, GCS (S3 mode), MinIO, DigitalOcean Spaces, Scaleway, etc.
type S3Storage struct {
	client       *s3.Client
	bucket       string
	region       string
	endpoint     string
	usePathStyle bool
}

// S3Config configuration S3
type S3Config struct {
	Endpoint       string
	Region         string
	Bucket         string
	AccessKey      string
	SecretKey      string
	UsePathStyle   bool
	ForcePathStyle bool
}

// NewS3Storage crée un nouveau storage S3-compatible
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	var awsCfg aws.Config
	var err error

	optFns := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		))
	}

	awsCfg, err = config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Options := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = cfg.UsePathStyle || cfg.ForcePathStyle
		},
	}

	if cfg.Endpoint != "" {
		s3Options = append(s3Options, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Options...)

	return &S3Storage{
		client:       client,
		bucket:       cfg.Bucket,
		region:       cfg.Region,
		endpoint:     cfg.Endpoint,
		usePathStyle: cfg.UsePathStyle || cfg.ForcePathStyle,
	}, nil
}

// Put stocke un fichier sur S3
func (s *S3Storage) Put(ctx context.Context, path string, data io.Reader, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        data,
		ContentType: aws.String(contentType),
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

// Get récupère un fichier depuis S3
func (s *S3Storage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	result, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return result.Body, nil
}

// Delete supprime un fichier de S3
func (s *S3Storage) Delete(ctx context.Context, path string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists vérifie si un fichier existe
func (s *S3Storage) Exists(ctx context.Context, path string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	_, err := s.client.HeadObject(ctx, input)
	if err != nil {
		return false, nil
	}

	return true, nil
}

// List liste les fichiers dans un préfixe
func (s *S3Storage) List(ctx context.Context, prefix string) ([]FileInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	var files []FileInfo

	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			files = append(files, FileInfo{
				Path:         *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				ETag:         *obj.ETag,
			})
		}
	}

	return files, nil
}

// GetSignedURL génère une URL signée temporaire
func (s *S3Storage) GetSignedURL(ctx context.Context, path string, expiry time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s.client)

	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}

	result, err := presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	return result.URL, nil
}

// GetPublicURL retourne l'URL publique
func (s *S3Storage) GetPublicURL(path string) string {
	if s.endpoint != "" {
		if s.usePathStyle {
			return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, path)
		}
		return fmt.Sprintf("%s/%s", s.endpoint, path)
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, path)
}

// PutWithACL stocke un fichier avec un ACL spécifique
func (s *S3Storage) PutWithACL(ctx context.Context, path string, data io.Reader, contentType string, acl types.ObjectCannedACL) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        data,
		ContentType: aws.String(contentType),
		ACL:         acl,
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}
