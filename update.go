package wifire

import (
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Status is the grill status returned from the MQTT subscription. If there was
// an error receiving the message the Error field is set.
type Status struct {
	Error           error        `json:"error,omitempty"`
	Ambient         int          `json:"ambient"`
	Connected       bool         `json:"connected"`
	Grill           int          `json:"grill"`
	GrillSet        int          `json:"grill_set"`
	KeepWarm        int          `json:"keep_warm,omitempty"`
	PelletLevel     int          `json:"pellet_level,omitempty"`
	Probe           int          `json:"probe,omitempty"`
	ProbeAlarmFired bool         `json:"probe_alarm_fired,omitempty"`
	ProbeConnected  bool         `json:"probe_connected,omitempty"`
	ProbeETA        JSONDuration `json:"probe_eta,omitempty"`
	ProbeSet        int          `json:"probe_set,omitempty"`
	RealTime        int          `json:"real_time,omitempty"`
	Smoke           int          `json:"smoke,omitempty"`
	Time            time.Time    `json:"time"`
	Units           int          `json:"units"`
}

type prodThingUpdate struct {
	Status status `json:"status"`
}

type status struct {
	Ambient           int    `json:"ambient"` // temperature
	Connected         bool   `json:"connected"`
	CookID            string `json:"cook_id"`
	CooKTimerComplete int    `json:"cook_timer_complete"`
	CookTimerEnd      int    `json:"cook_timer_end"`
	CookTimerStrart   int    `json:"cook_timer_start"`
	CurrentCycle      int    `json:"current_cycle"`
	CurrentStep       int    `json:"current_step"`
	Errors            int    `json:"errors"`
	Grill             int    `json:"grill"`
	InCustom          int    `json:"in_custom"`
	KeepWarm          int    `json:"keepwarm"`
	PelletLevel       int    `json:"pellet_level"`
	Probe             int    `json:"probe"` // temperature
	ProbeAlarmFired   int    `json:"probe_alarm_fired"`
	ProbeConnected    int    `json:"probe_con"`
	ProbeSet          int    `json:"probe_set"` // temperature
	RealTime          int    `json:"real_time"`
	ServerStatus      int    `json:"server_status"`
	Set               int    `json:"set"` // temperature
	Smoke             int    `json:"smoke"`
	SysTimerComplete  int    `json:"sys_timer_complete"`
	SysTimerEnd       int    `json:"sys_timer_end"`
	SysTimerStart     int    `json:"sys_timer_start"`
	SystemStatus      int    `json:"system_status"`
	Time              int64  `json:"time"`
	Units             int    `json:"units"`
}

// SubscribeStatus subscribes to the prod/thing/update for the grill. SubscribeStatus
// updates are pushed to the returned channel.
func (g *Grill) SubscribeStatus(ch chan Status) error {
	if !g.client.IsConnected() {
		if err := g.connect(); err != nil {
			return err
		}
	}

	token := g.client.Subscribe("prod/thing/update/"+g.name, 1, func(_ mqtt.Client, m mqtt.Message) {
		ch <- newUpdate(m.Payload())
	})

	token.Wait()

	return nil
}

func newUpdate(data []byte) Status {
	var msg prodThingUpdate

	if err := json.Unmarshal(data, &msg); err != nil {
		return Status{Error: err}
	}

	return Status{
		Ambient:         msg.Status.Ambient,
		Connected:       msg.Status.Connected,
		Grill:           msg.Status.Grill,
		GrillSet:        msg.Status.Set,
		KeepWarm:        msg.Status.KeepWarm,
		PelletLevel:     msg.Status.PelletLevel,
		Probe:           msg.Status.Probe,
		ProbeAlarmFired: msg.Status.ProbeAlarmFired != 0,
		ProbeConnected:  msg.Status.ProbeConnected != 0,
		ProbeSet:        msg.Status.ProbeSet,
		RealTime:        msg.Status.RealTime,
		Smoke:           msg.Status.Smoke,
		Time:            time.Unix(msg.Status.Time, 0),
		Units:           msg.Status.Units,
	}
}

// JSONDuration is a custom type that marshals time.Duration to JSON as a string
type JSONDuration time.Duration

// MarshalJSON implements json.Marshaler interface for JSONDuration
func (d JSONDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements json.Unmarshaler interface for JSONDuration
func (d *JSONDuration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return err
	}

	*d = JSONDuration(duration)

	return nil
}

// Duration returns the underlying time.Duration
func (d JSONDuration) Duration() time.Duration {
	return time.Duration(d)
}
