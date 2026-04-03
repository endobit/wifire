//! JSON shapes produced by `wifire` (Go) for each line in `grill.jsonl`.

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GrillUpdate {
    #[serde(rename = "ID")]
    pub id: i64,
    #[serde(rename = "Usage")]
    pub usage: Usage,
    #[serde(rename = "Status")]
    pub status: GrillStatus,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Usage {
    pub auger: i64,
    pub cook_cycles: i64,
    pub error_stats: serde_json::Value,
    pub fan: i64,
    pub grease_trap_clean_countdown: i64,
    pub grill_clean_countdown: i64,
    pub hotrod: i64,
    /// Go json v2 `format:units` — string like `42h43m8s`
    pub runtime: String,
    pub time: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GrillStatus {
    pub ambient: i64,
    pub timer_complete: bool,
    pub timer_end: String,
    pub timer_start: String,
    pub connected: bool,
    pub grill: i64,
    pub grill_set: i64,
    #[serde(default)]
    pub keep_warm: bool,
    #[serde(default)]
    pub pellet_level: i64,
    pub probe: i64,
    #[serde(default)]
    pub probe_alarm_fired: bool,
    #[serde(default)]
    pub probe_connected: bool,
    pub probe_set: i64,
    #[serde(default)]
    pub smoke: i64,
    pub time: String,
    pub units: String,
    pub system_status: String,
    #[serde(default)]
    pub probe_eta: String,
}
