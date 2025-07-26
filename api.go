package wifire

import (
	"encoding/json"
	"time"
)

//go:generate go tool enumer -type Units -linecomment

type Units int

const (
	UnitsCelsius    Units = iota // celsius
	UnitsFahrenheit              // fahrenheit
)

//go:generate go tool enumer -type SystemStatus -linecomment

type SystemStatus int

const (
	_ SystemStatus = iota
	_
	_
	StatusReady // ready
	_
	_
	StatusCooking                   // cooking
	StatusOffline SystemStatus = 99 // offline
)

// GetUserDataResponse is the wifire UserData.
type GetUserDataResponse struct {
	Cognito        string  `json:"cognito"`
	CustomerID     string  `json:"customerId"`
	Email          string  `json:"email"`
	FamiltyName    string  `json:"familyName"`
	FullName       string  `json:"fullName"`
	GivenName      string  `json:"givenName"`
	Teams          []team  `json:"teams"`
	Things         []thing `json:"things"`
	UrbanAirshipID string  `json:"urbanAirshipId"`
	UserID         string  `json:"userId"`
	Username       string  `json:"username"`
}

type team struct {
	IsAdmin   bool   `json:"isAdmin"`
	JoinDate  string `json:"joinDate"`
	TeamID    string `json:"teamId"`
	TeamName  string `json:"teamName"`
	TeamRowID string `json:"teamRowId"`
	ThingName string `json:"thingName"`
	UserID    string `json:"userId"`
}

type thing struct {
	DeviceTypeID string     `json:"deviceTypeId"`
	FriendlyName string     `json:"friendlyName"`
	GrillModel   grillModel `json:"grillModel"`
	ProductID    string     `json:"productId"`
	Status       string     `json:"status"`
	ThingName    string     `json:"thingName"`
	UserID       string     `json:"userId"`
}

type grillModel struct {
	Controller         string `json:"controller"`
	Description        string `json:"description"`
	DeviceTypeID       string `json:"deviceTypeId"`
	Group              string `json:"group"`
	Image              image  `json:"image"`
	IOTCapable         bool   `json:"iotCapable"`
	IsTraeger          bool   `json:"isTraegerBrand"`
	Make               string `json:"make"`
	ModuelNumber       string `json:"modelNumber"`
	Name               string `json:"name"`
	OwnersManualURL    string `json:"ownersManualUrl"`
	ReferenceProductID string `json:"referenceProductId"`
}

type image struct {
	DefaultHost string `json:"defaultHost"`
	Endpoint    string `json:"endpoint"`
	Name        string `json:"name"`
}

type getMQTTResponse struct {
	ExpirationSeconds int    `json:"expirationSeconds"`
	ExpiresAt         int    `json:"expiresAt"`
	SignedURL         string `json:"signedUrl"`
}

// Status is the real-time grill status. It is a cleaned up version of the
// status returned from the MQTT subscription. If there was an error receiving
// the message the Error field is set.
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
	Units           Units        `json:"units"`
	SystemStatus    SystemStatus `json:"system_status"`
}

type prodThingUpdate struct {
	Status status `json:"status"`
}

// status is the raw message returned from the MQTT subscription.
type status struct {
	Ambient           int    `json:"ambient"`   // temperature
	Connected         bool   `json:"connected"` // bool
	CookID            string `json:"cook_id"`
	CooKTimerComplete int    `json:"cook_timer_complete"` // bool?
	CookTimerEnd      int    `json:"cook_timer_end"`      // unix timestamp?
	CookTimerStrart   int    `json:"cook_timer_start"`    // unix timestamp?
	CurrentCycle      int    `json:"current_cycle"`
	CurrentStep       int    `json:"current_step"`
	Errors            int    `json:"errors"`            // bool?
	Grill             int    `json:"grill"`             // temperature
	InCustom          int    `json:"in_custom"`         // bool?
	KeepWarm          int    `json:"keepwarm"`          // bool?
	PelletLevel       int    `json:"pellet_level"`      // unknown - my grill doesn't have pellet monitor
	Probe             int    `json:"probe"`             // temperature
	ProbeAlarmFired   int    `json:"probe_alarm_fired"` // bool
	ProbeConnected    int    `json:"probe_con"`         // bool
	ProbeSet          int    `json:"probe_set"`         // temperature
	RealTime          int    `json:"real_time"`
	ServerStatus      int    `json:"server_status"`      // 1=online
	Set               int    `json:"set"`                // temperature
	Smoke             int    `json:"smoke"`              // bool? - my grill doesn't have super smoke
	SysTimerComplete  int    `json:"sys_timer_complete"` // bool?
	SysTimerEnd       int    `json:"sys_timer_end"`      // unix timestamp?
	SysTimerStart     int    `json:"sys_timer_start"`    // unix timestamp?
	SystemStatus      int    `json:"system_status"`      // 3=ready, 99=offline
	Time              int64  `json:"time"`               // unix timestamp
	Units             int    `json:"units"`              // 0 for celsius, 1 for fahrenheit
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
