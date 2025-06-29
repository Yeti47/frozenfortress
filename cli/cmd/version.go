package cmd

import (
	"fmt"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of the application",
	Long:  `Print the current version of the Frozen Fortress application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(ccc.AppVersion)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
