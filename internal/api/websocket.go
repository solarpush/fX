package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/render"
	"github.com/solarpush/fx/pkg/schema"
	"github.com/solarpush/fx/pkg/template"
)

const wsMaxMessageBytes = 16 * 1024 * 1024 // 16 MiB for templates + inline base64 images

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 256, // 256KB read buffer
	WriteBufferSize: 1024 * 512, // 512KB write buffer
	CheckOrigin: func(r *http.Request) bool {
		return true // Autoriser toutes les origines (CORS)
	},
}

// WSPreviewRequest représente une demande de preview via WebSocket.
// Compatible avec l'ancien protocole Factur-X ({type: "preview", template, data})
// ainsi que le protocole Studio v2 complet (SVG/PDF, Blueprint, Schema, Modules, Assets).
type WSPreviewRequest struct {
	Type      string                 `json:"type"`                // "preview" | "compile" | "ping"
	RequestID string                 `json:"requestId,omitempty"` // Identifiant de corrélation
	Mode      string                 `json:"mode,omitempty"`      // "svg" | "pdf"
	Kind      string                 `json:"kind,omitempty"`      // "blueprint" | "typst" | "facturx"
	Template  string                 `json:"template,omitempty"`  // Code Typst
	Blueprint *blueprint.Blueprint   `json:"blueprint,omitempty"` // Blueprint JSON IR
	Schema    json.RawMessage        `json:"schema,omitempty"`    // JSON Schema optionnel
	Data      json.RawMessage        `json:"data,omitempty"`      // Données JSON
	Modules   []flexibleModuleConfig `json:"modules,omitempty"`   // Modules de rendu optionnels
	Assets    []customAsset          `json:"assets,omitempty"`    // Assets binaires base64 optionnels
}

// WSValidationInfo résume la validation du schéma et la couverture des champs du template.
type WSValidationInfo struct {
	Valid             bool     `json:"valid"`
	SchemaErrors      []string `json:"schemaErrors"`
	MissingInTemplate []string `json:"missingInTemplate"`
	UnknownInTemplate []string `json:"unknownInTemplate"`
}

// WSPreviewResponse représente la réponse de preview (SVG/PDF + diagnostics + validation).
type WSPreviewResponse struct {
	Type          string              `json:"type"`                    // "preview" | "error" | "pong"
	RequestID     string              `json:"requestId,omitempty"`     // Corrélation client
	Format        string              `json:"format,omitempty"`        // "svg" | "pdf" | "pdf-a3b"
	PagesSVG      []string            `json:"pagesSvg,omitempty"`      // Pages SVG (Studio v2)
	LayoutBoxes   []render.LayoutBox  `json:"layoutBoxes,omitempty"`   // Exact block layout bounding boxes (pt)
	PDF           string              `json:"pdf,omitempty"`           // PDF en base64 (rétro-compat & mode pdf)
	CompiledTypst string              `json:"compiledTypst,omitempty"` // Code Typst compilé depuis Blueprint
	Diagnostics   []render.Diagnostic `json:"diagnostics,omitempty"`   // Erreurs/avertissements Typst structurés
	Validation    *WSValidationInfo   `json:"validation,omitempty"`    // Couverture & validation JSON Schema
	Time          int64               `json:"time,omitempty"`          // Temps de rendu en ms
	Backend       string              `json:"backend,omitempty"`       // Backend actif ("cli", "rust-daemon", ...)
	Error         string              `json:"error,omitempty"`         // Message d'erreur lisible
}

// HandlePreviewWebSocket gère la connexion WebSocket pour le live preview v1 & v2
// avec annulation par connexion des previews en vol remplacées par une frappe plus récente.
func (h *Handler) HandlePreviewWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}
	// LIFO: conn.Close() s'exécute APRÈS l'arrêt de toutes les goroutines de preview.
	defer conn.Close()

	conn.SetReadLimit(wsMaxMessageBytes)

	log.Printf("[WS] Client connected from %s", r.RemoteAddr)

	connCtx, connCancel := context.WithCancel(r.Context())

	var (
		writeMu    sync.Mutex
		inFlightMu sync.Mutex
		cancelPrev context.CancelFunc
		latestSeq  uint64
		wg         sync.WaitGroup
	)

	defer func() {
		inFlightMu.Lock()
		if cancelPrev != nil {
			cancelPrev()
			cancelPrev = nil
		}
		inFlightMu.Unlock()
		connCancel()
		wg.Wait()
	}()

	// Boucle de lecture des messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break
		}

		var req WSPreviewRequest
		if err := json.Unmarshal(message, &req); err != nil {
			sendWSResponse(conn, &writeMu, &WSPreviewResponse{
				Type:  "error",
				Error: "Invalid JSON: " + err.Error(),
			})
			continue
		}

		switch strings.ToLower(strings.TrimSpace(req.Type)) {
		case "preview", "compile":
			// Annuler toute compilation précédente encore en cours sur cette connexion
			// afin de ne jamais accumuler de travail CPU ni renvoyer de frame obsolète.
			inFlightMu.Lock()
			if cancelPrev != nil {
				cancelPrev()
			}
			latestSeq++
			seq := latestSeq
			jobCtx, jobCancel := context.WithCancel(connCtx)
			cancelPrev = jobCancel
			inFlightMu.Unlock()

			reqCopy := req
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer jobCancel()
				h.handleWSPreview(jobCtx, conn, &writeMu, &inFlightMu, &latestSeq, seq, &reqCopy)
			}()

		case "ping":
			sendWSResponse(conn, &writeMu, &WSPreviewResponse{
				Type:      "pong",
				RequestID: req.RequestID,
			})

		default:
			sendWSResponse(conn, &writeMu, &WSPreviewResponse{
				Type:      "error",
				RequestID: req.RequestID,
				Error:     "Unknown message type: " + req.Type,
			})
		}
	}

	log.Printf("[WS] Client disconnected from %s", r.RemoteAddr)
}

func (h *Handler) handleWSPreview(
	ctx context.Context,
	conn *websocket.Conn,
	writeMu *sync.Mutex,
	inFlightMu *sync.Mutex,
	latestSeq *uint64,
	seq uint64,
	req *WSPreviewRequest,
) {
	start := time.Now()

	sendIfCurrent := func(resp *WSPreviewResponse) {
		inFlightMu.Lock()
		current := (*latestSeq == seq) && (ctx.Err() == nil)
		inFlightMu.Unlock()
		if !current {
			return
		}
		sendWSResponse(conn, writeMu, resp)
	}

	backendName := ""
	if h.renderEngine != nil && h.renderEngine.Backend() != nil {
		backendName = h.renderEngine.Backend().Name()
	}

	// Déterminer le mode (svg/pdf) et la nature (blueprint/typst/facturx)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	mode := strings.ToLower(strings.TrimSpace(req.Mode))

	isLegacyFacturX := kind == "" &&
		mode == "" &&
		req.Blueprint == nil &&
		len(normalizeRawJSON(req.Schema)) == 0 &&
		len(req.Modules) == 0 &&
		len(req.Assets) == 0

	if kind == "" {
		switch {
		case req.Blueprint != nil:
			kind = "blueprint"
		case isLegacyFacturX:
			kind = "facturx"
		default:
			kind = "typst"
		}
	}

	if mode == "" {
		switch kind {
		case "blueprint", "typst":
			mode = "svg"
		default:
			mode = "pdf"
		}
	}

	targetFormat := render.FormatSVG
	if mode == "pdf" {
		targetFormat = render.FormatPDF
	} else if mode != "svg" {
		sendIfCurrent(&WSPreviewResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Time:      time.Since(start).Milliseconds(),
			Backend:   backendName,
			Error:     "Unsupported preview mode: " + req.Mode,
		})
		return
	}

	// Calcul de la validation / couverture JSON Schema (si un schéma est fourni)
	var validation *WSValidationInfo
	if len(normalizeRawJSON(req.Schema)) > 0 {
		refs := extractWSReferences(req)
		validation = evaluateSchemaCoverage(req.Schema, refs, req.Data)
	}

	// Décoder les assets éventuels
	assets, err := decodeRenderAssets(req.Assets)
	if err != nil {
		sendIfCurrent(&WSPreviewResponse{
			Type:       "error",
			RequestID:  req.RequestID,
			Format:     string(targetFormat),
			Validation: validation,
			Time:       time.Since(start).Milliseconds(),
			Backend:    backendName,
			Error:      err.Error(),
		})
		return
	}

	// Préparer les données (rétro-compat Factur-X avec getDefaultInvoice() si vide/partiel)
	dataPayload := req.Data
	if isLegacyFacturX || kind == "facturx" {
		if len(bytes.TrimSpace(dataPayload)) == 0 || bytes.Equal(bytes.TrimSpace(dataPayload), []byte("null")) {
			if defaultBytes, err := invoice.ToJSON(getDefaultInvoice()); err == nil {
				dataPayload = defaultBytes
			}
		} else if isLegacyFacturX {
			if parsed, err := invoice.FromJSON(dataPayload); err == nil {
				if serialized, serErr := invoice.ToJSON(parsed); serErr == nil {
					dataPayload = serialized
				}
			} else if defaultBytes, defErr := invoice.ToJSON(getDefaultInvoice()); defErr == nil {
				dataPayload = defaultBytes
			}
		}
	}

	// Fallback ultime sur l'ancien pipeline si le moteur de rendu unifié n'est pas câblé
	if h.renderEngine == nil {
		h.handleLegacyFallbackWSPreview(ctx, sendIfCurrent, req, dataPayload, validation, start)
		return
	}

	// Pré-compilation Blueprint optionnelle pour conserver compiledTypst même en cas
	// d'erreur Typst ultérieure.
	var precompiledTypst string
	if req.Blueprint != nil {
		if compiled, compErr := blueprint.Compile(req.Blueprint); compErr == nil {
			precompiledTypst = compiled
		}
	}

	engineMode := render.ModeAuto
	if kind == "blueprint" || req.Blueprint != nil {
		engineMode = render.ModeBlueprint
	}

	mods := toRenderModules(req.Modules)
	if isLegacyFacturX && len(mods) == 0 {
		mods = []render.ModuleConfig{{Name: "license"}}
	}

	renderReq := &render.RenderRequest{
		Template:   req.Template,
		Blueprint:  req.Blueprint,
		Schema:     normalizeRawJSON(req.Schema),
		Data:       dataPayload,
		Assets:     assets,
		Format:     targetFormat,
		Modules:    mods,
		EngineMode: engineMode,
	}

	res, err := h.renderEngine.Render(ctx, renderReq)
	if ctx.Err() != nil {
		// Requête remplacée par une nouvelle frappe ou connexion fermée.
		return
	}

	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		resp := &WSPreviewResponse{
			Type:          "error",
			RequestID:     req.RequestID,
			Format:        string(targetFormat),
			CompiledTypst: precompiledTypst,
			Validation:    validation,
			Time:          elapsed,
			Backend:       backendName,
			Error:         err.Error(),
		}
		var compileErr *render.CompileError
		if errors.As(err, &compileErr) {
			resp.Diagnostics = compileErr.Diagnostics
			resp.Error = compileErr.Error()
		}
		sendIfCurrent(resp)
		return
	}

	resp := &WSPreviewResponse{
		Type:          "preview",
		RequestID:     req.RequestID,
		Format:        string(res.Format),
		LayoutBoxes:   res.LayoutBoxes,
		CompiledTypst: res.CompiledTypst,
		Diagnostics:   res.Warnings,
		Validation:    validation,
		Time:          elapsed,
		Backend:       res.Backend,
	}

	if res.Format == render.FormatSVG {
		resp.PagesSVG = res.PagesSVG
	} else if len(res.Data) > 0 {
		resp.PDF = base64.StdEncoding.EncodeToString(res.Data)
	}

	log.Printf("[WS] Preview (%s/%s) rendered in %dms via %s (pages=%d)",
		kind, res.Format, elapsed, res.Backend, res.PageCount)

	sendIfCurrent(resp)
}

func extractWSReferences(req *WSPreviewRequest) []string {
	if req.Blueprint != nil {
		return blueprint.ExtractReferences(req.Blueprint)
	}
	src := req.Template
	if strings.TrimSpace(src) == "" {
		return nil
	}
	if strings.Contains(src, "{{") &&
		!strings.Contains(src, `json("data.json")`) &&
		!strings.Contains(src, `json("/data.json")`) {
		return template.ExtractReferences(src)
	}
	return schema.ExtractNativeReferences(src)
}

func (h *Handler) handleLegacyFallbackWSPreview(
	ctx context.Context,
	sendIfCurrent func(*WSPreviewResponse),
	req *WSPreviewRequest,
	dataPayload []byte,
	validation *WSValidationInfo,
	start time.Time,
) {
	if h.pipeline == nil {
		sendIfCurrent(&WSPreviewResponse{
			Type:       "error",
			RequestID:  req.RequestID,
			Validation: validation,
			Time:       time.Since(start).Milliseconds(),
			Error:      "compiler backend unavailable",
		})
		return
	}

	engine, err := template.New(dataPayload)
	if err != nil {
		sendIfCurrent(&WSPreviewResponse{
			Type:       "error",
			RequestID:  req.RequestID,
			Validation: validation,
			Time:       time.Since(start).Milliseconds(),
			Error:      "Template engine error: " + err.Error(),
		})
		return
	}

	filledTemplate, err := engine.Render(req.Template)
	if err != nil {
		sendIfCurrent(&WSPreviewResponse{
			Type:       "error",
			RequestID:  req.RequestID,
			Validation: validation,
			Time:       time.Since(start).Milliseconds(),
			Error:      "Template render error: " + err.Error(),
		})
		return
	}

	if ctx.Err() != nil {
		return
	}

	pdfBytes, err := h.compileTypstToBytes(filledTemplate)
	if err != nil {
		sendIfCurrent(&WSPreviewResponse{
			Type:       "error",
			RequestID:  req.RequestID,
			Validation: validation,
			Time:       time.Since(start).Milliseconds(),
			Error:      "Compilation error: " + err.Error(),
		})
		return
	}

	sendIfCurrent(&WSPreviewResponse{
		Type:       "preview",
		RequestID:  req.RequestID,
		Format:     string(render.FormatPDF),
		PDF:        base64.StdEncoding.EncodeToString(pdfBytes),
		Validation: validation,
		Time:       time.Since(start).Milliseconds(),
		Backend:    "legacy-cli",
	})
}

// compileTypstToBytes compile le code Typst et retourne les bytes du PDF (fallback legacy).
func (h *Handler) compileTypstToBytes(typstCode string) ([]byte, error) {
	tmpBase := "./tmp/typst-preview"
	_ = os.MkdirAll(tmpBase, 0755)

	tmpDir, err := os.MkdirTemp(tmpBase, "ws-preview-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	typstFile := filepath.Join(tmpDir, "template.typ")
	typstCode += "\n#pdf.attach(\"/license.txt\", bytes(\"Powered by fX\"), relationship: \"supplement\", description: \"License Info\", mime-type: \"text/plain\")\n"
	if err := os.WriteFile(typstFile, []byte(typstCode), 0644); err != nil {
		return nil, err
	}

	pdfFile := filepath.Join(tmpDir, "output.pdf")
	if err := h.pipeline.CompileFile(typstFile, pdfFile); err != nil {
		return nil, err
	}

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
