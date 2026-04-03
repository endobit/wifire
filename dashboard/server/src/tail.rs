//! Append-only tail of `grill.jsonl` with line buffering.

use std::path::PathBuf;

use tokio::io::{AsyncReadExt, AsyncSeekExt};
use tokio::sync::broadcast;
use tracing::{debug, warn};

pub fn spawn_tail_task(path: PathBuf, tx: broadcast::Sender<String>) {
    tokio::spawn(async move {
        if let Err(e) = tail_loop(path, tx).await {
            warn!(?e, "tail task ended");
        }
    });
}

async fn tail_loop(path: PathBuf, tx: broadcast::Sender<String>) -> std::io::Result<()> {
    let mut offset: u64 = 0;
    let mut pending = String::new();

    loop {
        match tokio::fs::File::open(&path).await {
            Ok(mut file) => {
                let len = file.metadata().await?.len();
                if len < offset {
                    offset = 0;
                    pending.clear();
                }
                if len > offset {
                    file.seek(std::io::SeekFrom::Start(offset)).await?;
                    let mut buf = Vec::new();
                    file.read_to_end(&mut buf).await?;
                    offset = file.metadata().await?.len();
                    pending.push_str(&String::from_utf8_lossy(&buf));
                    flush_lines(&mut pending, &tx);
                }
            }
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {
                debug!("grill file not yet present, retrying");
            }
            Err(e) => return Err(e),
        }
        tokio::time::sleep(std::time::Duration::from_millis(500)).await;
    }
}

fn flush_lines(pending: &mut String, tx: &broadcast::Sender<String>) {
    loop {
        if let Some(i) = pending.find('\n') {
            let line: String = pending[..i].to_string();
            pending.drain(..=i);
            let t = line.trim();
            if t.is_empty() {
                continue;
            }
            if serde_json::from_str::<wifire_dashboard_shared::GrillUpdate>(t).is_err() {
                warn!(len = t.len(), "skip malformed json line");
                continue;
            }
            let _ = tx.send(t.to_string());
        } else {
            break;
        }
    }
}
