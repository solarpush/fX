use std::io;
use std::net::SocketAddr;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

use axum::extract::rejection::JsonRejection;
use axum::extract::{DefaultBodyLimit, State};
use axum::http::StatusCode;
use axum::routing::{get, post};
use axum::{Json, Router};
use tokio::net::TcpListener;
#[cfg(unix)]
use tokio::net::UnixListener;
use tokio::sync::watch;
use tracing::{error, info, warn};

use crate::compiler::{CompileOutcome, CompilerService};
use crate::protocol::{
    CompileRequest, CompileResponse, DiagnosticItem, HealthResponse, DAEMON_VERSION, TYPST_VERSION,
};

/// Lock-free atomic render metrics exposed by `GET /health`.
#[derive(Debug, Default)]
pub struct RenderMetrics {
    renders_total: AtomicU64,
    total_duration_ms: AtomicU64,
}

impl RenderMetrics {
    /// Records the completion time of a Typst compilation run.
    pub fn record(&self, duration_ms: u64) {
        self.renders_total.fetch_add(1, Ordering::Relaxed);
        self.total_duration_ms
            .fetch_add(duration_ms, Ordering::Relaxed);
    }

    /// Snapshots `(renders_total, avg_ms)` for `GET /health`.
    pub fn snapshot(&self) -> (u64, f64) {
        let total = self.renders_total.load(Ordering::Relaxed);
        if total == 0 {
            return (0, 0.0);
        }
        let total_ms = self.total_duration_ms.load(Ordering::Relaxed);
        let avg = (total_ms as f64) / (total as f64);
        // Round to 2 decimal places to keep health JSON tidy.
        let rounded = (avg * 100.0).round() / 100.0;
        (total, rounded)
    }
}

/// Shared daemon state injected into Axum handlers.
pub struct AppState {
    compiler: Arc<CompilerService>,
    metrics: RenderMetrics,
}

impl AppState {
    pub fn new(compiler: Arc<CompilerService>) -> Self {
        Self {
            compiler,
            metrics: RenderMetrics::default(),
        }
    }

    /// Records the completion time of a Typst compilation run.
    pub fn record_render(&self, duration_ms: u64) {
        self.metrics.record(duration_ms);
    }

    /// Snapshots `(renders_total, avg_ms)` for `GET /health`.
    pub fn metrics_snapshot(&self) -> (u64, f64) {
        self.metrics.snapshot()
    }
}

/// Listener binding configuration: Unix socket, TCP address, or both.
#[derive(Debug, Clone)]
pub struct BindConfig {
    pub socket: Option<PathBuf>,
    pub listen: Option<SocketAddr>,
}

/// Builds the Axum router with `GET /health`, `POST /compile`, and body limit.
pub fn build_router(state: Arc<AppState>, max_body_bytes: usize) -> Router {
    Router::new()
        .route("/health", get(health_handler))
        .route("/compile", post(compile_handler))
        .layer(DefaultBodyLimit::max(max_body_bytes))
        .with_state(state)
}

async fn health_handler(State(state): State<Arc<AppState>>) -> Json<HealthResponse> {
    let (renders_total, avg_ms) = state.metrics_snapshot();
    Json(HealthResponse {
        status: "ok",
        version: DAEMON_VERSION,
        typst_version: TYPST_VERSION,
        fonts_loaded: state.compiler.font_count(),
        renders_total,
        avg_ms,
    })
}

async fn compile_handler(
    State(state): State<Arc<AppState>>,
    payload: Result<Json<CompileRequest>, JsonRejection>,
) -> (StatusCode, Json<CompileResponse>) {
    let Json(request) = match payload {
        Ok(json) => json,
        Err(rejection) => {
            let status = if rejection.status() == StatusCode::PAYLOAD_TOO_LARGE {
                StatusCode::PAYLOAD_TOO_LARGE
            } else {
                StatusCode::BAD_REQUEST
            };
            let resp = CompileResponse::failure(
                "",
                "",
                0,
                Vec::new(),
                vec![DiagnosticItem::error(rejection.body_text())],
            );
            return (status, Json(resp));
        }
    };

    match state.compiler.compile(request).await {
        CompileOutcome::Success(resp) => {
            state.record_render(resp.duration_ms);
            (StatusCode::OK, Json(resp))
        }
        CompileOutcome::InvalidRequest(resp) => (StatusCode::BAD_REQUEST, Json(resp)),
        CompileOutcome::CompileError(resp) => {
            state.record_render(resp.duration_ms);
            (StatusCode::UNPROCESSABLE_ENTITY, Json(resp))
        }
    }
}

/// RAII guard that removes a bound Unix domain socket file when dropped.
#[cfg(unix)]
struct UnixSocketCleanup {
    path: PathBuf,
}

#[cfg(unix)]
impl Drop for UnixSocketCleanup {
    fn drop(&mut self) {
        if let Err(err) = std::fs::remove_file(&self.path) {
            if err.kind() != io::ErrorKind::NotFound {
                warn!(path = %self.path.display(), error = %err, "failed to remove unix socket file");
            }
        } else {
            info!(path = %self.path.display(), "removed unix socket file");
        }
    }
}

#[cfg(unix)]
fn bind_unix_listener(path: &Path) -> io::Result<(UnixListener, UnixSocketCleanup)> {
    if let Some(parent) = path.parent() {
        if !parent.as_os_str().is_empty() {
            std::fs::create_dir_all(parent)?;
        }
    }

    match std::fs::symlink_metadata(path) {
        Ok(meta) => {
            if meta.is_dir() {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidInput,
                    format!("socket path '{}' is a directory", path.display()),
                ));
            }
            warn!(path = %path.display(), "removing stale unix socket file before bind");
            std::fs::remove_file(path)?;
        }
        Err(err) if err.kind() == io::ErrorKind::NotFound => {}
        Err(err) => return Err(err),
    }

    let listener = UnixListener::bind(path)?;
    Ok((
        listener,
        UnixSocketCleanup {
            path: path.to_path_buf(),
        },
    ))
}

/// Serves the Axum router on Unix socket, TCP socket, or both simultaneously,
/// shutting down gracefully on SIGINT / SIGTERM and cleaning up the `.sock` file.
pub async fn serve(router: Router, bind: BindConfig) -> io::Result<()> {
    let (shutdown_tx, shutdown_rx) = watch::channel(false);

    let signal_tx = shutdown_tx.clone();
    tokio::spawn(async move {
        shutdown_signal().await;
        let _ = signal_tx.send(true);
    });

    match (bind.socket, bind.listen) {
        (Some(socket_path), Some(tcp_addr)) => {
            #[cfg(unix)]
            {
                let (unix_listener, _socket_guard) = bind_unix_listener(&socket_path)?;
                let tcp_listener = TcpListener::bind(tcp_addr).await?;

                let local_tcp = tcp_listener.local_addr().unwrap_or(tcp_addr);
                info!(
                    unix_socket = %socket_path.display(),
                    tcp_addr = %local_tcp,
                    "typst-renderd listening on Unix socket and TCP"
                );

                let unix_app = router.clone();
                let unix_shutdown = shutdown_rx.clone();
                let mut unix_handle = tokio::spawn(async move {
                    axum::serve(unix_listener, unix_app)
                        .with_graceful_shutdown(wait_for_shutdown(unix_shutdown))
                        .await
                });

                let tcp_shutdown = shutdown_rx;
                let mut tcp_handle = tokio::spawn(async move {
                    axum::serve(tcp_listener, router)
                        .with_graceful_shutdown(wait_for_shutdown(tcp_shutdown))
                        .await
                });

                let first_err: Option<io::Error> = tokio::select! {
                    res = &mut unix_handle => {
                        let _ = shutdown_tx.send(true);
                        let err = flatten_join_result("unix", res);
                        if let Some(other_err) = flatten_join_result("tcp", tcp_handle.await) {
                            error!(error = %other_err, "tcp listener exited with error during shutdown");
                        }
                        err
                    }
                    res = &mut tcp_handle => {
                        let _ = shutdown_tx.send(true);
                        let err = flatten_join_result("tcp", res);
                        if let Some(other_err) = flatten_join_result("unix", unix_handle.await) {
                            error!(error = %other_err, "unix listener exited with error during shutdown");
                        }
                        err
                    }
                };

                if let Some(err) = first_err {
                    return Err(err);
                }
                Ok(())
            }
            #[cfg(not(unix))]
            {
                let _ = (socket_path, tcp_addr, shutdown_tx, shutdown_rx);
                Err(io::Error::new(
                    io::ErrorKind::Unsupported,
                    "unix domain sockets are not supported on this platform",
                ))
            }
        }
        (Some(socket_path), None) => {
            #[cfg(unix)]
            {
                let (unix_listener, _socket_guard) = bind_unix_listener(&socket_path)?;
                info!(
                    unix_socket = %socket_path.display(),
                    "typst-renderd listening on Unix socket"
                );
                axum::serve(unix_listener, router)
                    .with_graceful_shutdown(wait_for_shutdown(shutdown_rx))
                    .await
            }
            #[cfg(not(unix))]
            {
                let _ = (socket_path, shutdown_tx, shutdown_rx);
                Err(io::Error::new(
                    io::ErrorKind::Unsupported,
                    "unix domain sockets are not supported on this platform",
                ))
            }
        }
        (None, Some(tcp_addr)) => {
            let tcp_listener = TcpListener::bind(tcp_addr).await?;
            let local_tcp = tcp_listener.local_addr().unwrap_or(tcp_addr);
            info!(tcp_addr = %local_tcp, "typst-renderd listening on TCP");
            axum::serve(tcp_listener, router)
                .with_graceful_shutdown(wait_for_shutdown(shutdown_rx))
                .await
        }
        (None, None) => Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            "at least one of --socket or --listen must be configured",
        )),
    }
}

async fn wait_for_shutdown(mut rx: watch::Receiver<bool>) {
    let _ = rx.wait_for(|&signaled| signaled).await;
}

fn flatten_join_result(
    label: &str,
    res: Result<io::Result<()>, tokio::task::JoinError>,
) -> Option<io::Error> {
    match res {
        Ok(Ok(())) => None,
        Ok(Err(err)) => Some(err),
        Err(join_err) => Some(io::Error::other(format!(
            "{label} server task failed: {join_err}"
        ))),
    }
}

async fn shutdown_signal() {
    let ctrl_c = async {
        if let Err(err) = tokio::signal::ctrl_c().await {
            error!(error = %err, "failed to install Ctrl+C handler");
        }
    };

    #[cfg(unix)]
    let terminate = async {
        match tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate()) {
            Ok(mut stream) => {
                stream.recv().await;
            }
            Err(err) => {
                error!(error = %err, "failed to install SIGTERM handler");
                std::future::pending::<()>().await;
            }
        }
    };

    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();

    tokio::select! {
        _ = ctrl_c => {
            info!("received SIGINT, initiating graceful shutdown");
        }
        _ = terminate => {
            info!("received SIGTERM, initiating graceful shutdown");
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn render_metrics_tracks_count_and_average() {
        let metrics = RenderMetrics::default();
        assert_eq!(metrics.snapshot(), (0, 0.0));

        metrics.record(10);
        metrics.record(20);
        metrics.record(13);

        let (total, avg) = metrics.snapshot();
        assert_eq!(total, 3);
        assert!((avg - 14.33).abs() < f64::EPSILON);
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn unix_listener_cleans_stale_socket_and_removes_on_drop() {
        let unique = format!(
            "typst-renderd-test-{}-{}.sock",
            std::process::id(),
            chrono::Utc::now().timestamp_nanos_opt().unwrap_or(0)
        );
        let sock_path = std::env::temp_dir().join(unique);

        // Simulate a stale file left behind by an unclean crash.
        std::fs::write(&sock_path, b"stale").expect("create stale file");
        assert!(sock_path.exists());

        {
            let (_listener, _guard) = bind_unix_listener(&sock_path).expect("bind unix socket");
            assert!(sock_path.exists());
        }

        assert!(
            !sock_path.exists(),
            "socket file should be removed when UnixSocketCleanup drops"
        );
    }
}
