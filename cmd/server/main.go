package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/internal/api"
	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/internal/storage"
)

const version = "1.0.0"

func main() {
	log.Printf("🚀 Starting Factur-X Server v%s", version)

	// Charger la configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	log.Printf("📋 Configuration loaded:")
	log.Printf("  - Server: %s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("  - Storage: %s", cfg.Storage.Type)
	log.Printf("  - Web UI (Angular): %v", cfg.WebUI.Enabled)

	// Initialiser le storage
	ctx := context.Background()
	store, err := storage.NewStorage(ctx, cfg.Storage)
	if err != nil {
		log.Fatalf("❌ Failed to initialize storage: %v", err)
	}
	log.Printf("✅ Storage initialized: %s", cfg.Storage.Type)

	// Initialiser le client IA
	aiClient := ai.NewClient(cfg.AI)
	log.Printf("🤖 AI client initialized (Provider: %s)", cfg.AI.Provider)

	// Créer le handler API
	handler, err := api.NewHandler(store, aiClient, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create API handler: %v", err)
	}
	log.Printf("✅ API handler created")

	// Configurer les routes
	router := api.SetupRoutes(handler, cfg)
	log.Printf("✅ Routes configured")

	// Créer le serveur HTTP
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	// Démarrer le serveur dans une goroutine
	go func() {
		log.Printf("🌐 Server listening on http://%s", addr)
		log.Printf("📚 API documentation: http://%s/api/v1", addr)
		if cfg.WebUI.Enabled {
			log.Printf("🚀 Web UI (Angular): http://%s/", addr)
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	// Attendre un signal d'arrêt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// Arrêt gracieux
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped gracefully")
}
