package main

import (
	"bufio"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"

	"endobit.io/app"
	"endobit.io/app/log"
	"endobit.io/wifire"
)

type Config struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"` //nolint:gosec
}

func newRootCmd() *cobra.Command { //nolint:gocognit
	var (
		logFile *os.File
		logOpts *log.Options
		output  string
		v       *viper.Viper //nolint:varnamelen
	)

	cmd := cobra.Command{
		Use:     "wifire",
		Short:   "Traeger WiFire Grill Util",
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := loadDotEnv(); err != nil {
				return err
			}

			var err error

			if logOpts.Filename != "" {
				logFile, err = os.OpenFile(logOpts.Filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
				if err != nil {
					return err
				}

				logOpts.Writer = logFile
			}

			logger, err := log.New(logOpts)
			if err != nil {
				return err
			}

			cmd.SetContext(log.WithLogger(cmd.Context(), logger))
			logger.Info("starting", "version", version)

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer func() {
				if logFile != nil {
					_ = logFile.Close()
				}
			}()

			var cfg Config

			logger := log.FromContext(cmd.Context())

			if err := v.ReadInConfig(); err != nil {
				var notFound viper.ConfigFileNotFoundError
				if !errors.As(err, &notFound) {
					slog.Warn("failed to read config file", "error", err)
				}
			}

			if err := v.Unmarshal(&cfg); err != nil {
				return err
			}

			// Unmarshal uses AllSettings(), which omits keys that exist only in the
			// environment (e.g. WIFIRE_* from Docker env_file). Merge via Get.
			cfg.Username = v.GetString("username")
			cfg.Password = v.GetString("password")

			// Flags override config file and environment when set (empty flags must not
			// wipe WIFIRE_* from the environment or .env).
			if u, err := cmd.Flags().GetString("username"); err == nil && u != "" {
				cfg.Username = u
			}

			if p, err := cmd.Flags().GetString("password"); err == nil && p != "" {
				cfg.Password = p
			}

			if cfg.Username == "" || cfg.Password == "" {
				return errors.New("username and password must be set via flags, config file, .env, or environment (WIFIRE_USERNAME, WIFIRE_PASSWORD)")
			}

			grill, err := connectToGrill(cfg.Username, cfg.Password, logger)
			if err != nil {
				return fmt.Errorf("failed to connect to grill: %w", err)
			}

			userData, err := grill.UserData()
			if err != nil {
				return err
			}

			if len(userData.Things) == 0 {
				return errors.New("no grills found for this account")
			}

			if len(userData.Things) > 1 { // TODO: what to decide which grill to use?
				logger.Warn("multiple grills found, using the first one")
			}

			thing := userData.Things[0]
			logger.Info("found", "grill", thing.FriendlyName, "model", thing.GrillModel.Name)

			// Load historical data from file on startup for better ETA stability
			history := []status{}

			if output != "" {
				loadedHistory, err := loadHistoricalData(output, 20)
				if err != nil {
					logger.Warn("failed to load historical data", "error", err)
				} else if loadedHistory != nil {
					history = loadedHistory
				}
			}

			// Log startup information if we have historical data
			if len(history) > 0 {
				lastStatus := history[len(history)-1]
				logger.Info("loaded historical data for startup ETA stability",
					"entries", len(history),
					"last_ambient", lastStatus.Ambient,
					"last_grill", lastStatus.Grill,
					"last_probe", lastStatus.Probe)
			}

			m := monitor{
				Logger:  logger,
				Grill:   grill,
				History: history,
			}

			if output != "" {
				f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
				if err != nil {
					return err
				}

				defer f.Close()

				m.Output = f
			}

			return m.Run(cmd.Context(), thing.ThingName)
		},
	}

	path, err := app.UserConfigFilePath(cmd.Use)
	if err != nil {
		panic(err)
	}

	v = viper.New()
	v.SetEnvPrefix("WIFIRE")
	v.AutomaticEnv()
	v.AddConfigPath(path)
	v.SetConfigName("config")

	logOpts = log.NewOptions(cmd.PersistentFlags())

	cmd.Flags().StringVarP(&output, "output", "o", "", "log grill data to file")
	cmd.Flags().String("username", "", "account username (overrides WIFIRE_USERNAME / config)")
	cmd.Flags().String("password", "", "account password (overrides WIFIRE_PASSWORD / config)")

	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newPlotCmd())
	cmd.AddCommand(newForecastCmd())

	return &cmd
}

// loadDotEnv loads a .env file into the process environment so viper picks up
// WIFIRE_USERNAME and WIFIRE_PASSWORD. If WIFIRE_DOTENV is set, that path is
// loaded (must exist). Otherwise ".env" in the current directory is loaded when
// present.
func loadDotEnv() error {
	path := os.Getenv("WIFIRE_DOTENV")
	if path == "" {
		if _, err := os.Stat(".env"); err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return fmt.Errorf(".env: %w", err)
		}

		path = ".env"
	}

	if err := gotenv.Load(path); err != nil {
		return fmt.Errorf("load %s: %w", path, err)
	}

	return nil
}

var grillRegexp = regexp.MustCompile(`^\[([^\]]+)\]\s+(.+)$`)

func connectToGrill(username, password string, logger *slog.Logger) (*wifire.Client, error) {
	// When messages look like "[component] message", split out the component
	// and do a little bit of structured logging.
	filter := func(msg string) (string, []slog.Attr) {
		matches := grillRegexp.FindStringSubmatch(msg)
		if len(matches) == 3 {
			return matches[2], []slog.Attr{slog.String("component", matches[1])}
		}

		return msg, nil
	}

	legacy := func(level slog.Level) log.Legacy {
		return log.NewLegacy(logger, log.WithLevel(level), log.WithFilter(filter))
	}

	// wire in legacy logger for MQTT messages
	mqtt.CRITICAL = legacy(slog.LevelError)
	mqtt.ERROR = legacy(slog.LevelError)
	mqtt.WARN = legacy(slog.LevelWarn)
	mqtt.DEBUG = legacy(slog.LevelDebug)

	client, err := wifire.NewClient(
		wifire.WithLogger(logger),
		wifire.Credentials(username, password), // basic auth into cognito
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// loadHistoricalData reads existing JSON data from the output file to initialize history
func loadHistoricalData(filename string, maxEntries int) ([]status, error) {
	if filename == "" {
		return nil, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		// File doesn't exist yet - not an error for new files
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	var history []status

	scanner := bufio.NewScanner(file)

	// Read all lines first to get the most recent entries
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Parse the most recent entries (up to maxEntries)
	startIdx := 0
	if len(lines) > maxEntries {
		startIdx = len(lines) - maxEntries
	}

	for i := startIdx; i < len(lines); i++ {
		var status status

		if err := json.Unmarshal([]byte(lines[i]), &status); err != nil {
			// Skip invalid lines but continue processing
			slog.Warn("skipping invalid JSON line in history file", "line", i+1, "error", err) //nolint:gosec

			continue
		}

		// Only include entries with valid probe data and recent timestamps (last 2 hours)
		if status.Time.After(time.Now().Add(-2*time.Hour)) && status.Probe > 0 {
			history = append(history, status)
		}
	}

	if len(history) > 0 {
		slog.Info("loaded historical data for startup", "entries", len(history),
			"oldest", history[0].Time.Format("15:04:05"),
			"newest", history[len(history)-1].Time.Format("15:04:05"))
	}

	return history, nil
}
