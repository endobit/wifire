# WiFire dashboard (Rust + Leptos)

Live temperature chart for [`grill.jsonl`](../data/grill.jsonl) (one JSON `Update` per line from the WiFire monitor).

## Behavior

- **`GET /api/history?limit=500`** — last *limit* parsed updates from the JSONL file.
- **`GET /ws`** — WebSocket text frames, each a full JSON line (same shape as the file).
- **Static UI** — Leptos CSR app (SVG chart: probe, grill, grill set, ambient vs. time).

The server **tails** `GRILL_JSONL` (default `/data/grill.jsonl`) by polling the file size every 500ms and only forwards **complete newline-delimited JSON** rows.

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `GRILL_JSONL` | `/data/grill.jsonl` | Path to the JSONL log |
| `STATIC_DIR` | `../frontend/dist` (dev) / `/app/static` (image) | Static UI: `index.html` + `pkg/` (from `./scripts/build-frontend.sh` or Docker build) |
| `BIND_ADDR` | `0.0.0.0:8080` | Listen address |
| `RUST_LOG` | — | `tracing` filter |

Optional hardening (not implemented in code): put the service behind a reverse proxy and require auth at the proxy.

## Local development

1. Install `wasm-bindgen` once: `cargo install wasm-bindgen-cli --locked`

2. Build the WASM UI and static assets:

   ```bash
   ./scripts/build-frontend.sh
   ```

3. Run the API + static files:

   ```bash
   export CARGO_TARGET_DIR="$(pwd)/target"
   STATIC_DIR="$(pwd)/frontend/dist" \
   GRILL_JSONL="$(pwd)/../data/grill.jsonl" \
   cargo run -p wifire-dashboard-server --release
   ```

4. Open `http://127.0.0.1:8080/`.

## Docker

From `dashboard/` (repository root: the `wifire` tree):

```bash
docker build -t wifire-dashboard .
docker run --rm -p 8080:8080 -v "$(pwd)/../data:/data:ro" wifire-dashboard
```

Or use the `dashboard` service in [`../docker-compose.yml`](../docker-compose.yml) next to the `wifire` service (shared `./data` volume).
