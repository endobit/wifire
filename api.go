package wifire

import (
	"time"
)

//go:generate go tool enumer -type Units -linecomment

type Units int

const (
	UnitsCelsius    Units = iota // celsius
	UnitsFahrenheit              // fahrenheit
)

// MarshalText implements the encoding.TextMarshaler interface for u.
func (u Units) MarshalText() (text []byte, err error) {
	return []byte(u.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for u.
func (u *Units) UnmarshalText(text []byte) error {
	v, err := UnitsString(string(text))
	if err != nil {
		return err
	}

	*u = v

	return nil
}

//go:generate go tool enumer -type SystemStatus -linecomment

type SystemStatus int

const (
	_ SystemStatus = iota
	_
	StatusSleeping      // sleeping
	StatusReady         // ready
	StatusIgniting      // igniting
	StatusHeating       // heating
	StatusCooking       // cooking
	StatusCustomCooking // custom cooking
	StatusKeepWarm      // keep warm
	StatusShutdown      // shutdown

	StatusOffline SystemStatus = 99 // offline
)

// MarshalText implements the encoding.TextMarshaler interface for s.
func (s SystemStatus) MarshalText() (text []byte, err error) {
	return []byte(s.String()), nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for s.
func (s *SystemStatus) UnmarshalText(text []byte) error {
	v, err := SystemStatusString(string(text))
	if err != nil {
		return err
	}

	*s = v

	return nil
}

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

type Update struct {
	ID     int64
	Error  error `json:"error,omitzero"`
	Usage  Usage
	Status Status
}

type Usage struct {
	Auger                    int           `json:"auger"`
	CookCycles               int           `json:"cook_cycles"`
	ErrorStats               ErrorStats    `json:"error_stats"`
	Fan                      int           `json:"fan"`
	GreaseTrapCleanCountdown int           `json:"grease_trap_clean_countdown"`
	GrillCleanCountdown      int           `json:"grill_clean_countdown"`
	Hotrod                   int           `json:"hotrod"`
	RunTime                  time.Duration `json:"runtime"`
	Time                     time.Time     `json:"time"`
}

type ErrorStats errorStats

// Status is the real-time grill status. It is a cleaned up version of the
// status returned from the MQTT subscription. If there was an error receiving
// the message the Error field is set.
//
// The ProbeETA field is calculated separately and is included here for logging.
type Status struct {
	Ambient         int           `json:"ambient"`
	TimerComplete   bool          `json:"timer_complete"`
	TimerEnd        time.Time     `json:"timer_end"`
	TimerStart      time.Time     `json:"timer_start"`
	Connected       bool          `json:"connected"`
	Grill           int           `json:"grill"`
	GrillSet        int           `json:"grill_set"`
	KeepWarm        bool          `json:"keep_warm,omitzero"`
	PelletLevel     int           `json:"pellet_level,omitzero"`
	Probe           int           `json:"probe,omitzero"`
	ProbeAlarmFired bool          `json:"probe_alarm_fired,omitzero"`
	ProbeConnected  bool          `json:"probe_connected,omitzero"`
	ProbeSet        int           `json:"probe_set,omitzero"`
	Smoke           int           `json:"smoke,omitzero"`
	Time            time.Time     `json:"time"`
	Units           Units         `json:"units"`
	SystemStatus    SystemStatus  `json:"system_status"`
	ProbeETA        time.Duration `json:"probe_eta,omitzero,format:units"`
}

// update is returned from the prod/thing/update subscription.
type update struct {
	Status status `json:"status"`
	Usage  usage  `json:"usage"`
}

type usage struct {
	Auger                    int        `json:"auger"`
	CookCycles               int        `json:"cook_cycles"`
	ErrorStats               errorStats `json:"error_stats"`
	Fan                      int        `json:"fan"`
	GreaseTrapCleanCountdown int        `json:"grease_trap_clean_countdown"`
	GrillCleanCountdown      int        `json:"grill_clean_countdown"`
	Hotrod                   int        `json:"hotrod"`
	RunTime                  int64      `json:"runtime"`
	Time                     int64      `json:"time"`
}

type errorStats struct {
	AugerDisconnect   int `json:"auger_disco,omitzero"`
	AugerOverCurrent  int `json:"auger_ovrcur,omitzero"`
	BadThermocouple   int `json:"bad_thermocouple,omitzero"`
	FanDisconnect     int `json:"fan_disco,omitzero"`
	IgniterDisconnect int `json:"ign_disco,omitzero"`
	IgniteFail        int `json:"ignite_fail,omitzero"`
	LowTemperature    int `json:"lowtemp,omitzero"`
	OverHeat          int `json:"overheat,omitzero"`
}

// status is the raw status returned from the MQTT subscription.
type status struct {
	Ambient           int    `json:"ambient"` // temperature
	Connected         bool   `json:"connected"`
	CookID            string `json:"cook_id"`
	CookTimerComplete int    `json:"cook_timer_complete"` // bool
	CookTimerEnd      int64  `json:"cook_timer_end"`      // unix timestamp
	CookTimerStart    int64  `json:"cook_timer_start"`    // unix timestamp
	CurrentCycle      int    `json:"current_cycle"`
	CurrentStep       int    `json:"current_step"`
	Errors            int    `json:"errors"`            // bool?
	Grill             int    `json:"grill"`             // temperature
	InCustom          int    `json:"in_custom"`         // bool?
	KeepWarm          int    `json:"keepwarm"`          // 1 for transitioning to "keep warm" mode, but still "cooking"
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
