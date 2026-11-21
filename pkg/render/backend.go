package render

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const defaultHealthCacheTTL = 5 * time.Second

// CompilerBackend abstracts Typst compilation engines (CLI worker pool or
// persistent Rust typst-renderd daemon).
type CompilerBackend interface {
	Name() string
	Compile(ctx context.Context, job *CompileJob) (*CompileOutput, error)
	Health(ctx context.Context) error
	Stats() BackendStats
	Close() error
}

// SmartBackend multiplexes an optional primary backend (typically RustDaemonBackend)
// and an optional fallback backend (typically CLIBackend).
//
// Modes:
//   - "auto" (default): probes primary health (cached for healthTTL); if healthy
//     compiles via primary and gracefully falls back to fallback on transport errors.
//     User Typst CompileErrors are returned directly without fallback.
//   - "rust": strictly uses primary without fallback.
//   - "cli": strictly uses fallback.
type SmartBackend struct {
	mode      string
	primary   CompilerBackend
	fallback  CompilerBackend
	healthTTL time.Duration

	mu              sync.RWMutex
	primaryHealthy  bool
	lastHealthCheck time.Time
}

// NewSmartBackend constructs a SmartBackend with the given mode ("auto", "rust", or "cli").
func NewSmartBackend(mode string, primary, fallback CompilerBackend) *SmartBackend {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = "auto"
	}
	return &SmartBackend{
		mode:      m,
		primary:   primary,
		fallback:  fallback,
		healthTTL: defaultHealthCacheTTL,
	}
}

// SetHealthTTL overrides the cached primary health probe TTL (primarily for tests).
func (s *SmartBackend) SetHealthTTL(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d > 0 {
		s.healthTTL = d
	}
}

// Name reports the logical backend name.
func (s *SmartBackend) Name() string {
	switch s.mode {
	case "rust":
		if s.primary != nil {
			return s.primary.Name()
		}
		return "rust"
	case "cli":
		if s.fallback != nil {
			return s.fallback.Name()
		}
		return "cli"
	default:
		return "smart"
	}
}

// Compile executes job using the configured routing and fallback policy.
func (s *SmartBackend) Compile(ctx context.Context, job *CompileJob) (*CompileOutput, error) {
	switch s.mode {
	case "cli":
		if s.fallback == nil {
			return nil, errors.New("cli backend is not configured")
		}
		return s.fallback.Compile(ctx, job)

	case "rust":
		if s.primary == nil {
			return nil, errors.New("rust daemon backend is not configured")
		}
		return s.primary.Compile(ctx, job)

	default: // "auto"
		if s.primary != nil && s.isPrimaryHealthy(ctx) {
			out, err := s.primary.Compile(ctx, job)
			if err == nil {
				return out, nil
			}
			// Never mask user Typst syntax/compilation errors or caller cancellation.
			var compileErr *CompileError
			if errors.As(err, &compileErr) || ctx.Err() != nil {
				return nil, err
			}
			// Mark primary unhealthy immediately so subsequent requests bypass it until TTL expires.
			s.setPrimaryHealth(false)
			if s.fallback != nil {
				return s.fallback.Compile(ctx, job)
			}
			return nil, err
		}

		if s.fallback != nil {
			return s.fallback.Compile(ctx, job)
		}
		if s.primary != nil {
			return s.primary.Compile(ctx, job)
		}
		return nil, errors.New("no compiler backend configured")
	}
}

func (s *SmartBackend) isPrimaryHealthy(ctx context.Context) bool {
	s.mu.RLock()
	ttl := s.healthTTL
	last := s.lastHealthCheck
	healthy := s.primaryHealthy
	s.mu.RUnlock()

	if !last.IsZero() && time.Since(last) < ttl {
		return healthy
	}

	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ok := s.primary.Health(probeCtx) == nil
	s.setPrimaryHealth(ok)
	return ok
}

func (s *SmartBackend) setPrimaryHealth(healthy bool) {
	s.mu.Lock()
	s.primaryHealthy = healthy
	s.lastHealthCheck = time.Now()
	s.mu.Unlock()
}

// Health verifies that at least one usable backend under the active mode is healthy.
func (s *SmartBackend) Health(ctx context.Context) error {
	switch s.mode {
	case "cli":
		if s.fallback == nil {
			return errors.New("cli backend is not configured")
		}
		return s.fallback.Health(ctx)

	case "rust":
		if s.primary == nil {
			return errors.New("rust daemon backend is not configured")
		}
		err := s.primary.Health(ctx)
		s.setPrimaryHealth(err == nil)
		return err

	default: // "auto"
		var primaryErr, fallbackErr error
		if s.primary != nil {
			primaryErr = s.primary.Health(ctx)
			s.setPrimaryHealth(primaryErr == nil)
			if primaryErr == nil {
				return nil
			}
		}
		if s.fallback != nil {
			fallbackErr = s.fallback.Health(ctx)
			if fallbackErr == nil {
				return nil
			}
		}
		return fmt.Errorf("no healthy compiler backend (primary: %v, fallback: %v)", primaryErr, fallbackErr)
	}
}

// Stats returns aggregated runtime statistics for the active and underlying backends.
func (s *SmartBackend) Stats() BackendStats {
	s.mu.RLock()
	primaryHealthy := s.primaryHealthy
	s.mu.RUnlock()

	extra := map[string]interface{}{
		"mode": s.mode,
	}

	var active BackendStats
	hasActive := false

	if s.primary != nil {
		ps := s.primary.Stats()
		extra["primary"] = ps
		if s.mode == "rust" || (s.mode == "auto" && (primaryHealthy || s.fallback == nil)) {
			active = ps
			hasActive = true
		}
	}
	if s.fallback != nil {
		fs := s.fallback.Stats()
		extra["fallback"] = fs
		if s.mode == "cli" || (s.mode == "auto" && !hasActive) {
			active = fs
			hasActive = true
		}
	}

	if !hasActive {
		return BackendStats{
			Name:    s.Name(),
			Healthy: false,
			Extra:   extra,
		}
	}

	extra["activeBackend"] = active.Name

	// Aggregate totals across primary and fallback in auto mode for observability.
	stats := active
	if s.mode == "auto" && s.primary != nil && s.fallback != nil {
		ps := s.primary.Stats()
		fs := s.fallback.Stats()
		stats.TotalRenders = ps.TotalRenders + fs.TotalRenders
		stats.FailedRenders = ps.FailedRenders + fs.FailedRenders
		stats.ActiveJobs = ps.ActiveJobs + fs.ActiveJobs
		stats.Healthy = ps.Healthy || fs.Healthy
	}
	stats.Extra = extra
	return stats
}

// Close releases resources for both primary and fallback backends.
func (s *SmartBackend) Close() error {
	var errs []error
	if s.primary != nil {
		if err := s.primary.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if s.fallback != nil {
		if err := s.fallback.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
