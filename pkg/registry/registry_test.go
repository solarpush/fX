package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/solarpush/fx/pkg/blueprint"
)

func TestSanitizeID_ValidAndTraversalRejection(t *testing.T) {
	validCases := []struct {
		in   string
		want string
	}{
		{"invoice-2026", "invoice-2026"},
		{"EN16931.typ", "EN16931"},
		{"gazette.schema.json", "gazette"},
		{"tpl_modern_saas", "tpl_modern_saas"},
		{"report.v2", "report.v2"},
		{"  clean-id  ", "clean-id"},
	}

	for _, tc := range validCases {
		got, err := SanitizeID(tc.in)
		if err != nil {
			t.Errorf("SanitizeID(%q) unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("SanitizeID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}

	invalidCases := []string{
		"",
		"   ",
		"../etc/passwd",
		"..\\windows\\system32",
		"foo/bar",
		"foo\\bar",
		"..",
		".",
		".hidden",
		".typ",
		"a..b",
		"evil.",
		"null\x00byte",
		"has spaces",
		"../v2/other",
		strings.Repeat("a", maxIDLength+1),
	}

	for _, bad := range invalidCases {
		got, err := SanitizeID(bad)
		if err == nil {
			t.Errorf("SanitizeID(%q) expected error, got %q", bad, got)
			continue
		}
		if !errors.Is(err, ErrInvalidID) {
			t.Errorf("SanitizeID(%q) error = %v, want ErrInvalidID", bad, err)
		}
	}
}

func TestRegistry_PathTraversalRejectedAcrossMethods(t *testing.T) {
	reg := New(t.TempDir())
	attacks := []string{"../etc/passwd", "foo/bar", "..\\secret", ".env"}

	for _, bad := range attacks {
		if _, err := reg.Get(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Get(%q) = %v, want ErrInvalidID", bad, err)
		}
		if _, err := reg.Save(&TemplateRecord{TemplateMeta: TemplateMeta{ID: bad}, Content: "= Hi"}); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Save(%q) = %v, want ErrInvalidID", bad, err)
		}
		if err := reg.Delete(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Delete(%q) = %v, want ErrInvalidID", bad, err)
		}
		if _, err := reg.ListVersions(bad); !errors.Is(err, ErrInvalidID) {
			t.Errorf("ListVersions(%q) = %v, want ErrInvalidID", bad, err)
		}
		if _, err := reg.GetVersion(bad, 1); !errors.Is(err, ErrInvalidID) {
			t.Errorf("GetVersion(%q, 1) = %v, want ErrInvalidID", bad, err)
		}
	}
}

func TestRegistry_CRUDAndBlueprintAutoCompile(t *testing.T) {
	baseDir := t.TempDir()
	reg := New(baseDir)

	preset, ok := blueprint.GetPreset("editorial-gazette")
	if !ok {
		t.Fatal("editorial-gazette preset not found")
	}

	bp := preset.Blueprint
	saved, err := reg.Save(&TemplateRecord{
		TemplateMeta: TemplateMeta{
			ID:   "my-gazette",
			Name: "My Gazette",
			Kind: KindBlueprint,
			Tags: []string{"newsletter", "v2"},
		},
		Blueprint:  &bp,
		Schema:     string(preset.Schema),
		SampleData: string(preset.SampleData),
	})
	if err != nil {
		t.Fatalf("Save blueprint failed: %v", err)
	}

	if saved.Version != 1 {
		t.Fatalf("Version = %d, want 1", saved.Version)
	}
	if !strings.Contains(saved.Content, `#import "/_fx/ui.typ": *`) {
		t.Errorf("expected auto-compiled Typst content, got: %q", saved.Content)
	}
	if saved.EngineMode != "blueprint" {
		t.Errorf("EngineMode = %q, want blueprint", saved.EngineMode)
	}
	if saved.Category != "newsletter" {
		t.Errorf("Category = %q, want inherited 'newsletter'", saved.Category)
	}

	// Verify bundle files exist on disk under v2/my-gazette/
	bundleDir := filepath.Join(baseDir, "v2", "my-gazette")
	for _, fname := range []string{"meta.json", "template.typ", "blueprint.json", "schema.json", "sample.json", filepath.Join("versions", "v1.json")} {
		if _, err := os.Stat(filepath.Join(bundleDir, fname)); err != nil {
			t.Errorf("expected file %s in bundle: %v", fname, err)
		}
	}

	// Read back via Get
	loaded, err := reg.Get("my-gazette")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if loaded.ID != "my-gazette" || loaded.Version != 1 || loaded.Blueprint == nil {
		t.Fatalf("unexpected loaded record: %+v", loaded)
	}
	if loaded.Content != saved.Content {
		t.Errorf("loaded Content mismatch")
	}

	// Invalid blueprint must be rejected before writing
	badBP := &blueprint.Blueprint{Version: "99.0"}
	if _, err := reg.Save(&TemplateRecord{
		TemplateMeta: TemplateMeta{ID: "bad-bp", Kind: KindBlueprint},
		Blueprint:    badBP,
	}); err == nil {
		t.Error("expected error saving invalid blueprint")
	}

	// Delete
	if err := reg.Delete("my-gazette"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := reg.Get("my-gazette"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete = %v, want ErrNotFound", err)
	}
}

func TestRegistry_VersioningPruningAndRestore(t *testing.T) {
	reg := New(t.TempDir())

	// Save 25 versions to verify pruning retains the last MaxVersions (20).
	for i := 1; i <= 25; i++ {
		saved, err := reg.Save(&TemplateRecord{
			TemplateMeta: TemplateMeta{
				ID:   "versioned-doc",
				Name: "Versioned Doc",
				Kind: KindTypst,
			},
			Content: fmt.Sprintf("= Version %d", i),
		})
		if err != nil {
			t.Fatalf("Save v%d failed: %v", i, err)
		}
		if saved.Version != i {
			t.Fatalf("Save v%d returned version %d", i, saved.Version)
		}
	}

	versions, err := reg.ListVersions("versioned-doc")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != MaxVersions {
		t.Fatalf("len(versions) = %d, want %d", len(versions), MaxVersions)
	}
	if versions[0].Version != 25 || versions[len(versions)-1].Version != 6 {
		t.Errorf("unexpected version range: newest=%d oldest=%d (want 25..6)",
			versions[0].Version, versions[len(versions)-1].Version)
	}

	// Pruned v1 must return ErrVersionNotFound
	if _, err := reg.GetVersion("versioned-doc", 1); !errors.Is(err, ErrVersionNotFound) {
		t.Errorf("GetVersion(1) = %v, want ErrVersionNotFound", err)
	}

	// Retained v10 must return its exact historical content
	v10, err := reg.GetVersion("versioned-doc", 10)
	if err != nil {
		t.Fatalf("GetVersion(10) failed: %v", err)
	}
	if v10.Content != "= Version 10" {
		t.Errorf("v10.Content = %q, want '= Version 10'", v10.Content)
	}

	// Restore v10 creates v26 with v10's content
	restored, err := reg.RestoreVersion("versioned-doc", 10)
	if err != nil {
		t.Fatalf("RestoreVersion(10) failed: %v", err)
	}
	if restored.Version != 26 || restored.Content != "= Version 10" {
		t.Errorf("unexpected restored record: v=%d content=%q", restored.Version, restored.Content)
	}
}

func TestRegistry_LegacyInteropAndPrecedence(t *testing.T) {
	baseDir := t.TempDir()

	// Create legacy root Factur-X template
	if err := os.WriteFile(filepath.Join(baseDir, "EN16931.typ"), []byte("= Legacy Invoice {{invoice.number}}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create legacy custom template + schema
	customDir := filepath.Join(baseDir, "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "gazette.typ"), []byte("= Legacy Gazette {{header.titre_gazette}}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "gazette.schema.json"), []byte(`{"type":"object"}`), 0644); err != nil {
		t.Fatal(err)
	}

	reg := New(baseDir)

	// Get legacy Factur-X by ID with and without .typ suffix
	fx1, err := reg.Get("EN16931")
	if err != nil {
		t.Fatalf("Get(EN16931) failed: %v", err)
	}
	if fx1.Kind != KindFacturX || !strings.Contains(fx1.Content, "Legacy Invoice") {
		t.Errorf("unexpected legacy facturx record: %+v", fx1)
	}
	fx2, err := reg.Get("EN16931.typ")
	if err != nil {
		t.Fatalf("Get(EN16931.typ) failed: %v", err)
	}
	if fx2.ID != "EN16931" || fx2.Content != fx1.Content {
		t.Errorf("Get(EN16931.typ) mismatch: %+v", fx2)
	}

	// Get legacy custom template with schema
	cust, err := reg.Get("gazette")
	if err != nil {
		t.Fatalf("Get(gazette) failed: %v", err)
	}
	if cust.Kind != KindCustom || cust.Schema != `{"type":"object"}` {
		t.Errorf("unexpected legacy custom record: %+v", cust)
	}

	// List all and list filtered
	all, err := reg.List("")
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("List() len = %d, want 2", len(all))
	}

	onlyCustom, err := reg.List("custom")
	if err != nil {
		t.Fatalf("List(custom) failed: %v", err)
	}
	if len(onlyCustom) != 1 || onlyCustom[0].ID != "gazette" {
		t.Errorf("List(custom) = %+v", onlyCustom)
	}

	// Saving a v2 bundle with same ID takes precedence over legacy without duplicating in List
	if _, err := reg.Save(&TemplateRecord{
		TemplateMeta: TemplateMeta{
			ID:   "gazette",
			Name: "Gazette v2",
			Kind: KindTypst,
		},
		Content: "= Upgraded Gazette v2",
	}); err != nil {
		t.Fatalf("Save v2 override failed: %v", err)
	}

	upgraded, err := reg.Get("gazette")
	if err != nil {
		t.Fatalf("Get upgraded gazette failed: %v", err)
	}
	if upgraded.Kind != KindTypst || upgraded.Content != "= Upgraded Gazette v2" {
		t.Errorf("expected v2 precedence, got %+v", upgraded)
	}

	allAfter, err := reg.List("")
	if err != nil {
		t.Fatalf("List() after upgrade failed: %v", err)
	}
	if len(allAfter) != 2 {
		t.Errorf("List() len = %d, want 2 (no duplicate for overridden id)", len(allAfter))
	}
}

func TestRegistry_ConcurrentAccessRaceFree(t *testing.T) {
	reg := New(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("tpl-%d", idx%3)
			for j := 0; j < 5; j++ {
				_, _ = reg.Save(&TemplateRecord{
					TemplateMeta: TemplateMeta{ID: id, Kind: KindTypst},
					Content:      fmt.Sprintf("= Doc %d-%d", idx, j),
				})
				_, _ = reg.Get(id)
				_, _ = reg.List("")
				_, _ = reg.ListVersions(id)
			}
		}(i)
	}
	wg.Wait()
}
