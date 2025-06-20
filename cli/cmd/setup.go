package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"

	"slices"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/ccc"
	"github.com/spf13/cobra"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive setup for Frozen Fortress configuration",
	Long: `Interactive configuration setup for Frozen Fortress.

This command will prompt you for configuration values and create a script
that sets the appropriate environment variables. If you press Enter without
entering a value, the current/default value will be kept.

The generated script format depends on your operating system:
- Linux/macOS: Creates a shell script (frozenfortress-config.sh)
- Windows: Creates a batch file (frozenfortress-config.bat)

Usage examples:
  Linux/macOS: source frozenfortress-config.sh
  Windows:     frozenfortress-config.bat

The script can be used to set environment variables before running Frozen Fortress.

Use the --read flag to only display current configuration values without
prompting for changes or generating a script.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		readOnly, _ := cmd.Flags().GetBool("read")
		return runSetup(readOnly)
	},
}

// ConfigItem represents a configuration item with its metadata
type ConfigItem struct {
	EnvVar       string
	Description  string
	CurrentValue string
	DefaultValue string
	Type         string // "string", "int", "bool"
	Validation   func(string) (string, error)
}

func runSetup(readOnly bool) error {
	if readOnly {
		fmt.Println("=== Frozen Fortress Current Configuration ===")
		fmt.Println()
	} else {
		fmt.Println("=== Frozen Fortress Configuration Setup ===")
		fmt.Println()
		fmt.Println("This setup will help you configure environment variables for Frozen Fortress.")
		fmt.Println("Press Enter to keep the current/default value, or enter a new value.")
		fmt.Println()
	}

	// Load current configuration to show current values
	currentConfig := ccc.LoadConfigFromEnv()
	defaultConfig := ccc.DefaultConfig

	// Define all configuration items
	configItems := []ConfigItem{
		{
			EnvVar:       ccc.EnvDatabasePath,
			Description:  "Path to the SQLite database file",
			CurrentValue: currentConfig.DatabasePath,
			DefaultValue: defaultConfig.DatabasePath,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvMaxSignInAttempts,
			Description:  "Maximum number of sign-in attempts before locking account",
			CurrentValue: strconv.Itoa(currentConfig.MaxSignInAttempts),
			DefaultValue: strconv.Itoa(defaultConfig.MaxSignInAttempts),
			Type:         "int",
			Validation:   validatePositiveInt,
		},
		{
			EnvVar:       ccc.EnvSignInAttemptWindow,
			Description:  "Time window in minutes for counting sign-in attempts",
			CurrentValue: strconv.Itoa(currentConfig.SignInAttemptWindow),
			DefaultValue: strconv.Itoa(defaultConfig.SignInAttemptWindow),
			Type:         "int",
			Validation:   validatePositiveInt,
		},
		{
			EnvVar:       ccc.EnvRedisAddress,
			Description:  "Redis server address (host:port)",
			CurrentValue: currentConfig.RedisAddress,
			DefaultValue: defaultConfig.RedisAddress,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvRedisUser,
			Description:  "Redis username (leave empty if not required)",
			CurrentValue: currentConfig.RedisUser,
			DefaultValue: defaultConfig.RedisUser,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvRedisPassword,
			Description:  "Redis password (leave empty if not required)",
			CurrentValue: currentConfig.RedisPassword,
			DefaultValue: defaultConfig.RedisPassword,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvRedisSize,
			Description:  "Redis connection pool size",
			CurrentValue: strconv.Itoa(currentConfig.RedisSize),
			DefaultValue: strconv.Itoa(defaultConfig.RedisSize),
			Type:         "int",
			Validation:   validatePositiveInt,
		},
		{
			EnvVar:       ccc.EnvRedisNetwork,
			Description:  "Redis network type (tcp/unix)",
			CurrentValue: currentConfig.RedisNetwork,
			DefaultValue: defaultConfig.RedisNetwork,
			Type:         "string",
			Validation:   validateRedisNetwork,
		},
		{
			EnvVar:       ccc.EnvSigningKey,
			Description:  "Session signing key (leave empty to auto-generate)",
			CurrentValue: currentConfig.SigningKey,
			DefaultValue: defaultConfig.SigningKey,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvEncryptionKey,
			Description:  "Session encryption key (leave empty to auto-generate)",
			CurrentValue: currentConfig.EncryptionKey,
			DefaultValue: defaultConfig.EncryptionKey,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvKeyDir,
			Description:  "Directory to store persistent key files (leave empty for default)",
			CurrentValue: currentConfig.KeyDir,
			DefaultValue: defaultConfig.KeyDir,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvWebUIPort,
			Description:  "Port for the Web UI server",
			CurrentValue: strconv.Itoa(currentConfig.WebUiPort),
			DefaultValue: strconv.Itoa(defaultConfig.WebUiPort),
			Type:         "int",
			Validation:   validatePort,
		},
		{
			EnvVar:       ccc.EnvLogLevel,
			Description:  "Log level (Debug, Info, Warn, Error)",
			CurrentValue: currentConfig.LogLevel,
			DefaultValue: defaultConfig.LogLevel,
			Type:         "string",
			Validation:   validateLogLevel,
		},
		{
			EnvVar:       ccc.EnvBackupEnabled,
			Description:  "Enable automatic backups (true/false)",
			CurrentValue: strconv.FormatBool(currentConfig.Backup.Enabled),
			DefaultValue: strconv.FormatBool(defaultConfig.Backup.Enabled),
			Type:         "bool",
			Validation:   validateBool,
		},
		{
			EnvVar:       ccc.EnvBackupIntervalDays,
			Description:  "Backup interval in days (0 = disabled)",
			CurrentValue: strconv.Itoa(currentConfig.Backup.IntervalDays),
			DefaultValue: strconv.Itoa(defaultConfig.Backup.IntervalDays),
			Type:         "int",
			Validation:   validateNonNegativeInt,
		},
		{
			EnvVar:       ccc.EnvBackupDirectory,
			Description:  "Directory where backup files are stored",
			CurrentValue: currentConfig.Backup.Directory,
			DefaultValue: defaultConfig.Backup.Directory,
			Type:         "string",
		},
		{
			EnvVar:       ccc.EnvBackupMaxGenerations,
			Description:  "Maximum number of backup files to keep",
			CurrentValue: strconv.Itoa(currentConfig.Backup.MaxGenerations),
			DefaultValue: strconv.Itoa(defaultConfig.Backup.MaxGenerations),
			Type:         "int",
			Validation:   validatePositiveInt,
		},
		{
			EnvVar:       ccc.EnvOCRLanguages,
			Description:  "OCR languages (comma-separated, e.g., 'eng,deu' for English and German)",
			CurrentValue: strings.Join(currentConfig.OCR.Languages, ","),
			DefaultValue: strings.Join(defaultConfig.OCR.Languages, ","),
			Type:         "string",
			Validation:   validateOCRLanguages,
		},
	}

	// Collect user input for each configuration item or just display them
	envVars := make(map[string]string)
	scanner := bufio.NewScanner(os.Stdin)

	for _, item := range configItems {
		if readOnly {
			// Just display the current value
			displayConfigItem(item)
		} else {
			// Prompt for new value
			value, err := promptForValue(scanner, item)
			if err != nil {
				return fmt.Errorf("error reading input for %s: %w", item.EnvVar, err)
			}

			// Only set environment variable if value is different from default
			// or if there's already a value set (to preserve explicit empty values)
			if value != item.DefaultValue || os.Getenv(item.EnvVar) != "" {
				envVars[item.EnvVar] = value
			}
		}
	}

	// Skip script generation in read-only mode
	if readOnly {
		fmt.Println()
		fmt.Println("Configuration display complete.")
		return nil
	}

	// Generate configuration script
	scriptPath, err := generateConfigScript(envVars)
	if err != nil {
		return fmt.Errorf("failed to generate configuration script: %w", err)
	}

	// Display completion message
	output.PrintSuccess("Configuration setup completed!", map[string]interface{}{
		"script_path":   scriptPath,
		"variables_set": len(envVars),
	})

	fmt.Println()
	fmt.Println("To apply the configuration:")

	switch runtime.GOOS {
	case "windows":
		fmt.Printf("  %s\n", scriptPath)
		fmt.Println()
		fmt.Println("To make it permanent, you can:")
		fmt.Println("  1. Run the batch file before starting Frozen Fortress")
		fmt.Printf("  2. Add the 'set' commands from %s to your environment variables\n", scriptPath)
		fmt.Println("  3. Use 'setx' commands for permanent system environment variables")
	default:
		fmt.Printf("  source %s\n", scriptPath)
		fmt.Println()
		fmt.Println("To make it permanent, add the exports to your shell profile:")
		fmt.Printf("  cat %s >> ~/.bashrc  # For bash\n", scriptPath)
		fmt.Printf("  cat %s >> ~/.zshrc   # For zsh\n", scriptPath)
	}

	return nil
}

func promptForValue(scanner *bufio.Scanner, item ConfigItem) (string, error) {
	// Show current value
	currentDisplay := item.CurrentValue
	if currentDisplay == "" {
		currentDisplay = "(empty)"
	}

	fmt.Printf("%s:\n", item.Description)
	fmt.Printf("  Current: %s\n", currentDisplay)
	fmt.Printf("  %s: ", item.EnvVar)

	// Read user input
	if !scanner.Scan() {
		return "", scanner.Err()
	}

	input := strings.TrimSpace(scanner.Text())

	// If no input provided, keep current value
	if input == "" {
		return item.CurrentValue, nil
	}

	// Validate input if validation function is provided
	if item.Validation != nil {
		validated, err := item.Validation(input)
		if err != nil {
			fmt.Printf("  Error: %s\n", err.Error())
			fmt.Printf("  Please try again.\n")
			return promptForValue(scanner, item)
		}
		input = validated
	}

	fmt.Println()
	return input, nil
}

func displayConfigItem(item ConfigItem) {
	// Show current value
	currentDisplay := item.CurrentValue
	if currentDisplay == "" {
		currentDisplay = "(empty)"
	}

	// Check if the value is set via environment variable
	envValue := os.Getenv(item.EnvVar)
	var source string
	if envValue != "" {
		if envValue == item.DefaultValue {
			source = "environment (default)"
		} else {
			source = "environment (custom)"
		}
	} else {
		source = "default"
	}

	fmt.Printf("%s:\n", item.Description)
	fmt.Printf("  %s: %s (%s)\n", item.EnvVar, currentDisplay, source)
	fmt.Println()
}

func generateConfigScript(envVars map[string]string) (string, error) {
	var scriptPath string
	var scriptContent strings.Builder

	// Determine script type and path based on operating system
	switch runtime.GOOS {
	case "windows":
		scriptPath = "frozenfortress-config.bat"
		scriptContent.WriteString("@echo off\n")
		scriptContent.WriteString("REM Frozen Fortress Configuration\n")
		scriptContent.WriteString("REM Generated by: ffcli setup\n")
		scriptContent.WriteString("\n")

		// Write environment variables for Windows batch
		for envVar, value := range envVars {
			// Escape value for batch file
			escapedValue := strings.ReplaceAll(value, `"`, `""`)
			scriptContent.WriteString(fmt.Sprintf("set %s=%s\n", envVar, escapedValue))
		}
	default:
		// Unix-like systems (Linux, macOS, etc.)
		scriptPath = "frozenfortress-config.sh"
		scriptContent.WriteString("#!/bin/bash\n")
		scriptContent.WriteString("# Frozen Fortress Configuration\n")
		scriptContent.WriteString("# Generated by: ffcli setup\n")
		scriptContent.WriteString("\n")

		// Write environment variables for bash
		for envVar, value := range envVars {
			// Escape value for shell
			escapedValue := strings.ReplaceAll(value, `"`, `\"`)
			scriptContent.WriteString(fmt.Sprintf("export %s=\"%s\"\n", envVar, escapedValue))
		}
	}

	// Write the script file
	file, err := os.Create(scriptPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := file.WriteString(scriptContent.String()); err != nil {
		return "", err
	}

	// Make script executable on Unix-like systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(scriptPath, 0755); err != nil {
			return "", err
		}
	}

	return scriptPath, nil
}

// Validation functions
func validatePositiveInt(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	num, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("must be a valid integer")
	}

	if num <= 0 {
		return "", fmt.Errorf("must be a positive integer")
	}

	return value, nil
}

func validateNonNegativeInt(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	num, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("must be a valid integer")
	}

	if num < 0 {
		return "", fmt.Errorf("must be a non-negative integer")
	}

	return value, nil
}

func validatePort(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	port, err := strconv.Atoi(value)
	if err != nil {
		return "", fmt.Errorf("must be a valid integer")
	}

	if port <= 0 || port > 65535 {
		return "", fmt.Errorf("must be a valid port number (1-65535)")
	}

	return value, nil
}

func validateBool(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	lower := strings.ToLower(value)
	if lower != "true" && lower != "false" {
		return "", fmt.Errorf("must be 'true' or 'false'")
	}

	return lower, nil
}

func validateLogLevel(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	validLevels := []string{"Debug", "Info", "Warn", "Error"}
	for _, level := range validLevels {
		if strings.EqualFold(value, level) {
			return level, nil // Return with proper capitalization
		}
	}

	return "", fmt.Errorf("must be one of: %s", strings.Join(validLevels, ", "))
}

func validateRedisNetwork(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	lower := strings.ToLower(value)
	if lower != "tcp" && lower != "unix" {
		return "", fmt.Errorf("must be 'tcp' or 'unix'")
	}

	return lower, nil
}

func validateOCRLanguages(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("value cannot be empty")
	}

	// Split by comma and validate each language code
	languages := strings.Split(value, ",")
	var validatedLanguages []string

	for _, lang := range languages {
		lang = strings.TrimSpace(lang)
		if lang == "" {
			continue // Skip empty entries
		}

		// Basic validation: language codes should be 3 characters long
		if len(lang) != 3 {
			return "", fmt.Errorf("language code '%s' must be exactly 3 characters (e.g., 'eng', 'deu')", lang)
		}

		// Convert to lowercase for consistency
		lang = strings.ToLower(lang)

		// Check for common language codes (basic validation)
		validLanguages := []string{
			"eng", "deu", "fra", "spa", "ita", "por", "rus", "jpn", "chi_sim", "chi_tra",
			"ara", "hin", "kor", "tha", "vie", "nld", "swe", "dan", "nor", "fin",
		}

		isValid := slices.Contains(validLanguages, lang)

		if !isValid {
			fmt.Printf("  Warning: '%s' is not a commonly recognized language code.\n", lang)
			fmt.Printf("  Common codes: eng (English), deu (German), fra (French), spa (Spanish), etc.\n")
			fmt.Printf("  The language will be included but may not work if not installed.\n")
		}

		validatedLanguages = append(validatedLanguages, lang)
	}

	if len(validatedLanguages) == 0 {
		return "", fmt.Errorf("at least one language code must be provided")
	}

	return strings.Join(validatedLanguages, ","), nil
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().Bool("read", false, "display current configuration values without prompting for changes")
}
