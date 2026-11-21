#![forbid(unsafe_code)]

mod compiler;
mod fonts;
mod protocol;
mod server;
mod world;

use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Instant;

use clap::Parser;
use tracing::{error, info};
use tracing_subscriber::EnvFilter;

use crate::compiler::CompilerService;
use crate::fonts::FontCache;
use crate::protocol::{DAEMON_VERSION, TYPST_VERSION};
use crate::server::{build_router, serve, AppState, BindConfig};

const DEFAULT_TCP_PORT: u16 = 9090;
const DEFAULT_MAX_BODY_MB: usize = 32;

/// Persistent, high-performance Typst compilation daemon for fX.
#[derive(Debug, Parser)]
#[command(
    name = "typst-renderd",
    version = DAEMON_VERSION,
    about = "Persistent in-memory Typst rendering daemon (PDF, PDF/A-3b, SVG, PNG)"
)]
struct Cli {
    /// Unix domain socket path to bind (e.g. /tmp/typst-renderd.sock).
    #[arg(long, env = "TYPST_DAEMON_SOCKET")]
    socket: Option<PathBuf>,

    /// TCP listen address to bind (e.g. 127.0.0.1:9090).
    #[arg(long, env = "TYPST_DAEMON_LISTEN")]
    listen: Option<SocketAddr>,

    /// Extra directory to scan for fonts (repeatable; colon-separated via TYPST_FONT_PATHS).
    #[arg(
        long = "font-path",
        env = "TYPST_FONT_PATHS",
        value_delimiter = ':'
    )]
    font_paths: Vec<PathBuf>,

    /// Do not scan host system fonts; use only --font-path and Typst embedded fonts.
    #[arg(long, env = "TYPST_IGNORE_SYSTEM_FONTS", default_value_t = false)]
    ignore_system_fonts: bool,

    /// Maximum concurrent Typst compilation workers (defaults to available CPU parallelism).
    #[arg(long, env = "TYPST_DAEMON_WORKERS")]
    workers: Option<usize>,

    /// Maximum HTTP request body size in megabytes.
    #[arg(
        long,
        env = "TYPST_DAEMON_MAX_BODY_MB",
        default_value_t = DEFAULT_MAX_BODY_MB
    )]
    max_body_mb: usize,
}

fn default_workers() -> usize {
    std::thread::available_parallelism()
        .map(usize::from)
        .unwrap_or(4)
        .max(1)
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| EnvFilter::new("typst_renderd=info")),
        )
        .with_target(false)
        .init();

    let cli = Cli::parse();

    let workers = cli.workers.unwrap_or_else(default_workers).max(1);
    let max_body_mb = cli.max_body_mb.max(1);
    let max_body_bytes = max_body_mb.saturating_mul(1024 * 1024);

    let font_paths: Vec<PathBuf> = cli
        .font_paths
        .into_iter()
        .filter(|p| !p.as_os_str().is_empty())
        .collect();

    let (socket, listen) = match (cli.socket, cli.listen) {
        (None, None) => {
            let fallback = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), DEFAULT_TCP_PORT);
            info!(
                default_listen = %fallback,
                "neither --socket nor --listen specified; defaulting to TCP listener"
            );
            (None, Some(fallback))
        }
        other => other,
    };

    info!(
        version = DAEMON_VERSION,
        typst_version = TYPST_VERSION,
        workers,
        max_body_mb,
        ignore_system_fonts = cli.ignore_system_fonts,
        extra_font_dirs = font_paths.len(),
        "booting typst-renderd"
    );

    let boot_start = Instant::now();
    let font_cache = Arc::new(FontCache::load(&font_paths, cli.ignore_system_fonts));
    info!(
        fonts_loaded = font_cache.font_count(),
        elapsed_ms = boot_start.elapsed().as_millis() as u64,
        "font cache initialized"
    );

    let compiler = Arc::new(CompilerService::new(font_cache, workers));
    let state = Arc::new(AppState::new(compiler));
    let router = build_router(state, max_body_bytes);

    let bind = BindConfig { socket, listen };
    if let Err(err) = serve(router, bind).await {
        error!(error = %err, "server exited with error");
        std::process::exit(1);
    }

    info!("typst-renderd shutdown complete");
}
