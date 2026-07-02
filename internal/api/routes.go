package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/solarpush/fx/internal/config"
)

// spaHandler implements the http.Handler interface, so we can use it
// to respond to HTTP requests. The path to the static directory and
// path to the index file within that static directory are used to
// serve the SPA in the given static directory.
type spaHandler struct {
	staticPath string
	indexPath  string
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path, err := filepath.Abs(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path = filepath.Join(h.staticPath, path)
	_, err = os.Stat(path)
	if os.IsNotExist(err) {
		http.ServeFile(w, r, filepath.Join(h.staticPath, h.indexPath))
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.FileServer(http.Dir(h.staticPath)).ServeHTTP(w, r)
}

// SetupRoutes configure toutes les routes de l'API
func SetupRoutes(handler *Handler, cfg *config.Config) *mux.Router {
	r := mux.NewRouter()

	// Middlewares globaux
	r.Use(RecoveryMiddleware)
	r.Use(LoggingMiddleware)
	r.Use(CORSMiddleware)

	// API v1
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(ContentTypeMiddleware)

	// --- Routes Publiques (Auth non requise) ---
	api.HandleFunc("", handler.HandleAPIInfo).Methods(http.MethodGet)
	api.HandleFunc("/", handler.HandleAPIInfo).Methods(http.MethodGet)
	api.HandleFunc("/auth/login", handler.HandleLogin).Methods(http.MethodPost, http.MethodOptions)
	api.HandleFunc("/auth/logout", handler.HandleLogout).Methods(http.MethodPost, http.MethodOptions)

	// --- Routes Protégées (Auth requise) ---
	protectedApi := api.PathPrefix("").Subrouter()
	protectedApi.Use(AuthMiddleware(cfg))

	protectedApi.HandleFunc("/auth/me", handler.HandleMe).Methods(http.MethodGet, http.MethodOptions)

	// Routes de génération
	protectedApi.HandleFunc("/generate", handler.HandleGenerateFacturX).Methods(http.MethodPost, http.MethodOptions)
	protectedApi.HandleFunc("/generate/pdf", handler.HandleGeneratePDF).Methods(http.MethodPost, http.MethodOptions)
	protectedApi.HandleFunc("/generate/xml", handler.HandleGenerateXML).Methods(http.MethodPost, http.MethodOptions)

	// Routes de validation et extraction
	protectedApi.HandleFunc("/validate", handler.HandleValidate).Methods(http.MethodPost, http.MethodOptions)
	protectedApi.HandleFunc("/extract", handler.HandleExtract).Methods(http.MethodPost, http.MethodOptions)

	// Routes de templates
	protectedApi.HandleFunc("/templates", handler.HandleListTemplates).Methods(http.MethodGet, http.MethodOptions)
	protectedApi.HandleFunc("/templates", handler.HandleCreateTemplate).Methods(http.MethodPost, http.MethodOptions)
	protectedApi.HandleFunc("/templates/{id}", handler.HandleGetTemplate).Methods(http.MethodGet, http.MethodOptions)
	protectedApi.HandleFunc("/templates/{id}", handler.HandleUpdateTemplate).Methods(http.MethodPut, http.MethodOptions)
	protectedApi.HandleFunc("/templates/{id}", handler.HandleDeleteTemplate).Methods(http.MethodDelete, http.MethodOptions)
	protectedApi.HandleFunc("/templates/preview", handler.HandleCompilePreview).Methods(http.MethodPost, http.MethodOptions)

	// Route IA
	protectedApi.HandleFunc("/ai/generate", handler.HandleAIGenerate).Methods(http.MethodPost, http.MethodOptions)

	// --- Routes Custom Templates (feature-flag ALLOW_CUSTOM_TEMPLATES) ---
	// Scope isolé du flux Factur-X : template Typst libre + JSON Schema.
	if cfg.Features.AllowCustomTemplates {
		protectedApi.HandleFunc("/custom/templates", handler.HandleListCustomTemplates).Methods(http.MethodGet, http.MethodOptions)
		protectedApi.HandleFunc("/custom/templates", handler.HandleCreateCustomTemplate).Methods(http.MethodPost, http.MethodOptions)
		protectedApi.HandleFunc("/custom/templates/{id}", handler.HandleGetCustomTemplate).Methods(http.MethodGet, http.MethodOptions)
		protectedApi.HandleFunc("/custom/templates/{id}", handler.HandleUpdateCustomTemplate).Methods(http.MethodPut, http.MethodOptions)
		protectedApi.HandleFunc("/custom/templates/{id}", handler.HandleDeleteCustomTemplate).Methods(http.MethodDelete, http.MethodOptions)
		protectedApi.HandleFunc("/custom/validate", handler.HandleCustomValidate).Methods(http.MethodPost, http.MethodOptions)
		protectedApi.HandleFunc("/custom/preview", handler.HandleCustomPreview).Methods(http.MethodPost, http.MethodOptions)
		protectedApi.HandleFunc("/custom/generate", handler.HandleCustomGenerate).Methods(http.MethodPost, http.MethodOptions)
		protectedApi.HandleFunc("/custom/ai/generate", handler.HandleCustomAIGenerate).Methods(http.MethodPost, http.MethodOptions)
	}

	// Routes des images
	protectedApi.HandleFunc("/images", handler.HandleListImages).Methods(http.MethodGet, http.MethodOptions)
	protectedApi.HandleFunc("/images", handler.HandleUploadImage).Methods(http.MethodPost, http.MethodOptions)
	protectedApi.HandleFunc("/images/{filename}", handler.HandleDeleteImage).Methods(http.MethodDelete, http.MethodOptions)

	// WebSocket pour le live preview (plus performant)
	// Le paramètre token est traité dans le middleware
	protectedApi.HandleFunc("/ws/preview", handler.HandlePreviewWebSocket)

	// Health check (public)
	api.HandleFunc("/health", handler.HandleHealth).Methods(http.MethodGet)

	// Configuration publique (feature flags) — utilisée par le frontend
	api.HandleFunc("/config", handler.HandlePublicConfig).Methods(http.MethodGet, http.MethodOptions)

	// Servir le dossier d'images publiquement
	imagesPath := os.Getenv("IMAGES_PATH")
	if imagesPath == "" {
		imagesPath = "./images"
		// Ajustement du chemin si on est dans bin/
		if wd, err := os.Getwd(); err == nil && filepath.Base(wd) == "bin" {
			imagesPath = filepath.Join(filepath.Dir(wd), "images")
		}
	}
	// S'assurer que le dossier existe
	os.MkdirAll(imagesPath, 0755)
	
	imagesFs := http.FileServer(http.Dir(imagesPath))
	r.PathPrefix("/images/").Handler(http.StripPrefix("/images/", imagesFs))

	// Web UI Angular (si activé)
	if cfg.WebUI.Enabled {
		webPath := cfg.WebUI.Path
		// Ajuster le chemin si nécessaire (comme pour le builder legacy)
		if wd, err := os.Getwd(); err == nil && filepath.Base(wd) == "bin" {
			webPath = filepath.Join(filepath.Dir(wd), "web", "ng")
		}
		
		spa := spaHandler{staticPath: webPath, indexPath: "index.html"}
		r.PathPrefix("/").Handler(spa)
	}

	return r
}
