package wifire

import (
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Logging is an option setting function for New(). It sets the zerolog logging
// level for the IOT MQTT calls to the WiFire API. MQTT debug level messages are
// mapped mapped to the Trace level.
func Logging(logger zerolog.Logger, level zerolog.Level) func(*WiFire) {
	return func(w *WiFire) {
		if level <= zerolog.ErrorLevel {
			mqtt.ERROR = err{}
			mqtt.CRITICAL = err{}
		}

		if level <= zerolog.WarnLevel {
			mqtt.WARN = wrn{}
		}

		if level <= zerolog.TraceLevel {
			mqtt.DEBUG = trc{} // map DEBUG to Trace (really is low level)
		}
	}
}

func printf(e *zerolog.Event, format string, v ...interface{}) {
	e.Msg(strings.Trim(fmt.Sprintf(format, v...), "[]"))
}

func println(e *zerolog.Event, v ...interface{}) {
	if len(v) > 1 {
		comp := strings.TrimSpace(fmt.Sprint(v[0]))
		comp = strings.Trim(comp, "[]")
		e = e.Str("component", comp)
		v = v[1:]
	}

	msg := fmt.Sprint(v...)
	e.Msg(strings.Trim(msg, "[]"))
}

type (
	trc struct{}
	wrn struct{}
	err struct{}
)

func (trc) Printf(format string, v ...interface{}) { printf(log.Trace(), format, v...) }
func (trc) Println(v ...interface{})               { println(log.Trace(), v...) }

func (wrn) Printf(format string, v ...interface{}) { printf(log.Warn(), format, v...) }
func (wrn) Println(v ...interface{})               { println(log.Warn(), v...) }

func (err) Printf(format string, v ...interface{}) { printf(log.Error(), format, v...) }
func (err) Println(v ...interface{})               { println(log.Error(), v...) }
