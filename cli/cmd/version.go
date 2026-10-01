package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/updates"
	"github.com/spf13/cobra"
)

// updateCheckTimeout bounds the whole update check, including the GitHub request
const updateCheckTimeout = 15 * time.Second

// checkForUpdates holds the value of the --check flag
var checkForUpdates bool

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of the application",
	Long: `Print the current version of the Frozen Fortress application.

Use --check to look up the latest release on GitHub and report whether a newer
version is available. This makes one anonymous request to api.github.com and
is independent of FF_UPDATE_CHECK_ENABLED, which only controls the web UI's
automatic check.`,
	// The version command needs neither configuration nor a database,
	// so override the root command's database setup and cleanup.
	PersistentPreRunE:  func(cmd *cobra.Command, args []string) error { return nil },
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error { return nil },
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ccc.AppVersion)

		if !checkForUpdates {
			return nil
		}

		if !updates.IsReleaseVersion(ccc.AppVersion) {
			fmt.Printf("ℹ Update check is only available for release builds (current version: %s)\n", ccc.AppVersion)
			return nil
		}

		// Arguments are valid at this point; a failed check is not a usage error
		cmd.SilenceUsage = true

		ctx, cancel := context.WithTimeout(cmd.Context(), updateCheckTimeout)
		defer cancel()

		checker := updates.NewGitHubUpdateChecker(updates.DefaultGitHubReleasesURL, ccc.AppVersion, logger)
		release, err := checker.Check(ctx)
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		if release == nil {
			output.PrintSuccess("You are running the latest version", nil)
			return nil
		}

		fmt.Printf("ℹ A new version is available: %s\n", release.Version)
		fmt.Printf("  %s\n", release.URL)
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVarP(&checkForUpdates, "check", "c", false, "check GitHub for a newer release")
	rootCmd.AddCommand(versionCmd)
}
