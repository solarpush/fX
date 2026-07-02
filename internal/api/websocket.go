package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/template"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,  // 64KB pour les templates
	WriteBufferSize: 1024 * 256, // 256KB pour les PDFs
	CheckOrigin: func(r *http.Request) bool {
		return true // Autoriser toutes les origines (CORS)
	},
}

// WSPreviewRequest représente une demande de preview via WebSocket
type WSPreviewRequest struct {
	Type     string         `json:"type"`     // "preview"
	Template string         `json:"template"` // Code Typst
	Data     map[string]any `json:"data"`     // Données de la facture
}

// WSPreviewResponse représente la réponse de preview
type WSPreviewResponse struct {
	Type  string `json:"type"`  // "preview" ou "error"
	PDF   string `json:"pdf"`   // PDF en base64
	Error string `json:"error"` // Message d'erreur si applicable
	Time  int64  `json:"time"`  // Temps de compilation en ms
}

// HandlePreviewWebSocket gère la connexion WebSocket pour le live preview
func (h *Handler) HandlePreviewWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}
	defer conn.Close()

	log.Printf("[WS] Client connected from %s", r.RemoteAddr)

	// Mutex pour éviter les écritures concurrentes
	var writeMu sync.Mutex

	// Boucle de lecture des messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		// Parser la requête
		var req WSPreviewRequest
		if err := json.Unmarshal(message, &req); err != nil {
			sendWSError(conn, &writeMu, "Invalid JSON: "+err.Error())
			continue
		}

		// Traiter selon le type
		switch req.Type {
		case "preview":
			go h.handleWSPreview(conn, &writeMu, &req)
		case "ping":
			sendWSResponse(conn, &writeMu, &WSPreviewResponse{Type: "pong"})
		default:
			sendWSError(conn, &writeMu, "Unknown message type: "+req.Type)
		}
	}

	log.Printf("[WS] Client disconnected from %s", r.RemoteAddr)
}

func (h *Handler) handleWSPreview(conn *websocket.Conn, mu *sync.Mutex, req *WSPreviewRequest) {
	start := time.Now()

	// Convertir map[string]any en Invoice
	inv := getDefaultInvoice()
	if req.Data != nil {
		// Essayer de parser les données fournies
		jsonBytes, _ := json.Marshal(req.Data)
		if parsed, err := invoice.FromJSON(jsonBytes); err == nil {
			inv = parsed
		}
	}

	// Convertir l'invoice en JSON pour le template engine
	jsonData, err := invoice.ToJSON(inv)
	if err != nil {
		sendWSError(conn, mu, "Serialization error: "+err.Error())
		return
	}

	// Remplir le template avec les données
	engine, err := template.New(jsonData)
	if err != nil {
		sendWSError(conn, mu, "Template engine error: "+err.Error())
		return
	}

	filledTemplate, err := engine.Render(req.Template)
	if err != nil {
		sendWSError(conn, mu, "Template render error: "+err.Error())
		return
	}

	// Compiler avec Typst
	pdfBytes, err := h.compileTypstToBytes(filledTemplate)
	if err != nil {
		sendWSError(conn, mu, "Compilation error: "+err.Error())
		return
	}

	// Encoder en base64
	pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	elapsed := time.Since(start).Milliseconds()
	log.Printf("[WS] Preview compiled in %dms, size: %d bytes", elapsed, len(pdfBytes))

	// Envoyer la réponse
	sendWSResponse(conn, mu, &WSPreviewResponse{
		Type: "preview",
		PDF:  pdfBase64,
		Time: elapsed,
	})
}

// compileTypstToBytes compile le code Typst et retourne les bytes du PDF
func (h *Handler) compileTypstToBytes(typstCode string) ([]byte, error) {
	tmpBase := "./tmp/typst-preview"
	_ = os.MkdirAll(tmpBase, 0755)

	tmpDir, err := os.MkdirTemp(tmpBase, "ws-preview-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Écrire le template
	typstFile := filepath.Join(tmpDir, "template.typ")
	typstCode += "\n#pdf.attach(\"/license.txt\", bytes(\"Powered by fX\"), relationship: \"supplement\", description: \"License Info\", mime-type: \"text/plain\")\n"
	if err := os.WriteFile(typstFile, []byte(typstCode), 0644); err != nil {
		return nil, err
	}

	// Compiler
	pdfFile := filepath.Join(tmpDir, "output.pdf")
	if err := h.pipeline.CompileFile(typstFile, pdfFile); err != nil {
		return nil, err
	}

	// Lire le PDF
	return os.ReadFile(pdfFile)
}

// getDefaultInvoice retourne une facture de test par défaut
func getDefaultInvoice() *invoice.Invoice {
	return &invoice.Invoice{
		Invoice: invoice.Details{
			Number:    "FAC-2024-001",
			IssueDate: time.Now(),
			Currency:  "EUR",
			Type:      "380",
		},
		Seller: invoice.Party{
			Name:         "Ma Société SAS",
			Registration: "12345678901234",
			VatID:        "FR12345678901",
			Address: invoice.Address{
				Street:     "123 Rue Example",
				PostalCode: "75001",
				City:       "Paris",
				Country:    "FR",
			},
		},
		Buyer: invoice.Party{
			Name: "Client SARL",
			Address: invoice.Address{
				Street:     "456 Avenue Test",
				PostalCode: "69001",
				City:       "Lyon",
				Country:    "FR",
			},
		},
		Lines: []invoice.Line{
			{Description: "Prestation de conseil", Quantity: 5, UnitPrice: 150, VatRate: 20},
			{Description: "Développement web", Quantity: 10, UnitPrice: 120, VatRate: 20},
		},
	}
}

func sendWSResponse(conn *websocket.Conn, mu *sync.Mutex, resp *WSPreviewResponse) {
	mu.Lock()
	defer mu.Unlock()

	if err := conn.WriteJSON(resp); err != nil {
		log.Printf("[WS] Write error: %v", err)
	}
}

func sendWSError(conn *websocket.Conn, mu *sync.Mutex, errMsg string) {
	sendWSResponse(conn, mu, &WSPreviewResponse{
		Type:  "error",
		Error: errMsg,
	})
}
