//! Axum server: tail `grill.jsonl`, `GET /api/history`, `GET /ws`, static UI.

mod state;
mod tail;

use std::path::PathBuf;
use std::sync::Arc;

use axum::extract::ws::{Message, WebSocket, WebSocketUpgrade};
use axum::extract::{Query, State};
use axum::http::StatusCode;
use axum::response::IntoResponse;
use axum::routing::get;
use axum::Json;
use axum::Router;
use serde::Deserialize;
use state::AppState;
use tokio::sync::broadcast;
use tower_http::cors::{Any, CorsLayer};
use tower_http::services::ServeDir;
use tower_http::trace::TraceLayer;
use tracing::{error, info, warn};

#[derive(Debug, Deserialize)]
struct HistoryParams {
    limit: Option<usize>,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "wifire_dashboard_server=info,tower_http=info".into()),
        )
        .init();

    let path = grill_path();
    info!(?path, "grill jsonl path");

    let (tx, _rx) = broadcast::channel::<String>(256);
    let state = Arc::new(AppState {
        path: path.clone(),
        line_tx: tx.clone(),
    });

    tail::spawn_tail_task(path, tx);

    let static_dir = static_dir();
    info!(?static_dir, "static assets");

    let app = Router::new()
        .route("/api/history", get(history_handler))
        .route("/ws", get(ws_handler))
        .fallback_service(
            ServeDir::new(static_dir).append_index_html_on_directories(true),
        )
        .layer(
            CorsLayer::new()
                .allow_origin(Any)
                .allow_methods(Any)
                .allow_headers(Any),
        )
        .layer(TraceLayer::new_for_http())
        .with_state(state);

    let addr: std::net::SocketAddr = std::env::var("BIND_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:8080".to_string())
        .parse()
        .expect("BIND_ADDR");
    let listener = tokio::net::TcpListener::bind(addr).await.expect("bind");
    info!(%addr, "listening");
    axum::serve(listener, app).await.expect("serve");
}

fn grill_path() -> PathBuf {
    std::env::var("GRILL_JSONL")
        .map(PathBuf::from)
        .unwrap_or_else(|_| PathBuf::from("/data/grill.jsonl"))
}

/// Directory with `index.html` and Trunk `pkg/` (development: `frontend/dist`).
fn static_dir() -> PathBuf {
    std::env::var("STATIC_DIR").map(PathBuf::from).unwrap_or_else(|_| {
        // Running from workspace: server/../frontend/dist
        let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        manifest.join("../frontend/dist")
    })
}

async fn history_handler(
    State(state): State<Arc<AppState>>,
    Query(q): Query<HistoryParams>,
) -> Result<Json<Vec<wifire_dashboard_shared::GrillUpdate>>, StatusCode> {
    let limit = q.limit.unwrap_or(500).max(1).min(10_000);
    let content = tokio::fs::read_to_string(&state.path)
        .await
        .map_err(|e| {
            warn!(?e, "read grill file");
            StatusCode::NOT_FOUND
        })?;

    let lines: Vec<&str> = content.lines().filter(|l| !l.is_empty()).collect();
    let take = lines.len().min(limit);
    let start = lines.len().saturating_sub(take);
    let mut updates = Vec::new();
    for line in &lines[start..] {
        if let Ok(u) = serde_json::from_str::<wifire_dashboard_shared::GrillUpdate>(line) {
            updates.push(u);
        }
    }
    Ok(Json(updates))
}

async fn ws_handler(
    ws: WebSocketUpgrade,
    State(state): State<Arc<AppState>>,
) -> impl IntoResponse {
    ws.on_upgrade(move |socket| handle_ws(socket, state))
}

async fn handle_ws(mut socket: WebSocket, state: Arc<AppState>) {
    let mut rx = state.line_tx.subscribe();
    loop {
        tokio::select! {
            msg = rx.recv() => {
                match msg {
                    Ok(line) => {
                        if socket.send(Message::Text(line.into())).await.is_err() {
                            break;
                        }
                    }
                    Err(broadcast::error::RecvError::Lagged(_)) => {}
                    Err(broadcast::error::RecvError::Closed) => break,
                }
            }
            recv = socket.recv() => {
                match recv {
                    Some(Ok(Message::Close(_))) | None => break,
                    Some(Ok(Message::Ping(p))) => {
                        let _ = socket.send(Message::Pong(p)).await;
                    }
                    Some(Err(e)) => {
                        error!(?e, "ws recv");
                        break;
                    }
                    _ => {}
                }
            }
        }
    }
}
