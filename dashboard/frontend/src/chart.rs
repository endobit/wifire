use leptos::prelude::*;
use wasm_bindgen::JsValue;
use wifire_dashboard_shared::GrillUpdate;

#[derive(Clone, Copy, Debug)]
pub struct ChartPoint {
    pub t: f64,
    pub probe: f64,
    pub grill: f64,
    pub grill_set: f64,
    pub ambient: f64,
}

pub fn series_from_updates(updates: &[GrillUpdate]) -> Vec<ChartPoint> {
    updates.iter().filter_map(point_from_update).collect()
}

pub fn append_point(v: &mut Vec<ChartPoint>, u: &GrillUpdate) {
    if let Some(p) = point_from_update(u) {
        v.push(p);
    }
}

fn point_from_update(u: &GrillUpdate) -> Option<ChartPoint> {
    let t = parse_time_s(&u.status.time)?;
    Some(ChartPoint {
        t,
        probe: u.status.probe as f64,
        grill: u.status.grill as f64,
        grill_set: u.status.grill_set as f64,
        ambient: u.status.ambient as f64,
    })
}

fn parse_time_s(s: &str) -> Option<f64> {
    let v = js_sys::Date::new(&JsValue::from_str(s)).get_time();
    if v.is_nan() {
        None
    } else {
        Some(v / 1000.0)
    }
}

fn map_x(t: f64, t_min: f64, t_max: f64, w: f64, m_l: f64, m_r: f64) -> f64 {
    let inner = w - m_l - m_r;
    if (t_max - t_min).abs() < f64::EPSILON {
        m_l + inner / 2.0
    } else {
        m_l + (t - t_min) / (t_max - t_min) * inner
    }
}

fn map_y(
    temp: f64,
    temp_min: f64,
    temp_max: f64,
    h: f64,
    m_t: f64,
    m_b: f64,
) -> f64 {
    let inner = h - m_t - m_b;
    if (temp_max - temp_min).abs() < f64::EPSILON {
        m_t + inner / 2.0
    } else {
        m_t + (1.0 - (temp - temp_min) / (temp_max - temp_min)) * inner
    }
}

fn poly_attr(
    pts: &[ChartPoint],
    t_min: f64,
    t_max: f64,
    temp_min: f64,
    temp_max: f64,
    w: f64,
    h: f64,
    m_l: f64,
    m_r: f64,
    m_t: f64,
    m_b: f64,
    temp: impl Fn(&ChartPoint) -> f64,
) -> String {
    pts
        .iter()
        .map(|p| {
            format!(
                "{:.1},{:.1}",
                map_x(p.t, t_min, t_max, w, m_l, m_r),
                map_y(temp(p), temp_min, temp_max, h, m_t, m_b)
            )
        })
        .collect::<Vec<_>>()
        .join(" ")
}

fn time_tick_seconds(t_min: f64, t_max: f64, interval_minutes: u32) -> Vec<f64> {
    let interval_sec = (interval_minutes.clamp(1, 10_080) as f64) * 60.0;
    if t_max <= t_min {
        return vec![];
    }
    let mut ticks = Vec::new();
    let mut t = (t_min / interval_sec).ceil() * interval_sec;
    while t <= t_max + 1e-6 {
        ticks.push(t);
        t += interval_sec;
    }
    ticks
}

fn format_tick_time(t_sec: f64) -> String {
    let d = js_sys::Date::new(&JsValue::from_f64(t_sec * 1000.0));
    let h = d.get_hours();
    let m = d.get_minutes();
    format!("{h:02}:{m:02}")
}

#[component]
pub fn TemperatureChart(
    points: RwSignal<Vec<ChartPoint>>,
    tick_interval_minutes: RwSignal<u32>,
) -> impl IntoView {
    let svg = move || {
        let pts = points.get();
        let interval_min = tick_interval_minutes.get();
        if pts.len() < 2 {
            return view! { <p>"Waiting for points…"</p> }.into_any();
        }

        let t_min = pts.iter().map(|p| p.t).fold(f64::INFINITY, f64::min);
        let t_max = pts.iter().map(|p| p.t).fold(f64::NEG_INFINITY, f64::max);
        let temp_min = pts
            .iter()
            .flat_map(|p| [p.probe, p.grill, p.grill_set, p.ambient])
            .fold(f64::INFINITY, f64::min)
            - 5.0;
        let temp_max = pts
            .iter()
            .flat_map(|p| [p.probe, p.grill, p.grill_set, p.ambient])
            .fold(f64::NEG_INFINITY, f64::max)
            + 5.0;

        let w = 1000.0;
        let h = 400.0;
        let m_l = 52.0;
        let m_r = 16.0;
        let m_t = 28.0;
        let m_b = 56.0;

        let ticks = time_tick_seconds(t_min, t_max, interval_min);
        let ticks_for_lines = ticks.clone();
        let ticks_for_labels = ticks;

        let s_probe = poly_attr(
            &pts,
            t_min,
            t_max,
            temp_min,
            temp_max,
            w,
            h,
            m_l,
            m_r,
            m_t,
            m_b,
            |p| p.probe,
        );
        let s_grill = poly_attr(
            &pts,
            t_min,
            t_max,
            temp_min,
            temp_max,
            w,
            h,
            m_l,
            m_r,
            m_t,
            m_b,
            |p| p.grill,
        );
        let s_set = poly_attr(
            &pts,
            t_min,
            t_max,
            temp_min,
            temp_max,
            w,
            h,
            m_l,
            m_r,
            m_t,
            m_b,
            |p| p.grill_set,
        );
        let s_amb = poly_attr(
            &pts,
            t_min,
            t_max,
            temp_min,
            temp_max,
            w,
            h,
            m_l,
            m_r,
            m_t,
            m_b,
            |p| p.ambient,
        );

        let y0 = h - m_b;
        view! {
            <svg viewBox=format!("0 0 {w} {h}") preserveAspectRatio="xMidYMid meet" class="temp-chart-svg">
                <rect x="0" y="0" width=w height=h fill="#161b22" />
                <text x=m_l y="22" fill="#8b949e" font-size="12">"°F vs time"</text>
                <line
                    x1=m_l
                    y1=y0
                    x2=w - m_r
                    y2=y0
                    stroke="#484f58"
                    stroke-width="1"
                />
                <For
                    each=move || ticks_for_lines.clone()
                    key=|ts| format!("g{ts:.3}")
                    children=move |ts| {
                        let x = map_x(ts, t_min, t_max, w, m_l, m_r);
                        view! {
                            <line
                                x1=x
                                y1=m_t
                                x2=x
                                y2=h - m_b
                                stroke="#30363d"
                                stroke-width="1"
                                stroke-dasharray="4 4"
                            />
                        }
                    }
                />
                <For
                    each=move || ticks_for_labels.clone()
                    key=|ts| format!("l{ts:.3}")
                    children=move |ts| {
                        let x = map_x(ts, t_min, t_max, w, m_l, m_r);
                        let label = format_tick_time(ts);
                        view! {
                            <text
                                x=x
                                y=h - 18.0
                                fill="#8b949e"
                                font-size="11"
                                text-anchor="middle"
                            >
                                {label}
                            </text>
                        }
                    }
                />
                <polyline fill="none" stroke="#58a6ff" stroke-width="2" points=s_probe />
                <polyline fill="none" stroke="#f85149" stroke-width="2" points=s_grill />
                <polyline fill="none" stroke="#3fb950" stroke-width="2" points=s_set />
                <polyline fill="none" stroke="#d2a8ff" stroke-width="2" points=s_amb />
            </svg>
        }
        .into_any()
    };

    view! { <div class="chart-svg-inner">{svg}</div> }
}
