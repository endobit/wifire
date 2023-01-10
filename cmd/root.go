package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/endobit/wifire.git"
)

func logger(level wifire.LogLevel, component, msg string) {
	var e *zerolog.Event

	switch level {
	case wifire.LogDebug:
		e = log.Debug()
	case wifire.LogInfo:
		e = log.Info()
	case wifire.LogWarn:
		e = log.Warn()
	case wifire.LogError:
		e = log.Error()
	default:
		return
	}

	if component != "" {
		e = e.Str("component", component)
	}

	e.Msg(msg)
}

func newRootCmd() *cobra.Command {
	var (
		username, password string
		logLevel           string
		debug              bool
	)

	cmd := cobra.Command{
		Use:     "wifire",
		Short:   "Traeger WiFire Grill Util",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			level, err := zerolog.ParseLevel(logLevel)
			if err != nil {
				return fmt.Errorf("invalid log level %q", logLevel)
			}

			zerolog.SetGlobalLevel(level)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug {
				wifire.Logger = logger
			}

			w, err := wifire.New(wifire.Credentials(username, password))
			if err != nil {
				panic(err)
			}

			data, err := w.UserData()
			if err != nil {
				panic(err)
			}

			g := w.NewGrill(data.Things[0].Name)
			if err := g.Connect(); err != nil {
				panic(err)
			}

			defer g.Disconnect()

			go status(g)

			catch := make(chan os.Signal, 1)
			signal.Notify(catch, syscall.SIGINT, syscall.SIGTERM)
			<-catch

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&logLevel, "log", zerolog.LevelInfoValue, "log level")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "debug wifire API")
	cmd.Flags().StringVar(&username, "username", "", "account username")
	cmd.Flags().StringVar(&password, "password", "", "account password")

	if err := cmd.MarkFlagRequired("username"); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired("password"); err != nil {
		panic(err)
	}

	cmd.AddCommand(newVersionCmd())

	return &cmd
}

func status(g *wifire.Grill) {
	ch := make(chan wifire.Status, 1)

	if err := g.SubscribeStatus(ch); err != nil {
		log.Err(err).Msg("cannot subscribe to status")
		return
	}

	for {
		s := <-ch
		if s.Error != nil {
			log.Err(s.Error).Msg("invalid status")
		}

		log.Info().
			Int("ambient", s.Ambient).
			Int("grill", s.Grill).
			Int("probe", s.Probe).
			Interface("data", s).
			Send()
	}

}
