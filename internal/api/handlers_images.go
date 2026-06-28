package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
)

// ImageInfo représente les métadonnées d'une image
type ImageInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// getImagesDir retourne le chemin absolu du dossier d'images
func getImagesDir() string {
	imagesPath := os.Getenv("IMAGES_PATH")
	if imagesPath == "" {
		imagesPath = "./images"
		if wd, err := os.Getwd(); err == nil && filepath.Base(wd) == "bin" {
			imagesPath = filepath.Join(filepath.Dir(wd), "images")
		}
	}
	os.MkdirAll(imagesPath, 0755)
	return imagesPath
}

// HandleListImages retourne la liste des images disponibles
func (h *Handler) HandleListImages(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	imagesDir := getImagesDir()
	files, err := os.ReadDir(imagesDir)
	if err != nil {
		http.Error(w, "Impossible de lire le dossier d'images", http.StatusInternalServerError)
		return
	}

	var images []ImageInfo
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(f.Name()))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".svg" || ext == ".gif" {
			info, err := f.Info()
			var size int64
			if err == nil {
				size = info.Size()
			}

			images = append(images, ImageInfo{
				Name: f.Name(),
				URL:  "/images/" + f.Name(),
				Size: size,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

// HandleUploadImage permet d'uploader une nouvelle image
func (h *Handler) HandleUploadImage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Limite à 10 MB
	r.ParseMultipartForm(10 << 20)

	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Fichier image manquant", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Vérifier l'extension
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".svg" && ext != ".gif" {
		http.Error(w, "Format d'image non supporté", http.StatusBadRequest)
		return
	}

	// S'assurer que le nom de fichier est sûr
	filename := filepath.Base(handler.Filename)
	filename = strings.ReplaceAll(filename, " ", "-")

	// Chemin de destination
	imagesDir := getImagesDir()
	destPath := filepath.Join(imagesDir, filename)

	// Créer le fichier
	dst, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Erreur serveur lors de la création du fichier", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	// Copier les données
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Erreur lors de l'enregistrement de l'image", http.StatusInternalServerError)
		return
	}

	// Retourner l'information
	info, _ := dst.Stat()
	imageInfo := ImageInfo{
		Name: filename,
		URL:  "/images/" + filename,
		Size: info.Size(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(imageInfo)
}

// HandleDeleteImage supprime une image
func (h *Handler) HandleDeleteImage(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	vars := mux.Vars(r)
	filename := vars["filename"]
	if filename == "" {
		http.Error(w, "Nom de fichier requis", http.StatusBadRequest)
		return
	}

	// Empêcher le path traversal
	filename = filepath.Base(filename)

	imagesDir := getImagesDir()
	filePath := filepath.Join(imagesDir, filename)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Image non trouvée", http.StatusNotFound)
		return
	}

	if err := os.Remove(filePath); err != nil {
		http.Error(w, "Erreur lors de la suppression de l'image", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
