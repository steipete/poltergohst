// Package cli provides the command-line interface for Poltergeist
package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// CLI encapsulates the command-line interface and makes it testable
// by eliminating global state. This follows Go best practices.
type CLI struct {
	config   *Config
	rootCmd  *cobra.Command
	logger   logger.Logger
	output   io.Writer
	errorOut io.Writer
}

// NewCLI creates a new CLI instance with the given configuration
func NewCLI(config *Config) *CLI {
	if config == nil {
		config = NewConfig()
	}

	cli := &CLI{
		config:   config,
		output:   os.Stdout,
		errorOut: os.Stderr,
	}

	cli.setupCommands()
	return cli
}

// NewCLIWithOutput creates a CLI with custom output writers (for testing)
func NewCLIWithOutput(config *Config, output, errorOut io.Writer) *CLI {
	cli := NewCLI(config)
	cli.output = output
	cli.errorOut = errorOut
	return cli
}

// Execute runs the CLI with the given arguments
func (c *CLI) Execute(args []string) error {
	c.rootCmd.SetArgs(args)
	return c.rootCmd.Execute()
}

// ExecuteContext runs the CLI with context support
func (c *CLI) ExecuteContext(ctx context.Context, args []string) error {
	c.rootCmd.SetArgs(args)
	return c.rootCmd.ExecuteContext(ctx)
}

func (c *CLI) setupCommands() {
	c.rootCmd = &cobra.Command{
		Use:   "poltergeist",
		Short: "The invisible build system that haunts your code",
		Long: `👻 Poltergeist - Automatic incremental builds powered by file watching
		
Poltergeist watches your project files and automatically rebuilds targets when
changes are detected. It's like having a helpful ghost that builds your code
before you even ask!`,

		PersistentPreRunE: c.initializeConfig,
		Run: func(cmd *cobra.Command, args []string) {
			// If no subcommand, show help
			cmd.Help()
		},
	}

	// Setup flags
	c.setupFlags()

	// Setup version
	c.rootCmd.Version = c.config.Version
	c.rootCmd.SetVersionTemplate("👻 Poltergeist v{{.Version}}\n")

	// Add subcommands
	c.rootCmd.AddCommand(c.newWatchCmd())
	c.rootCmd.AddCommand(c.newInitCmd())
	c.rootCmd.AddCommand(c.newStatusCmd())
	c.rootCmd.AddCommand(c.newListCmd())
	c.rootCmd.AddCommand(c.newBuildCmd())
	c.rootCmd.AddCommand(c.newCleanCmd())
	c.rootCmd.AddCommand(c.newDaemonCmd())
	c.rootCmd.AddCommand(c.newLogsCmd())
	c.rootCmd.AddCommand(c.newValidateCmd())
}

func (c *CLI) setupFlags() {
	flags := c.rootCmd.PersistentFlags()

	// Global flags - bind to config struct
	flags.StringVar(&c.config.ConfigFile, "config", "", "config file (default: poltergeist.config.json)")
	flags.StringVar(&c.config.ProjectRoot, "root", ".", "project root directory")
	flags.StringVarP(&c.config.Verbosity, "verbosity", "v", "info", "log level (debug, info, warn, error)")
}

func (c *CLI) initializeConfig(cmd *cobra.Command, args []string) error {
	// Create logger with configured verbosity
	c.logger = logger.CreateLogger("", c.config.Verbosity)

	if c.config.ConfigFile != "" {
		// Use config file from flag
		viper.SetConfigFile(c.config.ConfigFile)
	} else {
		// Search for config in project root
		viper.AddConfigPath(c.config.ProjectRoot)
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
		// Use structured logging instead of fmt.Println
		if c.config.Verbosity == "debug" {
			c.logger.Debug("Using config file",
				logger.WithField("file", viper.ConfigFileUsed()))
		}
	}

	return nil
}

// Helper methods for structured output

func (c *CLI) printSuccess(message string) {
	c.logger.Success(message)
}

func (c *CLI) printError(message string) {
	c.logger.Error(message)
}

func (c *CLI) printInfo(message string) {
	c.logger.Info(message)
}

func (c *CLI) printWarning(message string) {
	c.logger.Warn(message)
}

func (c *CLI) getConfigPath() string {
	if c.config.ConfigFile != "" {
		return c.config.ConfigFile
	}
	return filepath.Join(c.config.ProjectRoot, "poltergeist.config.json")
}

// Placeholder command methods - these would be implemented similarly

func (c *CLI) newWatchCmd() *cobra.Command {
	// Implementation would follow similar pattern to existing newWatchCmd
	// but using c.config instead of global variables
	return &cobra.Command{
		Use:   "watch",
		Short: "Start watching files and building targets",
	}
}

func (c *CLI) newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a new Poltergeist configuration",
	}
}

func (c *CLI) newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Poltergeist status",
	}
}

func (c *CLI) newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured targets",
	}
}

func (c *CLI) newBuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build specific targets",
	}
}

func (c *CLI) newCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean build artifacts",
	}
}

func (c *CLI) newDaemonCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Poltergeist daemon",
	}
}

func (c *CLI) newLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs",
		Short: "Display build logs",
	}
}

func (c *CLI) newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration",
	}
}

// ExecuteWithVersion is a compatibility wrapper for the existing API
func ExecuteWithVersion(version string) error {
	config := NewConfig()
	config.Version = version
	cli := NewCLI(config)
	return cli.Execute(os.Args[1:])
}
