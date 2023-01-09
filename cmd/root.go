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

func newRootCmd() *cobra.Command {
	var (
		username, password string
		logLevel           string
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
			w, err := wifire.New(wifire.Credentials(username, password),
				wifire.Logging(log.Logger, zerolog.GlobalLevel()))
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

			if err := g.Status(statusHandler); err != nil {
				panic(err)
			}

			catch := make(chan os.Signal, 1)
			signal.Notify(catch, syscall.SIGINT, syscall.SIGTERM)
			<-catch

			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&logLevel, "log", zerolog.LevelInfoValue, "log level")
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

func statusHandler(s wifire.Status) {
	if s.Error != nil {
		log.Err(s.Error).Msg("invalid status")
		return
	}

	log.Info().
		Int("ambient", s.Ambient).
		Int("grill", s.Grill).
		Int("probe", s.Probe).
		Send()
}
