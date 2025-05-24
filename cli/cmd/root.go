package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal"
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/config"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// appConfig returns a singleton instance of the application configuration
var appConfig = func() func() (*config.Config, error) {
	var instance *config.Config
	var once sync.Once

	return func() (*config.Config, error) {
		var err error
		once.Do(func() {
			instance, err = config.LoadFromFile("")
			if err != nil {
				return
			}
			err = instance.Validate()
		})
		return instance, err
	}
}()

// database returns a singleton instance of the database connection
var database = func() func() (*sql.DB, error) {
	var instance *sql.DB
	var once sync.Once

	return func() (*sql.DB, error) {
		var err error
		once.Do(func() {
			var cfg *config.Config
			cfg, err = appConfig()
			if err != nil {
				return
			}
			instance, err = config.SetupDatabase(cfg)
		})
		return instance, err
	}
}()

// cleanupResources closes the database connection if it exists
var cleanupResources = func() func() error {
	return func() error {
		db, err := database()
		if err != nil {
			return nil // If we can't get the database, there's nothing to clean up
		}
		return db.Close()
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

		if cfg.Verbose {
			fmt.Printf("Using configuration: %s\n", cfg.String())
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
			if cfg, cfgErr := appConfig(); cfgErr == nil && cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Technical details: %s\n", apiErr.TechnicalMessage)
			}
			os.Exit(int(internal.ExitCodeFromApiError(apiErr)))
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(int(internal.ExitInternalError))
		}
	}
}
