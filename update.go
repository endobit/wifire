package wifire

import (
	"encoding/json"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Status struct {
	Ambient  int
	Grill    int
	Probe    int
	GrillSet int
	ProbeSet int
	Time     time.Time
	Error    error
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
	KeepWarn          int    `json:"keepwarm"`
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

func (g Grill) Status(fn func(Status)) error {
	if !g.client.IsConnected() {
		if err := g.connect(); err != nil {
			return err
		}
	}

	token := g.client.Subscribe("prod/thing/update/"+g.name, 1, func(c mqtt.Client, m mqtt.Message) {
		u := newUpdate(m.Payload())
		fn(u)
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
		Ambient:  msg.Status.Ambient,
		Grill:    msg.Status.Grill,
		Probe:    msg.Status.Probe,
		GrillSet: msg.Status.Set,
		ProbeSet: msg.Status.ProbeSet,
		Time:     time.Unix(msg.Status.Time, 0),
	}
}
