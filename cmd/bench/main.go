package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/solarpush/fx/pkg/blueprint"
	"github.com/solarpush/fx/pkg/cii"
	"github.com/solarpush/fx/pkg/invoice"
	"github.com/solarpush/fx/pkg/render"
	"github.com/solarpush/fx/pkg/render/modules"
	"github.com/solarpush/fx/pkg/schema"
)

const defaultDaemonSocket = "/tmp/typst-renderd.sock"

var sampleInvoiceJSON = []byte(`{
  "version": "1.0",
  "profile": "EN16931",
  "invoice": {
    "number": "INV-12345",
    "issue_date": "2026-06-27T00:00:00Z",
    "due_date": "2026-07-27T00:00:00Z",
    "currency": "EUR",
    "type": "380",
    "buyer_reference": "DEPT-FIN-2026"
  },
  "seller": {
    "name": "Tech Corp France",
    "vat_id": "FR12345678901",
    "global_id": {
      "scheme_id": "0009",
      "value": "12345678900012"
    },
    "address": {
      "street": "123 Avenue de la République",
      "city": "Paris",
      "postal_code": "75011",
      "country": "FR"
    }
  },
  "buyer": {
    "name": "Acme Solutions",
    "vat_id": "FR98765432109",
    "global_id": {
      "scheme_id": "0009",
      "value": "98765432100021"
    },
    "address": {
      "street": "45 Boulevard Haussmann",
      "city": "Paris",
      "postal_code": "75009",
      "country": "FR"
    }
  },
  "lines": [
    {
      "id": "1",
      "description": "Consulting Services",
      "quantity": 10,
      "unit": "DAY",
      "unit_price": 100,
      "vat_rate": 20,
      "total_excl_vat": 1000,
      "total_incl_vat": 1200,
      "vat_amount": 200
    }
  ],
  "totals": {
    "subtotal_excl_vat": 1000,
    "total_vat": 200,
    "total_incl_vat": 1200,
    "amount_due": 1200,
    "tax_basis_total": 1000,
    "vat_breakdown": [
      {
        "rate": 20,
        "taxable_amount": 1000,
        "vat_amount": 200
      }
    ]
  },
  "payment": {
    "terms": "30 jours fin de mois",
    "iban": "FR7630004008010001012345678",
    "payment_means": {
      "type_code": "58",
      "information": "Virement SEPA"
    }
  }
}`)

type benchConfig struct {
	iterations   int
	concurrency  int
	mode         string
	daemonSocket string
	daemonURL    string
	format       render.OutputFormat
}

type benchStats struct {
	count      int
	successes  int
	failures   int
	wall       time.Duration
	totalBytes int64
	min        time.Duration
	avg        time.Duration
	p50        time.Duration
	p95        time.Duration
	p99        time.Duration
	max        time.Duration
	firstErr   error
}

func (s benchStats) docsPerSec() float64 {
	if s.wall <= 0 || s.successes == 0 {
		return 0
	}
	return float64(s.successes) / s.wall.Seconds()
}

func (s benchStats) avgBytes() float64 {
	if s.successes == 0 {
		return 0
	}
	return float64(s.totalBytes) / float64(s.successes)
}

func main() {
	iterations := flag.Int("iterations", 200, "Number of iterations per benchmark suite")
	concurrency := flag.Int("concurrency", runtime.NumCPU(), "Number of concurrent workers")
	mode := flag.String("mode", "all", "Benchmark mode: all|xml|blueprint|render|facturx")
	daemonSocket := flag.String("daemon-socket", os.Getenv("RENDER_DAEMON_SOCKET"), "Unix socket path for Rust typst-renderd daemon")
	daemonURL := flag.String("daemon-url", os.Getenv("RENDER_DAEMON_URL"), "HTTP base URL for Rust typst-renderd daemon (e.g. http://127.0.0.1:9090)")
	formatFlag := flag.String("format", "pdf", "Output format for render benchmark: pdf|svg|png")
	flag.Parse()

	if *iterations <= 0 {
		fmt.Fprintln(os.Stderr, "error: -iterations must be > 0")
		os.Exit(2)
	}
	if *concurrency <= 0 {
		fmt.Fprintln(os.Stderr, "error: -concurrency must be > 0")
		os.Exit(2)
	}

	normMode := strings.ToLower(strings.TrimSpace(*mode))
	switch normMode {
	case "all", "xml", "blueprint", "render", "facturx":
	default:
		fmt.Fprintf(os.Stderr, "error: invalid -mode %q (expected all|xml|blueprint|render|facturx)\n", *mode)
		os.Exit(2)
	}

	normFmt := render.NormalizeFormat(*formatFlag)
	switch normFmt {
	case render.FormatPDF, render.FormatSVG, render.FormatPNG:
	default:
		fmt.Fprintf(os.Stderr, "error: invalid -format %q (expected pdf|svg|png)\n", *formatFlag)
		os.Exit(2)
	}

	cfg := benchConfig{
		iterations:   *iterations,
		concurrency:  *concurrency,
		mode:         normMode,
		daemonSocket: strings.TrimSpace(*daemonSocket),
		daemonURL:    strings.TrimSpace(*daemonURL),
		format:       normFmt,
	}

	fmt.Println("================================================================")
	fmt.Println("  fX v2 Universal Document Engine — Performance Benchmark CLI")
	fmt.Println("================================================================")
	fmt.Printf("  Mode        : %s\n", cfg.mode)
	fmt.Printf("  Iterations  : %d\n", cfg.iterations)
	fmt.Printf("  Concurrency : %d workers (CPUs: %d)\n", cfg.concurrency, runtime.NumCPU())
	fmt.Printf("  Format      : %s\n", cfg.format)
	if cfg.daemonSocket != "" {
		fmt.Printf("  Daemon Sock : %s\n", cfg.daemonSocket)
	}
	if cfg.daemonURL != "" {
		fmt.Printf("  Daemon URL  : %s\n", cfg.daemonURL)
	}
	fmt.Println("================================================================")
	fmt.Println()

	hadError := false

	if cfg.mode == "all" || cfg.mode == "xml" {
		if err := benchXML(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] XML benchmark failed: %v\n", err)
			hadError = true
		}
	}

	if cfg.mode == "all" || cfg.mode == "blueprint" {
		if err := benchBlueprint(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Blueprint benchmark failed: %v\n", err)
			hadError = true
		}
	}

	if cfg.mode == "all" || cfg.mode == "render" || cfg.mode == "facturx" {
		ctx := context.Background()
		engine, backend, backendErr := buildRenderEngine(ctx, cfg)
		if backendErr != nil {
			if cfg.mode == "all" {
				fmt.Println("── [3/4 & 4/4] Render Engine & Factur-X Benchmarks ──")
				fmt.Printf("  [SKIP] No healthy Typst compiler backend available: %v\n", backendErr)
				fmt.Println("  Hint : Start typst-renderd (`make run-daemon`) or install `typst` CLI in PATH,")
				fmt.Println("         or pass -daemon-socket=/tmp/typst-renderd.sock / -daemon-url=http://127.0.0.1:9090.")
				fmt.Println()
			} else {
				fmt.Fprintf(os.Stderr, "[ERROR] No healthy Typst compiler backend available: %v\n", backendErr)
				os.Exit(1)
			}
		} else {
			defer backend.Close()

			if cfg.mode == "all" || cfg.mode == "render" {
				if err := benchRender(ctx, engine, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Render benchmark failed: %v\n", err)
					hadError = true
				}
			}

			if cfg.mode == "all" || cfg.mode == "facturx" {
				if err := benchFacturX(ctx, engine, cfg); err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Factur-X benchmark failed: %v\n", err)
					hadError = true
				}
			}
		}
	}

	if hadError {
		os.Exit(1)
	}
}

// -----------------------------------------------------------------------------
// 1. CII XML Generation Benchmark
// -----------------------------------------------------------------------------

func benchXML(cfg benchConfig) error {
	fmt.Println("── [1/4] CII XML Generation (Factur-X EN16931) ──")

	stats := runConcurrent(cfg.iterations, cfg.concurrency, func(i int) (int64, error) {
		inv, err := invoice.FromJSON(sampleInvoiceJSON)
		if err != nil {
			return 0, fmt.Errorf("parse invoice JSON: %w", err)
		}
		inv.Invoice.Number = fmt.Sprintf("INV-%06d", i+1)

		xmlBytes, err := cii.Generate(inv)
		if err != nil {
			return 0, fmt.Errorf("generate CII XML: %w", err)
		}
		return int64(len(xmlBytes)), nil
	})

	printStats("CII XML", stats, "bytes")
	fmt.Println()

	if stats.failures > 0 {
		return fmt.Errorf("%d/%d XML generations failed (first: %v)", stats.failures, stats.count, stats.firstErr)
	}
	return nil
}

// -----------------------------------------------------------------------------
// 2. Blueprint JSON -> Native Typst Compilation + JSON Schema Validation
// -----------------------------------------------------------------------------

func benchBlueprint(cfg benchConfig) error {
	fmt.Println("── [2/4] Blueprint -> Native Typst Compilation & Schema Validation ──")

	presets := blueprint.BuiltInPresets()
	if len(presets) == 0 {
		return fmt.Errorf("no built-in presets available")
	}

	var allLatencies []time.Duration
	var totalBytes int64
	totalSuccess := 0
	totalFail := 0
	overallStart := time.Now()

	for _, preset := range presets {
		p := preset

		// Pre-flight check to ensure preset schema, sample data, and blueprint are valid.
		schCheck, err := schema.Parse(p.Schema)
		if err != nil {
			return fmt.Errorf("preset %q invalid schema: %w", p.ID, err)
		}
		var sampleObj interface{}
		if err := json.Unmarshal(p.SampleData, &sampleObj); err != nil {
			return fmt.Errorf("preset %q invalid sample data: %w", p.ID, err)
		}
		if errs := schCheck.Validate(sampleObj); len(errs) > 0 {
			return fmt.Errorf("preset %q sample data failed schema validation: %s", p.ID, strings.Join(errs, "; "))
		}
		compiledPreview, err := blueprint.Compile(&p.Blueprint)
		if err != nil {
			return fmt.Errorf("preset %q compile failed: %w", p.ID, err)
		}

		latencies := make([]time.Duration, cfg.iterations)
		var presetBytes atomic.Int64
		var presetFail atomic.Int64
		var firstErr error
		var errMu sync.Mutex

		jobs := make(chan int, cfg.iterations)
		for i := 0; i < cfg.iterations; i++ {
			jobs <- i
		}
		close(jobs)

		workers := cfg.concurrency
		if workers > cfg.iterations {
			workers = cfg.iterations
		}

		presetStart := time.Now()
		var wg sync.WaitGroup
		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for idx := range jobs {
					t0 := time.Now()
					sch, err := schema.Parse(p.Schema)
					if err == nil {
						var parsed interface{}
						if uErr := json.Unmarshal(p.SampleData, &parsed); uErr != nil {
							err = uErr
						} else if vErrs := sch.Validate(parsed); len(vErrs) > 0 {
							err = fmt.Errorf("schema validation: %s", strings.Join(vErrs, "; "))
						}
					}
					if err == nil {
						bpCopy := p.Blueprint
						var typstSrc string
						typstSrc, err = blueprint.Compile(&bpCopy)
						if err == nil {
							presetBytes.Add(int64(len(typstSrc)))
						}
					}
					latencies[idx] = time.Since(t0)
					if err != nil {
						presetFail.Add(1)
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
					}
				}
			}()
		}
		wg.Wait()
		presetWall := time.Since(presetStart)

		pStats := computeStats(latencies, presetWall, presetBytes.Load(), int(presetFail.Load()), firstErr)
		fmt.Printf("  %-26s | typst=%5d B | %5d/%-5d ok | %9.1f docs/s | p50=%-8s p95=%-8s p99=%-8s\n",
			p.ID,
			len(compiledPreview),
			pStats.successes,
			pStats.count,
			pStats.docsPerSec(),
			formatDuration(pStats.p50),
			formatDuration(pStats.p95),
			formatDuration(pStats.p99),
		)

		allLatencies = append(allLatencies, latencies...)
		totalBytes += presetBytes.Load()
		totalSuccess += pStats.successes
		totalFail += pStats.failures

		if firstErr != nil {
			return fmt.Errorf("preset %q benchmark failed: %w", p.ID, firstErr)
		}
	}

	overallWall := time.Since(overallStart)
	agg := computeStats(allLatencies, overallWall, totalBytes, totalFail, nil)
	agg.successes = totalSuccess
	fmt.Println("  -----------------------------------------------------------------------------------------------")
	printStats("All 4 Presets Combined", agg, "bytes")
	fmt.Println()
	return nil
}

// -----------------------------------------------------------------------------
// 3. v2 Render Engine Concurrent Throughput Benchmark
// -----------------------------------------------------------------------------

func benchRender(ctx context.Context, engine *render.Engine, cfg benchConfig) error {
	fmt.Printf("── [3/4] v2 Render Engine Concurrent Throughput (%s) ──\n", strings.ToUpper(string(cfg.format)))

	preset, ok := blueprint.GetPreset("editorial-gazette")
	if !ok {
		return fmt.Errorf("built-in preset editorial-gazette not found")
	}

	var basePayload map[string]interface{}
	if err := json.Unmarshal(preset.SampleData, &basePayload); err != nil {
		return fmt.Errorf("unmarshal editorial-gazette sample data: %w", err)
	}

	// Warmup render to initialize caches and discover the active backend.
	warmBP := preset.Blueprint
	warmRes, err := engine.Render(ctx, &render.RenderRequest{
		Blueprint:  &warmBP,
		Data:       preset.SampleData,
		Format:     cfg.format,
		EngineMode: render.ModeBlueprint,
	})
	if err != nil {
		return fmt.Errorf("warmup render failed: %w", err)
	}
	fmt.Printf("  Active backend : %s (warmup: %d ms, %d page(s), %d bytes)\n",
		warmRes.Backend, warmRes.DurationMs, warmRes.PageCount, resultByteSize(warmRes))

	// Pre-build per-iteration JSON payloads so JSON marshaling doesn't skew render latency.
	payloads := make([]json.RawMessage, cfg.iterations)
	for i := 0; i < cfg.iterations; i++ {
		hdrCopy := make(map[string]interface{})
		if h, ok := basePayload["header"].(map[string]interface{}); ok {
			for k, v := range h {
				hdrCopy[k] = v
			}
		}
		hdrCopy["date_publication"] = fmt.Sprintf("BENCHMARK #%05d — 2026", i+1)

		itemMap := map[string]interface{}{
			"header":   hdrCopy,
			"articles": basePayload["articles"],
		}
		raw, err := json.Marshal(itemMap)
		if err != nil {
			return fmt.Errorf("marshal item %d: %w", i, err)
		}
		payloads[i] = raw
	}

	stats := runConcurrent(cfg.iterations, cfg.concurrency, func(i int) (int64, error) {
		bpCopy := preset.Blueprint
		res, err := engine.Render(ctx, &render.RenderRequest{
			Blueprint:  &bpCopy,
			Data:       payloads[i],
			Format:     cfg.format,
			EngineMode: render.ModeBlueprint,
		})
		if err != nil {
			return 0, err
		}
		return int64(resultByteSize(res)), nil
	})

	printStats(fmt.Sprintf("Render Engine (%s / %s)", warmRes.Backend, cfg.format), stats, "KB")
	fmt.Println()

	if stats.failures > 0 {
		return fmt.Errorf("%d/%d renders failed (first: %v)", stats.failures, stats.count, stats.firstErr)
	}
	return nil
}

// -----------------------------------------------------------------------------
// 4. Full Factur-X Pipeline Benchmark via FacturXModule
// -----------------------------------------------------------------------------

func benchFacturX(ctx context.Context, engine *render.Engine, cfg benchConfig) error {
	fmt.Println("── [4/4] Full Factur-X EN16931 Pipeline (Blueprint + CII XML + PDF/A-3b + FacturXModule) ──")

	preset, ok := blueprint.GetPreset("facturx-en16931")
	if !ok {
		return fmt.Errorf("built-in preset facturx-en16931 not found")
	}

	// Warmup full Factur-X pipeline (EN16931 validation + CII XML + PDF/A-3b + XMP/EmbeddedFiles patch).
	warmBP := preset.Blueprint
	warmRes, err := engine.Render(ctx, &render.RenderRequest{
		Blueprint:  &warmBP,
		Data:       preset.SampleData,
		Format:     render.FormatPDFA3B,
		EngineMode: render.ModeBlueprint,
		Modules:    []render.ModuleConfig{{Name: "facturx-en16931"}},
	})
	if err != nil {
		return fmt.Errorf("warmup Factur-X render failed: %w", err)
	}
	fmt.Printf("  Active backend : %s (warmup: %d ms, %d page(s), %.1f KB PDF/A-3b)\n",
		warmRes.Backend, warmRes.DurationMs, warmRes.PageCount, float64(len(warmRes.Data))/1024.0)

	// Pre-build unique invoice JSONs per iteration.
	payloads := make([]json.RawMessage, cfg.iterations)
	for i := 0; i < cfg.iterations; i++ {
		inv, err := invoice.FromJSON(preset.SampleData)
		if err != nil {
			return fmt.Errorf("parse preset invoice JSON: %w", err)
		}
		inv.Invoice.Number = fmt.Sprintf("FAC-BENCH-%06d", i+1)
		raw, err := invoice.ToJSON(inv)
		if err != nil {
			return fmt.Errorf("marshal invoice %d: %w", i, err)
		}
		payloads[i] = raw
	}

	stats := runConcurrent(cfg.iterations, cfg.concurrency, func(i int) (int64, error) {
		bpCopy := preset.Blueprint
		res, err := engine.Render(ctx, &render.RenderRequest{
			Blueprint:  &bpCopy,
			Data:       payloads[i],
			Format:     render.FormatPDFA3B,
			EngineMode: render.ModeBlueprint,
			Modules:    []render.ModuleConfig{{Name: "facturx-en16931"}},
		})
		if err != nil {
			return 0, err
		}
		return int64(len(res.Data)), nil
	})

	printStats(fmt.Sprintf("Factur-X EN16931 (%s)", warmRes.Backend), stats, "KB")
	fmt.Println()

	if stats.failures > 0 {
		return fmt.Errorf("%d/%d Factur-X renders failed (first: %v)", stats.failures, stats.count, stats.firstErr)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Backend Wiring & Metrics Utilities
// -----------------------------------------------------------------------------

func buildRenderEngine(ctx context.Context, cfg benchConfig) (*render.Engine, *render.SmartBackend, error) {
	var primary render.CompilerBackend
	socketPath := cfg.daemonSocket
	daemonURL := cfg.daemonURL

	// Auto-detect local typst-renderd Unix socket when no explicit daemon address is passed.
	if socketPath == "" && daemonURL == "" {
		if info, err := os.Stat(defaultDaemonSocket); err == nil && (info.Mode()&os.ModeSocket != 0) {
			socketPath = defaultDaemonSocket
		}
	}

	if socketPath != "" || daemonURL != "" {
		rustBackend, err := render.NewRustDaemonBackend(render.RustDaemonConfig{
			SocketPath: socketPath,
			BaseURL:    daemonURL,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("init rust daemon backend: %w", err)
		}
		primary = rustBackend
	}

	var fallback render.CompilerBackend
	cliBackend, cliErr := render.NewCLIBackend(render.CLIConfig{
		Workers: cfg.concurrency,
	})
	if cliErr == nil {
		fallback = cliBackend
	}

	if primary == nil && fallback == nil {
		return nil, nil, fmt.Errorf("neither rust daemon nor typst CLI found (%v)", cliErr)
	}

	smart := render.NewSmartBackend("auto", primary, fallback)

	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := smart.Health(probeCtx); err != nil {
		_ = smart.Close()
		return nil, nil, err
	}

	modReg := modules.NewDefaultRegistry()
	engine := render.NewEngine(smart, modReg, nil)
	return engine, smart, nil
}

func runConcurrent(iterations, concurrency int, fn func(i int) (int64, error)) benchStats {
	latencies := make([]time.Duration, iterations)
	var totalBytes atomic.Int64
	var failCount atomic.Int64
	var firstErr error
	var errMu sync.Mutex

	jobs := make(chan int, iterations)
	for i := 0; i < iterations; i++ {
		jobs <- i
	}
	close(jobs)

	workers := concurrency
	if workers > iterations {
		workers = iterations
	}

	start := time.Now()
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for idx := range jobs {
				t0 := time.Now()
				n, err := fn(idx)
				latencies[idx] = time.Since(t0)
				if err != nil {
					failCount.Add(1)
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
					continue
				}
				totalBytes.Add(n)
			}
		}()
	}
	wg.Wait()
	wall := time.Since(start)

	return computeStats(latencies, wall, totalBytes.Load(), int(failCount.Load()), firstErr)
}

func computeStats(latencies []time.Duration, wall time.Duration, totalBytes int64, failures int, firstErr error) benchStats {
	n := len(latencies)
	stats := benchStats{
		count:      n,
		successes:  n - failures,
		failures:   failures,
		wall:       wall,
		totalBytes: totalBytes,
		firstErr:   firstErr,
	}
	if n == 0 {
		return stats
	}

	sorted := append([]time.Duration(nil), latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}

	stats.min = sorted[0]
	stats.max = sorted[n-1]
	stats.avg = sum / time.Duration(n)
	stats.p50 = percentile(sorted, 0.50)
	stats.p95 = percentile(sorted, 0.95)
	stats.p99 = percentile(sorted, 0.99)
	return stats
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}

func resultByteSize(res *render.RenderResult) int {
	if res == nil {
		return 0
	}
	if len(res.Data) > 0 {
		return len(res.Data)
	}
	total := 0
	for _, p := range res.Pages {
		total += len(p)
	}
	if total == 0 {
		for _, s := range res.PagesSVG {
			total += len(s)
		}
	}
	return total
}

func printStats(label string, s benchStats, sizeUnit string) {
	sizeStr := fmt.Sprintf("%.0f B", s.avgBytes())
	if sizeUnit == "KB" {
		sizeStr = fmt.Sprintf("%.1f KB", s.avgBytes()/1024.0)
	}
	fmt.Printf("  %-26s | %d/%d ok | wall=%s | throughput=%.2f docs/sec | avg size=%s\n",
		label, s.successes, s.count, s.wall.Round(time.Millisecond), s.docsPerSec(), sizeStr)
	fmt.Printf("  %-26s | min=%s  avg=%s  p50=%s  p95=%s  p99=%s  max=%s\n",
		"Latency",
		formatDuration(s.min),
		formatDuration(s.avg),
		formatDuration(s.p50),
		formatDuration(s.p95),
		formatDuration(s.p99),
		formatDuration(s.max),
	)
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%.1fµs", float64(d.Nanoseconds())/1e3)
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}
