use std::path::PathBuf;

use tokio::sync::broadcast;

pub struct AppState {
    pub path: PathBuf,
    pub line_tx: broadcast::Sender<String>,
}
