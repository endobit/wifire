package main

import (
	"context"
	"log/slog"

	"endobit.io/wifire"
)

func logger(level wifire.LogLevel, component, msg string) {
	var slogLevel slog.Level

	switch level {
	case wifire.LogDebug:
		slogLevel = slog.LevelDebug
	case wifire.LogInfo:
		slogLevel = slog.LevelInfo
	case wifire.LogWarn:
		slogLevel = slog.LevelWarn
	case wifire.LogError:
		slogLevel = slog.LevelError
	default:
		return
	}

	if component != "" {
		slog.LogAttrs(context.TODO(), slogLevel, msg, slog.String("component", component))
	} else {
		slog.LogAttrs(context.TODO(), slogLevel, msg)
	}
}
