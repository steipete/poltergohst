package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/poltergeist/poltergeist/pkg/logger"
	"github.com/poltergeist/poltergeist/pkg/poltergeist"
	"github.com/poltergeist/poltergeist/pkg/types"
	"github.com/spf13/cobra"
)

func newWatchCmd() *cobra.Command {
	var targetName string
	var daemon bool
	
	cmd := &cobra.Command{
		Use:   "watch [target]",
		Short: "Start watching files and building targets",
		Long: `Start Poltergeist in watch mode. It will monitor your files and automatically
rebuild targets when changes are detected.

If a target name is provided, only that target will be watched.
Otherwise, all enabled targets will be watched.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				targetName = args[0]
			}
			
			return runWatch(targetName, daemon)
		},
	}
	
	cmd.Flags().BoolVar(&daemon, "daemon", false, "run as daemon in background")
	
	return cmd
}

func runWatch(targetName string, daemon bool) error {
	// Create root context for the entire operation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Load configuration
	configPath := getConfigPath()
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	
	// Create logger
	log := logger.CreateLogger("", verbosity)
	
	// Create dependency factory and build dependencies
	factory := poltergeist.NewDependencyFactory(projectRoot, log, cfg)
	deps := factory.CreateDefaults()
	
	// Create Poltergeist instance with properly injected dependencies
	p := poltergeist.New(cfg, projectRoot, log, deps, configPath)
	
	// Start watching
	printInfo(fmt.Sprintf("Starting Poltergeist v%s", version))
	
	if targetName != "" {
		printInfo(fmt.Sprintf("Watching target: %s", targetName))
	} else {
		printInfo("Watching all enabled targets")
	}
	
	// Start with context - this will be passed down through all layers
	if err := p.StartWithContext(ctx, targetName); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}
	
	// Handle shutdown signals with proper context cancellation
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	
	// Wait for shutdown signal
	sig := <-sigChan
	printInfo(fmt.Sprintf("Received signal: %s", sig))
	
	// Cancel context to trigger graceful shutdown
	cancel()
	
	// Give some time for graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	printInfo("Shutting down gracefully...")
	p.StopWithContext(shutdownCtx)
	
	if err := p.Cleanup(); err != nil {
		printWarning(fmt.Sprintf("Cleanup error: %v", err))
	}
	
	printSuccess("Poltergeist stopped gracefully")
	return nil
}

func loadConfig(path string) (*types.PoltergeistConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	
	var cfg types.PoltergeistConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	
	// Validate version
	if cfg.Version != "1.0" {
		return nil, fmt.Errorf("unsupported config version: %s", cfg.Version)
	}
	
	return &cfg, nil
}