package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/endobit/clog"
	"github.com/endobit/wifire"
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

func newRootCmd() *cobra.Command {
	var (
		output             string
		username, password string
		logLevel           string
		debug              bool
	)

	cmd := cobra.Command{
		Use:     "wifire",
		Short:   "Traeger WiFire Grill Util",
		Version: version,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			var level slog.Level

			if err := level.UnmarshalText([]byte(logLevel)); err != nil {
				return fmt.Errorf("invalid log level %q", logLevel)
			}

			opts := clog.HandlerOptions{Level: level}
			slog.SetDefault(slog.New(opts.NewHandler(os.Stderr)))

			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			if debug {
				wifire.Logger = logger
			}

			w, err := wifire.New(wifire.Credentials(username, password))
			if err != nil {
				return err
			}

			data, err := w.UserData()
			if err != nil {
				return err
			}

			grill := w.NewGrill(data.Things[0].Name)
			if err := grill.Connect(); err != nil {
				return err
			}

			defer grill.Disconnect()

			if output != "" {
				fout, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
				if err != nil {
					return err
				}

				defer fout.Close()

				go status(grill, fout)
			} else {
				go status(grill, nil)
			}

			catch := make(chan os.Signal, 1)
			signal.Notify(catch, syscall.SIGINT, syscall.SIGTERM)
			<-catch

			return nil
		},
	}

	info := strings.ToLower(slog.LevelInfo.String())
	cmd.PersistentFlags().StringVar(&logLevel, "log", info, "log level")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug wifire API")
	cmd.Flags().StringVar(&username, "username", "", "account username")
	cmd.Flags().StringVar(&password, "password", "", "account password")
	cmd.Flags().StringVar(&output, "output", "", "log to file")

	if err := cmd.MarkFlagRequired("username"); err != nil {
		panic(err)
	}

	if err := cmd.MarkFlagRequired("password"); err != nil {
		panic(err)
	}

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newPlotCmd())

	return &cmd
}

func status(g *wifire.Grill, out io.Writer) {
	ch := make(chan wifire.Status, 1)

	if err := g.SubscribeStatus(ch); err != nil {
		slog.Error("cannot subscribe to status", "error", err)

		return
	}

	for {
		msg := <-ch
		if msg.Error != nil {
			slog.Error("invalid status", "error", msg.Error)
		}

		slog.LogAttrs(context.TODO(), slog.LevelInfo, "",
			slog.Int("ambient", msg.Ambient),
			slog.Int("grill", msg.Grill),
			slog.Int("grill_set", msg.GrillSet),
			slog.Int("probe", msg.Probe),
			slog.Int("probe_set", msg.ProbeSet),
			slog.Bool("probe_alarm", msg.ProbeAlarmFired))

		if out != nil {
			b, err := json.Marshal(msg)
			if err != nil {
				slog.Error("cannot marshal", "error", err)
			}

			_, _ = out.Write(b)
			_, _ = out.Write([]byte("\n"))
		}
	}
}
