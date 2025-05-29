package cmd

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// Package-level logger
var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// verbose holds the value of the --verbose flag
var verbose bool

// appConfig returns a singleton instance of the application configuration
var appConfig = func() func() (ccc.AppConfig, error) {
	var instance ccc.AppConfig
	var once sync.Once

	return func() (ccc.AppConfig, error) {
		once.Do(func() {
			instance = ccc.LoadConfigFromEnv()
		})
		return instance, nil
	}
}()

// database returns a singleton instance of the database connection
var database = func() func() (*sql.DB, error) {
	var instance *sql.DB
	var once sync.Once

	return func() (*sql.DB, error) {
		var err error
		once.Do(func() {
			var cfg ccc.AppConfig
			cfg, err = appConfig()
			if err != nil {
				return
			}
			instance, err = ccc.SetupDatabase(cfg)
		})
		return instance, err
	}
}()

// cleanupResources closes the database connection if it exists
var cleanupResources = func() func() error {
	return func() error {
		db, err := database()
		if err != nil {
			// If we can't get the database, there's nothing to clean up
			// or the database was already closed by a previous cleanupResources call
			return nil
		}
		// Check if the database connection is still valid before trying to close it
		if errPing := db.Ping(); errPing == nil {
			return db.Close()
		}
		return nil // Connection already closed or invalid
	}
}()

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ffcli",
	Short: "FrozenFortress CLI - Admin tool for user management",
	Long: `FrozenFortress CLI is an administrative command-line tool for managing users
in the FrozenFortress password manager system.

This tool is designed for server administrators with direct access to the database
and does not require authentication as it assumes privileged access.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize configuration (this will be cached via singleton)
		cfg, err := appConfig()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		if verbose {
			fmt.Printf("Using configuration: %s\\n", cfg.String())
		}

		// Initialize database (this will be cached via singleton)
		_, err = database()
		if err != nil {
			return fmt.Errorf("failed to connect to database: %w", err)
		}

		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// Clean up database connection
		return cleanupResources()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		// Check if it's an ApiError and exit with appropriate code
		if apiErr, ok := ccc.IsApiError(err); ok {
			fmt.Fprintf(os.Stderr, "Error: %s\n", apiErr.UserMessage)
			if verbose {
				fmt.Fprintf(os.Stderr, "Technical details: %s\n", apiErr.TechnicalMessage)
			}
			os.Exit(int(internal.ExitCodeFromApiError(apiErr)))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(internal.ExitInternalError))
		}
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}
