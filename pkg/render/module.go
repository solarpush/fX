package render

import (
	"context"
	"sort"
	"strings"
	"sync"
)

// Module defines a pluggable document capability executed along the rendering lifecycle:
//   - PreValidate: validate business/compliance inputs before compilation
//   - PreCompile: enrich CompileJob (virtual files, attachments, format upgrade, Typst directives)
//   - PostCompile: transform compiled output bytes (e.g. Factur-X PDF/A-3 XMP & EmbeddedFiles patch)
type Module interface {
	Name() string
	Description() string
	Schema() map[string]interface{}
	PreValidate(ctx context.Context, req *RenderRequest, opts map[string]interface{}) error
	PreCompile(ctx context.Context, job *CompileJob, req *RenderRequest, opts map[string]interface{}) error
	PostCompile(ctx context.Context, out *CompileOutput, req *RenderRequest, opts map[string]interface{}) error
}

// ModuleInfo describes a registered module for discovery endpoints (e.g. /api/v1/capabilities).
type ModuleInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

type moduleAlias struct {
	target      string
	defaultOpts map[string]interface{}
}

// ModuleRegistry stores registered render modules and optional aliases.
type ModuleRegistry struct {
	mu      sync.RWMutex
	modules map[string]Module
	aliases map[string]moduleAlias
}

// NewModuleRegistry creates an empty, thread-safe ModuleRegistry.
func NewModuleRegistry() *ModuleRegistry {
	return &ModuleRegistry{
		modules: make(map[string]Module),
		aliases: make(map[string]moduleAlias),
	}
}

// Register registers (or replaces) a module under its normalized canonical Name().
func (r *ModuleRegistry) Register(m Module) {
	if m == nil {
		return
	}
	name := normalizeModuleName(m.Name())
	if name == "" {
		return
	}
	r.mu.Lock()
	r.modules[name] = m
	r.mu.Unlock()
}

// RegisterAlias maps an alias name (e.g. "facturx-en16931") to a registered
// canonical module (e.g. "facturx") with optional default options.
func (r *ModuleRegistry) RegisterAlias(alias, target string, defaultOpts map[string]interface{}) {
	a := normalizeModuleName(alias)
	t := normalizeModuleName(target)
	if a == "" || t == "" {
		return
	}
	var copied map[string]interface{}
	if len(defaultOpts) > 0 {
		copied = make(map[string]interface{}, len(defaultOpts))
		for k, v := range defaultOpts {
			copied[k] = v
		}
	}
	r.mu.Lock()
	r.aliases[a] = moduleAlias{target: t, defaultOpts: copied}
	r.mu.Unlock()
}

// Get retrieves a registered module by canonical name or alias.
func (r *ModuleRegistry) Get(name string) (Module, bool) {
	key := normalizeModuleName(name)
	if key == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.modules[key]; ok {
		return m, true
	}
	if al, ok := r.aliases[key]; ok {
		m, exists := r.modules[al.target]
		return m, exists
	}
	return nil, false
}

// Resolve looks up a ModuleConfig by canonical name or alias and merges any
// alias default options with caller-provided options (caller options win).
func (r *ModuleRegistry) Resolve(cfg ModuleConfig) (Module, map[string]interface{}, bool) {
	key := normalizeModuleName(cfg.Name)
	if key == "" {
		return nil, nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if m, ok := r.modules[key]; ok {
		return m, cloneOptions(cfg.Options), true
	}
	if al, ok := r.aliases[key]; ok {
		m, exists := r.modules[al.target]
		if !exists {
			return nil, nil, false
		}
		merged := cloneOptions(al.defaultOpts)
		if merged == nil && len(cfg.Options) > 0 {
			merged = make(map[string]interface{}, len(cfg.Options))
		}
		for k, v := range cfg.Options {
			merged[k] = v
		}
		return m, merged, true
	}
	return nil, nil, false
}

// List returns metadata for all registered canonical modules sorted by Name.
func (r *ModuleRegistry) List() []ModuleInfo {
	r.mu.RLock()
	out := make([]ModuleInfo, 0, len(r.modules))
	for _, m := range r.modules {
		out = append(out, ModuleInfo{
			Name:        m.Name(),
			Description: m.Description(),
			Schema:      m.Schema(),
		})
	}
	r.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeModuleName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func cloneOptions(opts map[string]interface{}) map[string]interface{} {
	if len(opts) == 0 {
		return nil
	}
	cp := make(map[string]interface{}, len(opts))
	for k, v := range opts {
		cp[k] = v
	}
	return cp
}
