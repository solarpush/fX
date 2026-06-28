package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/solarpush/fx/internal/config"
)

// LoginRequest contient les identifiants
type LoginRequest struct {
	Password string `json:"password"`
}

// Claims du JWT
type Claims struct {
	jwt.RegisteredClaims
}

// HandleLogin vérifie le mot de passe et crée un cookie JWT
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.Enabled {
		w.WriteHeader(http.StatusOK)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	if req.Password != h.cfg.Auth.Password {
		http.Error(w, "Mot de passe incorrect", http.StatusUnauthorized)
		return
	}

	// Création du JWT
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(h.cfg.Auth.JWTSecret))
	if err != nil {
		http.Error(w, "Erreur serveur", http.StatusInternalServerError)
		return
	}

	// Définition du cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "fx_token",
		Value:    tokenString,
		Expires:  expirationTime,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusOK)
}

// HandleLogout supprime le cookie
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "fx_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Path:     "/",
	})
	w.WriteHeader(http.StatusOK)
}

// HandleMe vérifie si l'utilisateur est authentifié
func (h *Handler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if !h.cfg.Auth.Enabled {
		w.WriteHeader(http.StatusOK)
		return
	}
	
	_, err := validateToken(r, h.cfg.Auth.JWTSecret)
	if err != nil {
		http.Error(w, "Non authentifié", http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// AuthMiddleware protège les routes
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.Auth.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// OPTIONS requêtes toujours autorisées
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Vérification API Key (pour l'API)
			apiKey := r.Header.Get("X-API-Key")
			if apiKey != "" {
				if apiKey == cfg.Auth.APIKey {
					next.ServeHTTP(w, r)
					return
				}
				http.Error(w, "Clé API invalide", http.StatusUnauthorized)
				return
			}

			// L'authentification par JWT est ignorée sur certaines routes via le routeur.
			// Ici on vérifie le cookie JWT
			_, err := validateToken(r, cfg.Auth.JWTSecret)
			if err != nil {
				// Autorisation par query parameter token pour websockets
				tokenQuery := r.URL.Query().Get("token")
				if tokenQuery != "" {
					_, errQuery := validateTokenString(tokenQuery, cfg.Auth.JWTSecret)
					if errQuery == nil {
						next.ServeHTTP(w, r)
						return
					}
				}

				http.Error(w, "Non autorisé", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func validateToken(r *http.Request, secret string) (*Claims, error) {
	cookie, err := r.Cookie("fx_token")
	if err != nil {
		// Vérification Authorization Header pour les requêtes API (Bearer)
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			return validateTokenString(tokenStr, secret)
		}
		return nil, err
	}

	return validateTokenString(cookie.Value, secret)
}

func validateTokenString(tokenStr, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !token.Valid {
		return nil, http.ErrNoCookie
	}

	return claims, nil
}
