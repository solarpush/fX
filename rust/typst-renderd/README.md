# `typst-renderd`

Persistent, high-performance, hermetic Typst rendering daemon for **fX** (Factur-X PDF/A-3b, plain PDF, multi-page SVG, and PNG rendering).

`typst-renderd` embeds the Typst **0.15.1** compiler directly as a long-lived Rust service so fX never pays per-request process-spawn or font-discovery overhead.

## Architecture

| Module | Responsibility |
| --- | --- |
| `src/fonts.rs` | `FontCache`: scans `--font-path`, host system fonts, and Typst embedded fallback fonts **once at startup**, storing `LazyHash<Library>`, `LazyHash<FontBook>`, and lazily-cached font faces behind `Arc<FontCache>`. |
| `src/world.rs` | `VirtualWorld`: strictly hermetic, in-memory `typst::World` implementation. Normalizes virtual paths (stripping `..` traversals and backslashes), decodes Base64/data-URI binary assets, resolves 1-based line/column diagnostics, and **never reads the host filesystem** during compilation. |
| `src/compiler.rs` | `CompilerService`: executes `typst::compile::<PagedDocument>` and PDF / PDF/A-3b / SVG / PNG exporters on Tokio's `spawn_blocking` pool bounded by a `Semaphore` (`--workers`), periodically calling `comemo::evict(10)` to keep daemon RSS bounded. |
| `src/protocol.rs` | JSON wire types (`CompileRequest`, `CompileResponse`, `DiagnosticItem`, `HealthResponse`), format/PPI validation, and Base64 data-URI decoding. |
| `src/server.rs` | Axum 0.8 router (`GET /health`, `POST /compile`), `DefaultBodyLimit`, atomic render metrics, simultaneous Unix domain socket + TCP serving, stale `.sock` cleanup, and graceful SIGINT/SIGTERM shutdown. |
| `src/main.rs` | `clap` CLI/env parsing, `tracing` initialization, boot-time font indexing, and daemon lifecycle. |

## Prerequisites & Build

Typst 0.15 requires **Rust 1.92+** (with network access to `crates.io` on the first build to fetch Typst crates).

```bash
cd rust/typst-renderd

# Build optimized release binary
cargo build --release

# Run unit tests
cargo test
```

The release binary is emitted at `target/release/typst-renderd`.

## Configuration (CLI Flags & Environment Variables)

| Flag | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `--socket <path>` | `TYPST_DAEMON_SOCKET` | *(none)* | Unix domain socket path (e.g. `/tmp/typst-renderd.sock`) |
| `--listen <addr>` | `TYPST_DAEMON_LISTEN` | `127.0.0.1:9090` *(only if neither `--socket` nor `--listen` is set)* | TCP listen address (e.g. `127.0.0.1:9090` or `0.0.0.0:9090`) |
| `--font-path <dir>` | `TYPST_FONT_PATHS` | *(none)* | Extra font directory to scan (repeatable flag; colon-separated env var) |
| `--ignore-system-fonts` | `TYPST_IGNORE_SYSTEM_FONTS` | `false` | Skip host system font discovery (use only `--font-path` + Typst embedded fonts) |
| `--workers <n>` | `TYPST_DAEMON_WORKERS` | CPU count | Max concurrent Typst compilation workers |
| `--max-body-mb <n>` | `TYPST_DAEMON_MAX_BODY_MB` | `32` | Max HTTP JSON body size in MiB |
| — | `RUST_LOG` | `typst_renderd=info` | Tracing filter (e.g. `typst_renderd=debug`) |

You can bind a Unix socket alone, TCP alone, or **both simultaneously**:

```bash
./target/release/typst-renderd \
  --socket /tmp/typst-renderd.sock \
  --listen 127.0.0.1:9090 \
  --font-path /app/fonts \
  --workers 4 \
  --max-body-mb 32
```

On startup, stale socket files at `--socket` are cleaned up safely; on `SIGINT` / `SIGTERM`, in-flight renders drain gracefully and the `.sock` file is removed.

## HTTP / Unix Socket API

### `GET /health`

Returns HTTP `200 OK`:

```json
{
  "status": "ok",
  "version": "0.1.0",
  "typst_version": "0.15.1",
  "fonts_loaded": 42,
  "renders_total": 123,
  "avg_ms": 14.2
}
```

### `POST /compile`

Request JSON:

```json
{
  "source": "#set page(paper: \"a4\")\n= Facture\n#image(\"/logo.png\", width: 3cm)",
  "main_path": "/main.typ",
  "virtual_files": {
    "/data.json": "{\"invoice_number\": \"FA-2026-001\"}",
    "/factur-x.xml": "<rsm:CrossIndustryInvoice/>"
  },
  "binary_files": {
    "/logo.png": "data:image/png;base64,iVBORw0KGgo..."
  },
  "format": "pdf-a3b",
  "ppi": 144.0
}
```

Supported `format` values:
- `"pdf"` — standard PDF (`application/pdf`, returned in `data_base64`)
- `"pdf-a3b"` — archival PDF/A-3b (`application/pdf`, returned in `data_base64`, used for Factur-X)
- `"svg"` — per-page SVG strings (`image/svg+xml`, returned in `pages_svg`)
- `"png"` — per-page Base64 PNGs rendered at `ppi` (`image/png`, returned in `pages_base64`)

HTTP status codes:
- `200 OK`: compilation and export succeeded (`success: true`)
- `400 Bad Request`: malformed JSON, unsupported format, invalid PPI, invalid virtual path, or invalid Base64 payload (`success: false`)
- `413 Payload Too Large`: request body exceeds `--max-body-mb`
- `422 Unprocessable Entity`: Typst compilation or PDF/PNG export error with structured `errors` and `warnings` (`file`, 1-based `line`, 1-based `column`, `hints`)

### Example Requests

Over Unix domain socket:

```bash
curl --unix-socket /tmp/typst-renderd.sock http://localhost/health

curl --unix-socket /tmp/typst-renderd.sock \
  -H 'Content-Type: application/json' \
  -d '{"source":"= Hello Factur-X","format":"pdf-a3b"}' \
  http://localhost/compile
```

Over TCP:

```bash
curl http://127.0.0.1:9090/health

curl -X POST http://127.0.0.1:9090/compile \
  -H 'Content-Type: application/json' \
  -d '{"source":"= Preview","format":"svg"}'
```

## Docker Usage Notes

In a multi-stage container build (matching fX's Alpine runtime), build `typst-renderd` with a Rust 1.92+ builder stage and copy the binary into the final runtime image alongside the fonts (`ttf-liberation`, `ttf-dejavu`, `ttf-opensans`, `font-noto`, plus custom `/fonts`):

```dockerfile
FROM rust:1.92-alpine AS typst-daemon-builder
RUN apk add --no-cache musl-dev pkgconfig
WORKDIR /src/rust/typst-renderd
COPY rust/typst-renderd/Cargo.toml ./
COPY rust/typst-renderd/src ./src
RUN cargo build --release

# In the runtime stage:
# COPY --from=typst-daemon-builder /src/rust/typst-renderd/target/release/typst-renderd /usr/local/bin/typst-renderd
```

Recommended production configuration when co-located with `fx-server`:
- Bind a shared Unix socket (e.g. `TYPST_DAEMON_SOCKET=/tmp/typst-renderd.sock`) for minimal IPC latency and zero exposed ports.
- Set `TYPST_FONT_PATHS=/fonts` so custom brand fonts are scanned once at boot.
- Optionally also bind `TYPST_DAEMON_LISTEN=127.0.0.1:9090` if your orchestrator prefers HTTP health probes on `GET /health`.
