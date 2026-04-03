#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export CARGO_TARGET_DIR="$ROOT/target"
rustup target add wasm32-unknown-unknown
cargo build --release --target wasm32-unknown-unknown -p wifire-dashboard-frontend
mkdir -p frontend/dist/pkg
WASM="$CARGO_TARGET_DIR/wasm32-unknown-unknown/release/wifire_dashboard_frontend.wasm"
wasm-bindgen "$WASM" --out-dir frontend/dist/pkg --target web --no-typescript
cp frontend/style.css frontend/dist/
cp frontend/index.html frontend/dist/
echo "Built frontend → frontend/dist (serve with STATIC_DIR=$ROOT/frontend/dist)"
