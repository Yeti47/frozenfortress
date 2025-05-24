package output

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Yeti47/frozenfortress/frozenfortress/core/auth"
)

// Formatter handles output formatting for the CLI
type Formatter struct {
	verbose bool
}

// NewFormatter creates a new output formatter
func NewFormatter(verbose bool) *Formatter {
	return &Formatter{
		verbose: verbose,
	}
}

// PrintSuccess prints a success message
func (f *Formatter) PrintSuccess(message string) {
	fmt.Printf("✓ %s\n", message)
}

// PrintError prints an error message
func (f *Formatter) PrintError(message string) {
	fmt.Fprintf(os.Stderr, "✗ Error: %s\n", message)
}

// PrintWarning prints a warning message
func (f *Formatter) PrintWarning(message string) {
	fmt.Printf("⚠ Warning: %s\n", message)
}

// PrintInfo prints an informational message
func (f *Formatter) PrintInfo(message string) {
	if f.verbose {
		fmt.Printf("ℹ %s\n", message)
	}
}

// PrintUser prints user information in a formatted way
func (f *Formatter) PrintUser(user *auth.UserDto) {
	if user == nil {
		f.PrintError("User not found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "User ID:\t%s\n", user.Id)
	fmt.Fprintf(w, "Username:\t%s\n", user.UserName)
	fmt.Fprintf(w, "Active:\t%t\n", user.IsActive)
	fmt.Fprintf(w, "Locked:\t%t\n", user.IsLocked)
	fmt.Fprintf(w, "Created:\t%s\n", user.CreatedAt)
	fmt.Fprintf(w, "Modified:\t%s\n", user.ModifiedAt)
	w.Flush()
}

// PrintUsers prints a list of users in a table format
func (f *Formatter) PrintUsers(users []auth.UserDto) {
	if len(users) == 0 {
		fmt.Println("No users found")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tUSERNAME\tACTIVE\tLOCKED\tCREATED\n")
	fmt.Fprintf(w, "--\t--------\t------\t------\t-------\n")

	for _, user := range users {
		activeStatus := "No"
		if user.IsActive {
			activeStatus = "Yes"
		}

		lockedStatus := "No"
		if user.IsLocked {
			lockedStatus = "Yes"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			user.Id,
			user.UserName,
			activeStatus,
			lockedStatus,
			user.CreatedAt,
		)
	}

	w.Flush()
}

// Package-level convenience functions for easier usage in commands
var defaultFormatter = NewFormatter(false)

// PrintSuccess prints a success message using the default formatter
func PrintSuccess(message string, details map[string]interface{}) {
	defaultFormatter.PrintSuccess(message)
	for key, value := range details {
		fmt.Printf("  %s: %v\n", key, value)
	}
}

// PrintError prints an error message using the default formatter
func PrintError(message string, err error) {
	if err != nil {
		defaultFormatter.PrintError(fmt.Sprintf("%s: %v", message, err))
	} else {
		defaultFormatter.PrintError(message)
	}
}

// PrintWarning prints a warning message using the default formatter
func PrintWarning(message string) {
	defaultFormatter.PrintWarning(message)
}

// PrintInfo prints an informational message using the default formatter
func PrintInfo(message string) {
	defaultFormatter.PrintInfo(message)
}

// PrintUser prints user information using the default formatter
func PrintUser(user *auth.UserDto) {
	defaultFormatter.PrintUser(user)
}
