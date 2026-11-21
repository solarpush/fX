// Package registry provides the unified filesystem-backed Template Registry for
// the fX v2 Universal Document Engine. It manages v2 template bundles (Blueprint,
// native Typst, JSON Schema, sample data, and version snapshots) while remaining
// 100% interoperable with legacy Factur-X (*.typ) and custom (custom/*.typ)
// templates.
package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/solarpush/fx/pkg/blueprint"
)

const (
	// DefaultBaseDir is the default root directory for template storage.
	DefaultBaseDir = "./templates-custom"

	// MaxVersions is the maximum number of historical version snapshots kept per template.
	MaxVersions = 20

	maxIDLength = 128
)

var (
	// ErrNotFound is returned when a requested template does not exist.
	ErrNotFound = errors.New("template not found")
	// ErrInvalidID is returned when a template ID is empty or unsafe (e.g. path traversal).
	ErrInvalidID = errors.New("invalid template id")
	// ErrVersionNotFound is returned when a requested template version does not exist.
	ErrVersionNotFound = errors.New("template version not found")
)

// TemplateKind classifies how a template is authored and stored.
type TemplateKind string

const (
	KindBlueprint TemplateKind = "blueprint"
	KindTypst     TemplateKind = "typst"
	KindFacturX   TemplateKind = "facturx"
	KindCustom    TemplateKind = "custom"
)

// IsValid reports whether k is a known TemplateKind.
func (k TemplateKind) IsValid() bool {
	switch k {
	case KindBlueprint, KindTypst, KindFacturX, KindCustom:
		return true
	default:
		return false
	}
}

// TemplateMeta holds lightweight template metadata returned by List operations.
type TemplateMeta struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Category    string       `json:"category,omitempty"`
	Kind        TemplateKind `json:"kind"`
	EngineMode  string       `json:"engineMode,omitempty"` // "auto"|"native"|"blueprint"|"legacy"
	Modules     []string     `json:"modules,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Version     int          `json:"version"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// TemplateRecord is the complete persisted representation of a template bundle.
type TemplateRecord struct {
	TemplateMeta
	Content    string               `json:"content,omitempty"`
	Blueprint  *blueprint.Blueprint `json:"blueprint,omitempty"`
	Schema     string               `json:"schema,omitempty"`
	SampleData string               `json:"sampleData,omitempty"`
}

// TemplateVersion summarizes a single historical version snapshot.
type TemplateVersion struct {
	Version   int          `json:"version"`
	UpdatedAt time.Time    `json:"updatedAt"`
	Kind      TemplateKind `json:"kind"`
	Summary   string       `json:"summary,omitempty"`
}

type versionSnapshot struct {
	Version   int            `json:"version"`
	UpdatedAt time.Time      `json:"updatedAt"`
	Kind      TemplateKind   `json:"kind"`
	Summary   string         `json:"summary,omitempty"`
	Record    TemplateRecord `json:"record"`
}

// Registry manages v2 template bundles and legacy templates under baseDir.
type Registry struct {
	mu      sync.RWMutex
	baseDir string
}

// New creates a filesystem-backed Registry rooted at baseDir (defaults to DefaultBaseDir).
func New(baseDir string) *Registry {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = DefaultBaseDir
	}
	return &Registry{
		baseDir: filepath.Clean(baseDir),
	}
}

// BaseDir returns the root directory used by the registry.
func (r *Registry) BaseDir() string {
	return r.baseDir
}

// SanitizeID strictly validates and normalizes a template identifier.
// It rejects empty IDs, path separators, traversal sequences (".."), null bytes,
// leading/trailing dots, and unsafe characters, while stripping known legacy
// file extensions (".typ", ".schema.json").
func SanitizeID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", fmt.Errorf("%w: empty id", ErrInvalidID)
	}
	if strings.ContainsAny(id, "/\\\x00") || strings.Contains(id, "..") {
		return "", fmt.Errorf("%w: unsafe characters in %q", ErrInvalidID, raw)
	}

	lower := strings.ToLower(id)
	switch {
	case strings.HasSuffix(lower, ".schema.json"):
		id = id[:len(id)-len(".schema.json")]
	case strings.HasSuffix(lower, ".typ"):
		id = id[:len(id)-len(".typ")]
	}

	if id == "" || len(id) > maxIDLength {
		return "", fmt.Errorf("%w: invalid length for %q", ErrInvalidID, raw)
	}
	if strings.HasPrefix(id, ".") || strings.HasSuffix(id, ".") || strings.Contains(id, "..") {
		return "", fmt.Errorf("%w: invalid dot placement in %q", ErrInvalidID, raw)
	}

	for _, ch := range id {
		if !isAllowedIDRune(ch) {
			return "", fmt.Errorf("%w: disallowed character %q in %q", ErrInvalidID, ch, raw)
		}
	}

	return id, nil
}

func isAllowedIDRune(r rune) bool {
	return (r >= 'a' && r <= 'z') ||
		(r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') ||
		r == '-' || r == '_' || r == '.'
}

func slugifyName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var sb strings.Builder
	prevDash := false
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
			sb.WriteRune(r)
			prevDash = false
		case r == '-' || r == ' ' || r == '.':
			if !prevDash && sb.Len() > 0 {
				sb.WriteByte('-')
				prevDash = true
			}
		}
		if sb.Len() >= maxIDLength {
			break
		}
	}
	return strings.Trim(sb.String(), "-.")
}

func (r *Registry) v2Dir(id string) (string, error) {
	v2Root := filepath.Join(r.baseDir, "v2")
	target := filepath.Join(v2Root, id)
	if err := ensureWithin(v2Root, target); err != nil {
		return "", err
	}
	return target, nil
}

func (r *Registry) legacyCustomPaths(id string) (typPath, schemaPath string, err error) {
	customRoot := filepath.Join(r.baseDir, "custom")
	typPath = filepath.Join(customRoot, id+".typ")
	schemaPath = filepath.Join(customRoot, id+".schema.json")
	if err := ensureWithin(customRoot, typPath); err != nil {
		return "", "", err
	}
	return typPath, schemaPath, nil
}

func (r *Registry) legacyFacturXPath(id string) (string, error) {
	typPath := filepath.Join(r.baseDir, id+".typ")
	if err := ensureWithin(r.baseDir, typPath); err != nil {
		return "", err
	}
	return typPath, nil
}

func ensureWithin(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("%w: path escapes registry root", ErrInvalidID)
	}
	return nil
}

// List returns metadata for all templates across v2 bundles, legacy custom
// templates, and legacy root Factur-X templates. If kindFilter is non-empty
// (and not "all"), results are filtered to matching TemplateKind.
func (r *Registry) List(kindFilter string) ([]TemplateMeta, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	filter := strings.ToLower(strings.TrimSpace(kindFilter))
	if filter == "all" {
		filter = ""
	}

	seen := make(map[string]struct{})
	result := make([]TemplateMeta, 0)

	// 1. v2 bundles under <baseDir>/v2/<id>/
	v2Root := filepath.Join(r.baseDir, "v2")
	if entries, err := os.ReadDir(v2Root); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id, err := SanitizeID(entry.Name())
			if err != nil {
				continue
			}
			meta, err := r.readV2Meta(filepath.Join(v2Root, entry.Name()))
			if err != nil {
				continue
			}
			if meta.ID == "" {
				meta.ID = id
			}
			seen[meta.ID] = struct{}{}
			if filter == "" || string(meta.Kind) == filter {
				result = append(result, meta)
			}
		}
	}

	// 2. Legacy custom templates under <baseDir>/custom/*.typ
	customRoot := filepath.Join(r.baseDir, "custom")
	if entries, err := os.ReadDir(customRoot); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".typ") {
				continue
			}
			id, err := SanitizeID(entry.Name())
			if err != nil {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			if filter != "" && filter != string(KindCustom) {
				continue
			}
			info, _ := entry.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}
			result = append(result, TemplateMeta{
				ID:         id,
				Name:       id,
				Kind:       KindCustom,
				EngineMode: "auto",
				Version:    1,
				CreatedAt:  modTime,
				UpdatedAt:  modTime,
			})
		}
	}

	// 3. Legacy Factur-X templates under <baseDir>/*.typ
	if entries, err := os.ReadDir(r.baseDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".typ") {
				continue
			}
			id, err := SanitizeID(entry.Name())
			if err != nil {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			if filter != "" && filter != string(KindFacturX) {
				continue
			}
			info, _ := entry.Info()
			modTime := time.Time{}
			if info != nil {
				modTime = info.ModTime()
			}
			result = append(result, TemplateMeta{
				ID:         id,
				Name:       id,
				Kind:       KindFacturX,
				EngineMode: "auto",
				Modules:    []string{"facturx"},
				Version:    1,
				CreatedAt:  modTime,
				UpdatedAt:  modTime,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if !result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].UpdatedAt.After(result[j].UpdatedAt)
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// Get resolves a template by ID, checking v2 bundles first, then legacy custom
// templates (<baseDir>/custom/<id>.typ), and finally legacy Factur-X root
// templates (<baseDir>/<id>.typ).
func (r *Registry) Get(id string) (*TemplateRecord, error) {
	cleanID, err := SanitizeID(id)
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getUnlocked(cleanID)
}

func (r *Registry) getUnlocked(cleanID string) (*TemplateRecord, error) {
	// 1. Check v2 bundle
	dir, err := r.v2Dir(cleanID)
	if err != nil {
		return nil, err
	}
	if rec, err := r.readV2Record(dir); err == nil {
		return rec, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// 2. Fallback to legacy custom/<id>.typ (+ .schema.json)
	customTyp, customSchema, err := r.legacyCustomPaths(cleanID)
	if err != nil {
		return nil, err
	}
	if content, err := os.ReadFile(customTyp); err == nil {
		info, _ := os.Stat(customTyp)
		modTime := time.Time{}
		if info != nil {
			modTime = info.ModTime()
		}
		schemaBytes, _ := os.ReadFile(customSchema)
		return &TemplateRecord{
			TemplateMeta: TemplateMeta{
				ID:         cleanID,
				Name:       cleanID,
				Kind:       KindCustom,
				EngineMode: "auto",
				Version:    1,
				CreatedAt:  modTime,
				UpdatedAt:  modTime,
			},
			Content: string(content),
			Schema:  string(schemaBytes),
		}, nil
	}

	// 3. Fallback to legacy root <id>.typ (Factur-X)
	facturXTyp, err := r.legacyFacturXPath(cleanID)
	if err != nil {
		return nil, err
	}
	if content, err := os.ReadFile(facturXTyp); err == nil {
		info, _ := os.Stat(facturXTyp)
		modTime := time.Time{}
		if info != nil {
			modTime = info.ModTime()
		}
		return &TemplateRecord{
			TemplateMeta: TemplateMeta{
				ID:         cleanID,
				Name:       cleanID,
				Kind:       KindFacturX,
				EngineMode: "auto",
				Modules:    []string{"facturx"},
				Version:    1,
				CreatedAt:  modTime,
				UpdatedAt:  modTime,
			},
			Content: string(content),
		}, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrNotFound, cleanID)
}

// Save creates or updates a v2 template bundle, auto-compiling Blueprints when
// Kind == KindBlueprint and Blueprint != nil, incrementing the version number,
// and writing a version snapshot (retaining the last MaxVersions snapshots).
func (r *Registry) Save(rec *TemplateRecord) (*TemplateRecord, error) {
	if rec == nil {
		return nil, errors.New("template record is nil")
	}

	rawID := strings.TrimSpace(rec.ID)
	if rawID == "" && strings.TrimSpace(rec.Name) != "" {
		rawID = slugifyName(rec.Name)
	}
	cleanID, err := SanitizeID(rawID)
	if err != nil {
		return nil, err
	}

	// Work on a copy so caller input is not unexpectedly mutated on error.
	saved := *rec
	saved.ID = cleanID
	if strings.TrimSpace(saved.Name) == "" {
		saved.Name = cleanID
	}
	if saved.Modules != nil {
		saved.Modules = append([]string(nil), saved.Modules...)
	}
	if saved.Tags != nil {
		saved.Tags = append([]string(nil), saved.Tags...)
	}

	if saved.Kind == "" {
		if saved.Blueprint != nil {
			saved.Kind = KindBlueprint
		} else {
			saved.Kind = KindTypst
		}
	}
	if !saved.Kind.IsValid() {
		return nil, fmt.Errorf("invalid template kind %q", saved.Kind)
	}

	// Auto-compile Blueprint into native Typst when Kind == blueprint.
	if saved.Blueprint != nil && saved.Kind == KindBlueprint {
		compiled, err := blueprint.Compile(saved.Blueprint)
		if err != nil {
			return nil, fmt.Errorf("compile blueprint: %w", err)
		}
		saved.Content = compiled
		if saved.EngineMode == "" {
			saved.EngineMode = "blueprint"
		}
		if len(saved.Modules) == 0 && len(saved.Blueprint.Meta.Modules) > 0 {
			saved.Modules = append([]string(nil), saved.Blueprint.Meta.Modules...)
		}
		if saved.Category == "" && saved.Blueprint.Meta.Category != "" {
			saved.Category = saved.Blueprint.Meta.Category
		}
		if saved.Description == "" && saved.Blueprint.Meta.Description != "" {
			saved.Description = saved.Blueprint.Meta.Description
		}
	}

	if saved.EngineMode == "" {
		saved.EngineMode = "auto"
	}

	if strings.TrimSpace(saved.Schema) != "" && !json.Valid([]byte(saved.Schema)) {
		return nil, errors.New("invalid schema JSON")
	}
	if strings.TrimSpace(saved.SampleData) != "" && !json.Valid([]byte(saved.SampleData)) {
		return nil, errors.New("invalid sampleData JSON")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dir, err := r.v2Dir(cleanID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	existingMeta, err := r.readV2Meta(dir)
	if err == nil {
		saved.Version = existingMeta.Version + 1
		if !existingMeta.CreatedAt.IsZero() {
			saved.CreatedAt = existingMeta.CreatedAt
		} else {
			saved.CreatedAt = now
		}
	} else {
		saved.Version = 1
		saved.CreatedAt = now
	}
	saved.UpdatedAt = now

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create template directory: %w", err)
	}

	metaBytes, err := json.MarshalIndent(saved.TemplateMeta, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaBytes, 0644); err != nil {
		return nil, fmt.Errorf("write meta.json: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "template.typ"), []byte(saved.Content), 0644); err != nil {
		return nil, fmt.Errorf("write template.typ: %w", err)
	}

	bpPath := filepath.Join(dir, "blueprint.json")
	if saved.Blueprint != nil {
		bpBytes, err := json.MarshalIndent(saved.Blueprint, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal blueprint: %w", err)
		}
		if err := os.WriteFile(bpPath, bpBytes, 0644); err != nil {
			return nil, fmt.Errorf("write blueprint.json: %w", err)
		}
	} else {
		_ = os.Remove(bpPath)
	}

	schemaPath := filepath.Join(dir, "schema.json")
	if strings.TrimSpace(saved.Schema) != "" {
		if err := os.WriteFile(schemaPath, []byte(saved.Schema), 0644); err != nil {
			return nil, fmt.Errorf("write schema.json: %w", err)
		}
	} else {
		_ = os.Remove(schemaPath)
	}

	samplePath := filepath.Join(dir, "sample.json")
	if strings.TrimSpace(saved.SampleData) != "" {
		if err := os.WriteFile(samplePath, []byte(saved.SampleData), 0644); err != nil {
			return nil, fmt.Errorf("write sample.json: %w", err)
		}
	} else {
		_ = os.Remove(samplePath)
	}

	if err := r.writeVersionSnapshot(dir, saved); err != nil {
		return nil, err
	}

	out := saved
	return &out, nil
}

// Delete removes a template by ID from v2 storage, or falls back to removing a
// legacy custom or root Factur-X template if no v2 bundle exists.
func (r *Registry) Delete(id string) error {
	cleanID, err := SanitizeID(id)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	dir, err := r.v2Dir(cleanID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err == nil {
		return os.RemoveAll(dir)
	}

	customTyp, customSchema, err := r.legacyCustomPaths(cleanID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(customTyp); err == nil {
		if err := os.Remove(customTyp); err != nil {
			return err
		}
		_ = os.Remove(customSchema)
		return nil
	}

	facturXTyp, err := r.legacyFacturXPath(cleanID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(facturXTyp); err == nil {
		return os.Remove(facturXTyp)
	}

	return fmt.Errorf("%w: %s", ErrNotFound, cleanID)
}

// ListVersions returns all stored version snapshots for a v2 template sorted
// from newest to oldest.
func (r *Registry) ListVersions(id string) ([]TemplateVersion, error) {
	cleanID, err := SanitizeID(id)
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	dir, err := r.v2Dir(cleanID)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		// Check if template exists only as legacy (no version snapshots).
		if _, legErr := r.getUnlocked(cleanID); legErr != nil {
			return nil, legErr
		}
		return []TemplateVersion{}, nil
	}

	versionsDir := filepath.Join(dir, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []TemplateVersion{}, nil
		}
		return nil, fmt.Errorf("read versions dir: %w", err)
	}

	versions := make([]TemplateVersion, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if _, ok := parseVersionFilename(entry.Name()); !ok {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(versionsDir, entry.Name()))
		if err != nil {
			continue
		}
		var snap versionSnapshot
		if err := json.Unmarshal(raw, &snap); err != nil {
			continue
		}
		versions = append(versions, TemplateVersion{
			Version:   snap.Version,
			UpdatedAt: snap.UpdatedAt,
			Kind:      snap.Kind,
			Summary:   snap.Summary,
		})
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version > versions[j].Version
	})

	return versions, nil
}

// GetVersion loads the full TemplateRecord from a specific historical version snapshot.
func (r *Registry) GetVersion(id string, version int) (*TemplateRecord, error) {
	cleanID, err := SanitizeID(id)
	if err != nil {
		return nil, err
	}
	if version <= 0 {
		return nil, fmt.Errorf("%w: v%d", ErrVersionNotFound, version)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	dir, err := r.v2Dir(cleanID)
	if err != nil {
		return nil, err
	}
	snapPath := filepath.Join(dir, "versions", fmt.Sprintf("v%d.json", version))
	raw, err := os.ReadFile(snapPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s v%d", ErrVersionNotFound, cleanID, version)
		}
		return nil, fmt.Errorf("read version snapshot: %w", err)
	}

	var snap versionSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("parse version snapshot: %w", err)
	}

	rec := snap.Record
	return &rec, nil
}

// RestoreVersion loads an earlier snapshot and saves it as a new incremented version.
func (r *Registry) RestoreVersion(id string, version int) (*TemplateRecord, error) {
	rec, err := r.GetVersion(id, version)
	if err != nil {
		return nil, err
	}
	return r.Save(rec)
}

func (r *Registry) readV2Meta(dir string) (TemplateMeta, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return TemplateMeta{}, err
	}
	var meta TemplateMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return TemplateMeta{}, fmt.Errorf("parse meta.json: %w", err)
	}
	return meta, nil
}

func (r *Registry) readV2Record(dir string) (*TemplateRecord, error) {
	meta, err := r.readV2Meta(dir)
	if err != nil {
		return nil, err
	}

	rec := &TemplateRecord{
		TemplateMeta: meta,
	}

	if content, err := os.ReadFile(filepath.Join(dir, "template.typ")); err == nil {
		rec.Content = string(content)
	}
	if bpRaw, err := os.ReadFile(filepath.Join(dir, "blueprint.json")); err == nil && len(bpRaw) > 0 {
		var bp blueprint.Blueprint
		if err := json.Unmarshal(bpRaw, &bp); err != nil {
			return nil, fmt.Errorf("parse blueprint.json: %w", err)
		}
		rec.Blueprint = &bp
	}
	if schemaRaw, err := os.ReadFile(filepath.Join(dir, "schema.json")); err == nil {
		rec.Schema = string(schemaRaw)
	}
	if sampleRaw, err := os.ReadFile(filepath.Join(dir, "sample.json")); err == nil {
		rec.SampleData = string(sampleRaw)
	}

	return rec, nil
}

func (r *Registry) writeVersionSnapshot(dir string, rec TemplateRecord) error {
	versionsDir := filepath.Join(dir, "versions")
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return fmt.Errorf("create versions dir: %w", err)
	}

	summary := fmt.Sprintf("v%d (%s)", rec.Version, rec.Kind)
	if rec.Version == 1 {
		summary = fmt.Sprintf("Initial version (%s)", rec.Kind)
	}

	snap := versionSnapshot{
		Version:   rec.Version,
		UpdatedAt: rec.UpdatedAt,
		Kind:      rec.Kind,
		Summary:   summary,
		Record:    rec,
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal version snapshot: %w", err)
	}
	snapPath := filepath.Join(versionsDir, fmt.Sprintf("v%d.json", rec.Version))
	if err := os.WriteFile(snapPath, raw, 0644); err != nil {
		return fmt.Errorf("write version snapshot: %w", err)
	}

	// Prune older snapshots, retaining only the latest MaxVersions.
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return nil
	}
	var nums []int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if n, ok := parseVersionFilename(e.Name()); ok {
			nums = append(nums, n)
		}
	}
	if len(nums) <= MaxVersions {
		return nil
	}
	sort.Ints(nums)
	toDelete := nums[:len(nums)-MaxVersions]
	for _, n := range toDelete {
		_ = os.Remove(filepath.Join(versionsDir, fmt.Sprintf("v%d.json", n)))
	}
	return nil
}

func parseVersionFilename(name string) (int, bool) {
	if !strings.HasPrefix(name, "v") || !strings.HasSuffix(name, ".json") {
		return 0, false
	}
	numStr := name[1 : len(name)-len(".json")]
	n, err := strconv.Atoi(numStr)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
