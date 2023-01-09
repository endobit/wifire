package wifire

import (
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type dbg struct{}
type wrn struct{}
type err struct{}

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

func (dbg) Printf(format string, v ...interface{}) {
	printf(log.Debug(), format, v...)
}

func (dbg) Println(v ...interface{}) {
	println(log.Debug(), v...)
}

func (wrn) Printf(format string, v ...interface{}) {
	printf(log.Warn(), format, v...)
}

func (wrn) Println(v ...interface{}) {
	println(log.Warn(), v...)
}

func (err) Printf(format string, v ...interface{}) {
	printf(log.Error(), format, v...)
}

func (err) Println(v ...interface{}) {
	println(log.Error(), v...)
}

func init() {
	mqtt.ERROR = err{}
	mqtt.CRITICAL = err{}
	mqtt.WARN = wrn{}
	//	mqtt.DEBUG = dbg{}
}
