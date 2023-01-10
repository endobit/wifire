package wifire

import (
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// LogLevel is the log level for MQTT calls.
type LogLevel int

const (
	_ LogLevel = iota
	// LogError maps to mqtt.ERROR and mqtt.CRITICAL.
	LogError
	// LogWarn maps to mqtt.WARN.
	LogWarn
	// LogInfo does not have a mqtt level.
	LogInfo
	// LogDebug maps to mqtt.DEBUG.
	LogDebug
)

// Logger is the package global logging handler.
var Logger func(level LogLevel, component string, message string)

func logf(l LogLevel, format string, v ...interface{}) {
	if Logger == nil {
		return
	}

	Logger(l, "", strings.Trim(fmt.Sprintf(format, v...), "[]"))
}

func logln(l LogLevel, v ...interface{}) {
	if Logger == nil {
		return
	}

	var comp string

	if len(v) > 1 {
		comp = strings.Trim(strings.TrimSpace(fmt.Sprint(v[0])), "[]")
		v = v[1:]
	}

	Logger(l, comp, strings.Trim(fmt.Sprint(v...), "[]"))
}

type (
	dbg struct{}
	wrn struct{}
	err struct{}
)

func (dbg) Printf(format string, v ...interface{}) { logf(LogDebug, format, v...) }
func (dbg) Println(v ...interface{})               { logln(LogDebug, v...) }

func (wrn) Printf(format string, v ...interface{}) { logf(LogWarn, format, v...) }
func (wrn) Println(v ...interface{})               { logln(LogWarn, v...) }

func (err) Printf(format string, v ...interface{}) { logf(LogError, format, v...) }
func (err) Println(v ...interface{})               { logln(LogError, v...) }

func init() {
	mqtt.ERROR = err{}
	mqtt.CRITICAL = err{}
	mqtt.WARN = wrn{}
	mqtt.DEBUG = dbg{}
}
