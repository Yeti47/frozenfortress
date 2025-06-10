package cmd

import (
	"fmt"
	"sync"

	"github.com/Yeti47/frozenfortress/frozenfortress/cli/internal/output"
	"github.com/Yeti47/frozenfortress/frozenfortress/core/backup"
	"github.com/spf13/cobra"
)

// backupService returns a singleton instance of the BackupService
var backupService = func() func() (backup.BackupService, error) {
	var instance backup.BackupService
	var once sync.Once
	var initErr error

	return func() (backup.BackupService, error) {
		once.Do(func() {
			config, err := appConfig()
			if err != nil {
				initErr = err
				return
			}

			instance = backup.NewFileBasedBackupService(config, logger)
		})
		return instance, initErr
	}
}()

// backupCmd represents the backup command group
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup management commands",
	Long:  `Commands for managing database backups in the FrozenFortress system.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// backupCreateCmd represents the command to create a manual backup
var backupCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a manual backup",
	Long:  `Creates a manual backup of the database immediately.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backupSvc, err := backupService()
		if err != nil {
			return fmt.Errorf("failed to initialize backup service: %w", err)
		}

		backupInfo, err := backupSvc.CreateBackup(backup.BackupTriggerManual)
		if err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}

		// Print success message
		output.PrintSuccess("Backup created successfully", map[string]interface{}{
			"filename":   backupInfo.Filename,
			"size_bytes": backupInfo.SizeBytes,
			"path":       backupInfo.FilePath,
		})

		return nil
	},
}

// backupListCmd represents the command to list all backups
var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all backups",
	Long:  `Lists all available backups with details.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backupSvc, err := backupService()
		if err != nil {
			return fmt.Errorf("failed to initialize backup service: %w", err)
		}

		backups, err := backupSvc.ListBackups()
		if err != nil {
			return fmt.Errorf("failed to list backups: %w", err)
		}

		if len(backups) == 0 {
			fmt.Println("No backups found")
			return nil
		}

		// Print backups in a table format
		formatter := output.NewFormatter(verbose)
		formatter.PrintBackups(backups)

		return nil
	},
}

// backupDeleteCmd represents the command to delete a specific backup
var backupDeleteCmd = &cobra.Command{
	Use:   "delete <filename>",
	Short: "Delete a specific backup",
	Long:  `Deletes a specific backup file by filename.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		backupSvc, err := backupService()
		if err != nil {
			return fmt.Errorf("failed to initialize backup service: %w", err)
		}

		if err := backupSvc.DeleteBackup(filename); err != nil {
			return fmt.Errorf("failed to delete backup: %w", err)
		}

		// Print success message
		output.PrintSuccess("Backup deleted successfully", map[string]interface{}{
			"filename": filename,
		})

		return nil
	},
}

// backupCleanupCmd represents the command to cleanup old backups
var backupCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old backups",
	Long:  `Removes old backup files according to the MaxGenerations configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		backupSvc, err := backupService()
		if err != nil {
			return fmt.Errorf("failed to initialize backup service: %w", err)
		}

		if err := backupSvc.CleanupOldBackups(); err != nil {
			return fmt.Errorf("failed to cleanup backups: %w", err)
		}

		output.PrintSuccess("Backup cleanup completed", nil)

		return nil
	},
}

func init() {
	backupCmd.AddCommand(backupCreateCmd)
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupDeleteCmd)
	backupCmd.AddCommand(backupCleanupCmd)

	rootCmd.AddCommand(backupCmd)
}
