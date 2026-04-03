# syntax=docker/dockerfile:1

FROM rust:bookworm AS frontend-build

WORKDIR /build

ENV CARGO_TARGET_DIR=/build/target

COPY Cargo.toml Cargo.lock ./
COPY shared ./shared
COPY server ./server
COPY frontend ./frontend

RUN rustup target add wasm32-unknown-unknown \
	&& cargo install wasm-bindgen-cli --locked \
	&& cargo build --release --target wasm32-unknown-unknown -p wifire-dashboard-frontend \
	&& mkdir -p /app/static/pkg \
	&& wasm-bindgen target/wasm32-unknown-unknown/release/wifire_dashboard_frontend.wasm \
		--out-dir /app/static/pkg --target web --no-typescript \
	&& cp frontend/style.css /app/static/ \
	&& cp frontend/index.html /app/static/

FROM rust:bookworm AS server-build

WORKDIR /build

ENV CARGO_TARGET_DIR=/build/target

COPY Cargo.toml Cargo.lock ./
COPY shared ./shared
COPY server ./server
COPY frontend ./frontend

RUN cargo build --release -p wifire-dashboard-server

FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates \
	&& rm -rf /var/lib/apt/lists/* \
	&& useradd -m -u 1000 app

COPY --from=frontend-build /app/static /app/static
COPY --from=server-build /build/target/release/wifire-dashboard /usr/local/bin/wifire-dashboard

USER app
WORKDIR /app

ENV STATIC_DIR=/app/static
ENV GRILL_JSONL=/data/grill.jsonl
ENV BIND_ADDR=0.0.0.0:8080
ENV RUST_LOG=wifire_dashboard_server=info,tower_http=info

EXPOSE 8080

CMD ["/usr/local/bin/wifire-dashboard"]
