package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/solarpush/fx/internal/ai"
	"github.com/solarpush/fx/internal/api"
	"github.com/solarpush/fx/internal/config"
	"github.com/solarpush/fx/internal/storage"
)

const version = "2.0.0"

func main() {
	log.Printf("🚀 Starting fX Universal Document Engine & Factur-X Server v%s", version)

	// Charger la configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ Failed to load config: %v", err)
	}

	log.Printf("📋 Configuration loaded:")
	log.Printf("  - Server: %s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Printf("  - Storage: %s", cfg.Storage.Type)
	log.Printf("  - Web UI (Angular): %v", cfg.WebUI.Enabled)
	log.Printf("  - Studio Builder v2: %v", cfg.Features.StudioEnabled)
	log.Printf("  - Render Backend mode: %s (configured workers: %d, maxBatchSize: %d)",
		cfg.Render.Backend, cfg.Render.Workers, cfg.Render.MaxBatchSize)

	// Initialiser le storage
	ctx := context.Background()
	store, err := storage.NewStorage(ctx, cfg.Storage)
	if err != nil {
		log.Fatalf("❌ Failed to initialize storage: %v", err)
	}
	log.Printf("✅ Storage initialized: %s", cfg.Storage.Type)

	// Initialiser le client IA
	aiClient := ai.NewClient(cfg.AI)
	log.Printf("🤖 AI client initialized (Provider: %s, Model: %s)", cfg.AI.Provider, cfg.AI.Model)

	// Créer le handler API
	handler, err := api.NewHandler(store, aiClient, cfg)
	if err != nil {
		log.Fatalf("❌ Failed to create API handler: %v", err)
	}
	defer func() {
		if closeErr := handler.Close(); closeErr != nil {
			log.Printf("⚠️ Error closing render backend: %v", closeErr)
		}
	}()

	if engine := handler.RenderEngine(); engine != nil {
		if backend := engine.Backend(); backend != nil {
			stats := backend.Stats()
			log.Printf("⚙️ Render Engine ready: backend=%s workers=%d healthy=%v",
				stats.Name, stats.Workers, stats.Healthy)
		}
		if modReg := engine.Modules(); modReg != nil {
			mods := modReg.List()
			names := make([]string, 0, len(mods))
			for _, m := range mods {
				names = append(names, m.Name)
			}
			log.Printf("🧩 Registered Render Modules (%d): %s", len(names), strings.Join(names, ", "))
		}
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
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped gracefully")
}
