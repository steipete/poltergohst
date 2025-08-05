// Package cli provides the command-line interface for Poltergeist
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile     string
	projectRoot string
	verbosity   string
	version     string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "poltergeist",
	Short: "The invisible build system that haunts your code",
	Long: `👻 Poltergeist - Automatic incremental builds powered by file watching
	
Poltergeist watches your project files and automatically rebuilds targets when
changes are detected. It's like having a helpful ghost that builds your code
before you even ask!`,
	
	Run: func(cmd *cobra.Command, args []string) {
		// Check if version flag is set
		if v, _ := cmd.Flags().GetBool("version"); v {
			fmt.Printf("👻 Poltergeist v%s\n", version)
			return
		}
		// If no subcommand, show help
		cmd.Help()
	},
}

// Execute runs the CLI
func Execute(v string) error {
	version = v
	
	// Initialize the root command explicitly (avoiding init())
	initializeRootCommand()
	
	return rootCmd.Execute()
}

// initializeRootCommand sets up the root command and its flags.
// This replaces the init() function to make initialization explicit and testable.
func initializeRootCommand() {
	// Set up config initialization
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: poltergeist.config.json)")
	rootCmd.PersistentFlags().StringVar(&projectRoot, "root", ".", "project root directory")
	rootCmd.PersistentFlags().StringVarP(&verbosity, "verbosity", "v", "info", "log level (debug, info, warn, error)")
	
	// Add version flag
	rootCmd.Flags().Bool("version", false, "Print version information and quit")
	
	// Add subcommands
	rootCmd.AddCommand(newWatchCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newBuildCmd())
	rootCmd.AddCommand(newCleanCmd())
	rootCmd.AddCommand(newDaemonCmd())
	rootCmd.AddCommand(newLogsCmd())
	rootCmd.AddCommand(newValidateCmd())
	rootCmd.AddCommand(newVersionCmd())
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in project root
		viper.AddConfigPath(projectRoot)
		viper.SetConfigName("poltergeist.config")
		viper.SetConfigType("json")
		
		// Also try YAML
		viper.SetConfigName("poltergeist.config")
		viper.SetConfigType("yaml")
	}

	// Read in environment variables
	viper.SetEnvPrefix("POLTERGEIST")
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err == nil {
		if verbosity == "debug" {
			fmt.Println("Using config file:", viper.ConfigFileUsed())
		}
	}
}

// Helper functions

func printSuccess(message string) {
	ghost := "👻"
	fmt.Printf("%s %s %s\n", ghost, color.GreenString("[Poltergeist]"), message)
}

func printError(message string) {
	ghost := "👻"
	fmt.Fprintf(os.Stderr, "%s %s %s\n", ghost, color.RedString("[Poltergeist]"), message)
}

func printInfo(message string) {
	ghost := "👻"
	fmt.Printf("%s %s %s\n", ghost, color.CyanString("[Poltergeist]"), message)
}

func printWarning(message string) {
	ghost := "👻"
	fmt.Printf("%s %s %s\n", ghost, color.YellowString("[Poltergeist]"), message)
}

func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	return filepath.Join(projectRoot, "poltergeist.config.json")
}