package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config contient toute la configuration de l'application
type Config struct {
	Server   ServerConfig
	Storage  StorageConfig
	WebUI    WebUIConfig
	AI       AIConfig
	Auth     AuthConfig
	Features FeaturesConfig
	Render   RenderConfig
	Verbose  bool
}

// RenderConfig regroupe les paramètres du moteur de rendu universel (fX v2)
type RenderConfig struct {
	Backend           string // "auto", "rust", "cli"
	Workers           int    // 0 = runtime.NumCPU()
	DaemonSocket      string // Chemin du socket Unix typst-renderd (ex: /var/run/fx-typst.sock)
	DaemonURL         string // URL TCP du daemon typst-renderd (ex: http://127.0.0.1:9090)
	TmpDir            string // Dossier temporaire explicite (défaut: /dev/shm/fx-render ou ./tmp/fx-render)
	FontPaths         string // Chemins de polices supplémentaires (TYPST_FONT_PATHS)
	IgnoreSystemFonts bool   // Ignorer les polices système (TYPST_IGNORE_SYSTEM_FONTS)
	MaxBatchSize      int    // Nombre maximal de documents par requête batch (défaut: 100)
}

// FeaturesConfig regroupe les feature flags de l'application
type FeaturesConfig struct {
	// AllowCustomTemplates active le mode "template Typst custom" (découplé de Factur-X):
	// endpoints /custom/* et page d'édition dédiée côté UI.
	AllowCustomTemplates bool
	// StudioEnabled active l'éditeur visuel Studio v2 et l'API v2 universelle.
	StudioEnabled bool
}

// WebUIConfig configuration de l'interface Angular servie par le backend
type WebUIConfig struct {
	Enabled bool
	Path    string
}

// ServerConfig configuration du serveur HTTP
type ServerConfig struct {
	Port            string
	Host            string
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
}

// StorageConfig configuration du stockage
type StorageConfig struct {
	Type      string // "local", "s3", "gcs", "azure"
	LocalPath string

	// S3-compatible (AWS S3, GCS, MinIO, DigitalOcean Spaces, etc.)
	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3UsePathStyle   bool // Pour MinIO et certains providers
	S3ForcePathStyle bool

	// GCS specific (peut aussi utiliser S3 compatible)
	GCSProjectID   string
	GCSCredentials string // Path to JSON credentials

	// Azure Blob Storage
	AzureAccountName string
	AzureAccountKey  string
	AzureContainer   string
}

// AuthConfig configuration de l'authentification
type AuthConfig struct {
	Enabled   bool
	APIKey    string
	Password  string
	JWTSecret string
}

// AIConfig configuration pour la génération de templates par l'IA
type AIConfig struct {
	Provider string // "openai", "ollama", "gemini"
	APIKey   string
	BaseURL  string // Pour Ollama ou API custom
	Model    string
}

// Load charge la configuration depuis les variables d'environnement
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("PORT", "8080"),
			Host:            getEnv("HOST", "0.0.0.0"),
			ReadTimeout:     getEnvAsInt("READ_TIMEOUT", 30),
			WriteTimeout:    getEnvAsInt("WRITE_TIMEOUT", 30),
			ShutdownTimeout: getEnvAsInt("SHUTDOWN_TIMEOUT", 5),
		},
		Storage: StorageConfig{
			Type:             getEnv("STORAGE_TYPE", "local"),
			LocalPath:        getEnv("STORAGE_LOCAL_PATH", "./storage"),
			S3Endpoint:       getEnv("S3_ENDPOINT", ""),
			S3Region:         getEnv("S3_REGION", "us-east-1"),
			S3Bucket:         getEnv("S3_BUCKET", ""),
			S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
			S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
			S3UsePathStyle:   getEnvAsBool("S3_USE_PATH_STYLE", false),
			S3ForcePathStyle: getEnvAsBool("S3_FORCE_PATH_STYLE", false),
			GCSProjectID:     getEnv("GCS_PROJECT_ID", ""),
			GCSCredentials:   getEnv("GCS_CREDENTIALS_PATH", ""),
			AzureAccountName: getEnv("AZURE_ACCOUNT_NAME", ""),
			AzureAccountKey:  getEnv("AZURE_ACCOUNT_KEY", ""),
			AzureContainer:   getEnv("AZURE_CONTAINER", ""),
		},
		Auth: AuthConfig{
			Enabled:   getEnvAsBool("AUTH_ENABLED", false),
			APIKey:    getEnv("AUTH_API_KEY", ""),
			Password:  getEnv("AUTH_PASSWORD", ""),
			JWTSecret: getEnv("AUTH_JWT_SECRET", "super-secret-key-change-me-in-production"),
		},
		WebUI: WebUIConfig{
			Enabled: getEnvAsBool("WEB_UI_ENABLED", true), // Activé par défaut
			Path:    getEnv("WEB_UI_PATH", "./web/ng"),    // Chemin des statiques d'Angular
		},
		AI: AIConfig{
			Provider: getEnv("AI_PROVIDER", "ollama"),
			APIKey:   getEnv("AI_API_KEY", ""),
			BaseURL:  getEnv("AI_BASE_URL", "http://localhost:11434"),
			Model:    getEnv("AI_MODEL", "llama3"),
		},
		Features: FeaturesConfig{
			AllowCustomTemplates: getEnvAsBool("ALLOW_CUSTOM_TEMPLATES", false),
			StudioEnabled:        getEnvAsBool("STUDIO_ENABLED", true),
		},
		Render: RenderConfig{
			Backend:           getEnv("RENDER_BACKEND", "auto"),
			Workers:           getEnvAsInt("RENDER_WORKERS", 0),
			DaemonSocket:      getEnv("RENDER_DAEMON_SOCKET", ""),
			DaemonURL:         getEnv("RENDER_DAEMON_URL", ""),
			TmpDir:            getEnv("RENDER_TMP_DIR", ""),
			FontPaths:         getEnv("RENDER_FONT_PATHS", os.Getenv("TYPST_FONT_PATHS")),
			IgnoreSystemFonts: getEnvAsBool("TYPST_IGNORE_SYSTEM_FONTS", false),
			MaxBatchSize:      getEnvAsInt("RENDER_MAX_BATCH_SIZE", 100),
		},
		Verbose: getEnvAsBool("VERBOSE", false),
	}

	// Validation
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate vérifie la cohérence de la configuration
func (c *Config) Validate() error {
	// Vérifier la configuration storage selon le type
	switch c.Storage.Type {
	case "s3":
		if c.Storage.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET requis quand STORAGE_TYPE=s3")
		}
		if c.Storage.S3AccessKey == "" || c.Storage.S3SecretKey == "" {
			return fmt.Errorf("S3_ACCESS_KEY et S3_SECRET_KEY requis")
		}
	case "gcs":
		if c.Storage.S3Bucket == "" && c.Storage.GCSProjectID == "" {
			return fmt.Errorf("S3_BUCKET ou GCS_PROJECT_ID requis pour GCS")
		}
	case "azure":
		if c.Storage.AzureAccountName == "" || c.Storage.AzureAccountKey == "" {
			return fmt.Errorf("AZURE_ACCOUNT_NAME et AZURE_ACCOUNT_KEY requis")
		}
		if c.Storage.AzureContainer == "" {
			return fmt.Errorf("AZURE_CONTAINER requis")
		}
	case "local":
		// OK, pas de validation spécifique
	default:
		return fmt.Errorf("STORAGE_TYPE invalide: %s (valeurs: local, s3, gcs, azure)", c.Storage.Type)
	}

	// Vérifier la configuration du moteur de rendu
	switch c.Render.Backend {
	case "", "auto", "rust", "cli":
		// OK
	default:
		return fmt.Errorf("RENDER_BACKEND invalide: %s (valeurs: auto, rust, cli)", c.Render.Backend)
	}
	if c.Render.Workers < 0 {
		return fmt.Errorf("RENDER_WORKERS invalide: %d (doit être >= 0)", c.Render.Workers)
	}
	if c.Render.MaxBatchSize < 0 {
		return fmt.Errorf("RENDER_MAX_BATCH_SIZE invalide: %d (doit être >= 0)", c.Render.MaxBatchSize)
	}

	return nil
}

// Helpers pour lire les env vars
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
