mod chart;

use chart::{series_from_updates, ChartPoint};
use futures::StreamExt;
use gloo_net::http::Request;
use gloo_net::websocket::futures::WebSocket;
use gloo_net::websocket::Message;
use leptos::logging::error;
use leptos::prelude::*;
use leptos::task::spawn_local;
use wifire_dashboard_shared::GrillUpdate;

#[wasm_bindgen::prelude::wasm_bindgen(start)]
pub fn main() {
    console_error_panic_hook::set_once();
    leptos::mount::mount_to_body(|| view! { <App /> });
}

#[component]
fn App() -> impl IntoView {
    let points = RwSignal::new(Vec::<ChartPoint>::new());
    let status = RwSignal::new(String::from("Loading history…"));
    let connected = RwSignal::new(false);
    let tick_interval_minutes = RwSignal::new(30u32);

    let _ = Effect::new(move |_| {
        spawn_local({
            let points = points;
            let status = status;
            let connected = connected;
            async move {
                match Request::get("/api/history?limit=500").send().await {
                    Ok(resp) => {
                        if resp.ok() {
                            match resp.text().await {
                                Ok(text) => {
                                    match serde_json::from_str::<Vec<GrillUpdate>>(&text) {
                                        Ok(updates) => {
                                            points.set(series_from_updates(&updates));
                                            status.set(format!("Loaded {} points", points.get().len()));
                                        }
                                        Err(e) => {
                                            status.set(format!("Bad history JSON: {e}"));
                                        }
                                    }
                                }
                                Err(e) => status.set(format!("Read body: {e}")),
                            }
                        } else {
                            status.set(format!("History HTTP {}", resp.status()));
                        }
                    }
                    Err(e) => status.set(format!("Fetch history: {e}")),
                }

                let ws_url = match ws_url() {
                    Ok(u) => u,
                    Err(e) => {
                        status.set(format!("WS URL: {e}"));
                        return;
                    }
                };

                loop {
                    match WebSocket::open(&ws_url) {
                        Ok(mut ws) => {
                            connected.set(true);
                            status.set("Live (WebSocket)".to_string());
                            while let Some(msg) = ws.next().await {
                                match msg {
                                    Ok(Message::Text(t)) => {
                                        if let Ok(u) = serde_json::from_str::<GrillUpdate>(&t) {
                                            points.update(|v| {
                                                chart::append_point(v, &u);
                                                let cap = 2000usize;
                                                if v.len() > cap {
                                                    let drop = v.len() - cap;
                                                    v.drain(0..drop);
                                                }
                                            });
                                        }
                                    }
                                    Ok(Message::Bytes(_)) => {}
                                    Err(e) => {
                                        error!("ws: {e:?}");
                                        break;
                                    }
                                }
                            }
                            connected.set(false);
                        }
                        Err(e) => {
                            status.set(format!("WS open failed: {e:?}"));
                        }
                    }

                    gloo_timers::future::TimeoutFuture::new(1_000).await;
                }
            }
        });
    });

    view! {
        <header class="page-header">
            <h1>"WiFire live"</h1>
            <div class="toolbar">
                <p class="status">
                    {move || status.get()}
                    " — "
                    {move || if connected.get() { "connected" } else { "reconnecting…" }}
                </p>
                <div class="interval-form">
                    <label class="interval-label" for="tick-interval">"Time grid (minutes)"</label>
                    <input
                        id="tick-interval"
                        class="interval-input"
                        type="number"
                        min="1"
                        max="10080"
                        step="1"
                        prop:value=move || tick_interval_minutes.get().to_string()
                        on:input=move |ev| {
                            let s = event_target_value(&ev);
                            if s.is_empty() {
                                return;
                            }
                            if let Ok(n) = s.parse::<u32>() {
                                tick_interval_minutes.set(n.clamp(1, 10_080));
                            }
                        }
                    />
                </div>
            </div>
        </header>
        <div class="chart-wrap">
            <chart::TemperatureChart points=points tick_interval_minutes=tick_interval_minutes />
            <div class="legend">
                <span><i class="swatch" style="background:#58a6ff"></i>"probe"</span>
                <span><i class="swatch" style="background:#f85149"></i>"grill"</span>
                <span><i class="swatch" style="background:#3fb950"></i>"grill set"</span>
                <span><i class="swatch" style="background:#d2a8ff"></i>"ambient"</span>
            </div>
        </div>
    }
}

fn ws_url() -> Result<String, String> {
    let window = web_sys::window().ok_or("no window")?;
    let loc = window.location();
    let host = loc.host().map_err(|_| "no host")?;
    let proto = loc.protocol().map_err(|_| "no protocol")?;
    let scheme = if proto == "https:" { "wss" } else { "ws" };
    Ok(format!("{scheme}://{host}/ws"))
}
