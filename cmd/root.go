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
	var sl slog.Level

	switch level {
	case wifire.LogDebug:
		sl = slog.LevelDebug
	case wifire.LogInfo:
		sl = slog.LevelInfo
	case wifire.LogWarn:
		sl = slog.LevelWarn
	case wifire.LogError:
		sl = slog.LevelError
	default:
		return
	}

	if component != "" {
		slog.LogAttrs(context.TODO(), sl, msg, slog.String("component", component))
	} else {
		slog.LogAttrs(context.TODO(), sl, msg)
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var level slog.Level

			if err := level.UnmarshalText([]byte(logLevel)); err != nil {
				return fmt.Errorf("invalid log level %q", logLevel)
			}

			opts := clog.HandlerOptions{Level: level}
			slog.SetDefault(slog.New(opts.NewHandler(os.Stderr)))

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			g := w.NewGrill(data.Things[0].Name)
			if err := g.Connect(); err != nil {
				return err
			}

			defer g.Disconnect()

			if output != "" {
				fout, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666)
				if err != nil {
					return err
				}

				defer fout.Close()

				go status(g, fout)
			} else {
				go status(g, nil)
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

func status(g *wifire.Grill, w io.Writer) {
	ch := make(chan wifire.Status, 1)

	if err := g.SubscribeStatus(ch); err != nil {
		slog.Error("cannot subscribe to status", "error", err)
		return
	}

	for {
		s := <-ch
		if s.Error != nil {
			slog.Error("invalid status", "error", s.Error)
		}

		slog.LogAttrs(context.TODO(), slog.LevelInfo, "",
			slog.Int("ambient", s.Ambient),
			slog.Int("grill", s.Grill),
			slog.Int("grill_set", s.GrillSet),
			slog.Int("probe", s.Probe),
			slog.Int("probe_set", s.ProbeSet),
			slog.Bool("probe_alarm", s.ProbeAlarmFired))

		if w != nil {
			b, err := json.Marshal(s)
			if err != nil {
				slog.Error("cannot marshal", "error", err)
			}

			_, _ = w.Write(b)
			_, _ = w.Write([]byte("\n"))
		}
	}

}
