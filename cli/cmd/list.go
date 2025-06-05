package cmd

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all user accounts",
	Long: `List all user accounts in the FrozenFortress system.

This command displays all users in a table format showing:
- User ID
- Username  
- Active status
- Locked status
- Created date

Examples:
  frozen-fortress user list`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get user manager
		userMgr, err := userManager()
		if err != nil {
			return err
		}

		// Get all users
		users, err := userMgr.GetAllUsers()
		if err != nil {
			return err
		}

		// Print users using the formatter
		output.PrintUsers(users)

		return nil
	},
}

func init() {
	userCmd.AddCommand(listCmd)
}
