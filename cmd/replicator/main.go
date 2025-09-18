package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/replicator"
	"github.com/sirupsen/logrus"
)

var (
	// Build information (set by ldflags during build)
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Parse command line flags
	var (
		configFile    = flag.String("config", "", "Configuration file path")
		logLevel      = flag.String("log-level", "", "Log level (debug, info, warn, error)")
		showVersion   = flag.Bool("version", false, "Show version information")
		showConfig    = flag.Bool("show-config", false, "Show configuration and exit")
		validateOnly  = flag.Bool("validate", false, "Validate configuration and exit")
		sleep         = flag.Bool("sleep", false, "Sleep for a duration (for testing purposes)")
		generateConfig = flag.String("generate-config", "", "Generate configuration template to file")
	)
	flag.Parse()

	// Show version information
	if *showVersion {
		fmt.Printf("Replicator %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Build Date: %s\n", date)
		os.Exit(0)
	}

	// Generate configuration template
	if *generateConfig != "" {
		if err := generateConfigTemplate(*generateConfig); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating config template: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Configuration template generated: %s\n", *generateConfig)
		os.Exit(0)
	}

	// Create logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := loadConfiguration(*configFile, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to load configuration")
	}

	// Override log level from command line
	if *logLevel != "" {
		cfg.Logging.Level = *logLevel
	}

	// Configure logger
	if err := configureLogger(logger, cfg.Logging.Level); err != nil {
		logger.WithError(err).Fatal("Failed to configure logger")
	}

	logger.WithFields(logrus.Fields{
		"version": version,
		"commit":  commit,
		"date":    date,
	}).Info("Starting Replicator")

	// Show configuration and exit if requested
	if *showConfig {
		showConfiguration(cfg, logger)
		os.Exit(0)
	}

	// Validate configuration and exit if requested
	if *validateOnly {
		logger.Info("Configuration validation passed")
		os.Exit(0)
	}

	// Sleep for a duration and exit if requested
	if *sleep {
		logger.Info("Sleep for 60 minutes, continuing...")
		time.Sleep(60 * time.Minute)
	}

	// Run the application
	if err := run(cfg, logger); err != nil {
		logger.WithError(err).Fatal("Application failed")
	}
}

// loadConfiguration loads and validates configuration
func loadConfiguration(configFile string, logger *logrus.Logger) (*config.Config, error) {
	loader := config.NewLoader()

	var cfg *config.Config
	var err error

	// Check for REPLICATOR_CONFIG_FILE environment variable first
	if configFile == "" {
		if envConfigFile := os.Getenv("REPLICATOR_CONFIG_FILE"); envConfigFile != "" {
			configFile = envConfigFile
		}
	}

	if configFile != "" {
		// Load from specified file
		cfg, err = loader.LoadFromFile(configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from file %s: %w", configFile, err)
		}
		logger.WithField("file", configFile).Info("Configuration loaded from file")
	} else {
		// Load with default behavior (search paths, env vars, etc.)
		cfg, err = loader.LoadDefault()
		if err != nil {
			return nil, fmt.Errorf("failed to load configuration: %w", err)
		}
		logger.Info("Configuration loaded with defaults")
	}

	return cfg, nil
}

// configureLogger configures the logger based on log level
func configureLogger(logger *logrus.Logger, logLevel string) error {
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("invalid log level %s: %w", logLevel, err)
	}

	logger.SetLevel(level)
	
	// Use text formatter for development
	if level == logrus.DebugLevel {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	return nil
}

// showConfiguration displays the loaded configuration
func showConfiguration(cfg *config.Config, logger *logrus.Logger) {
	fmt.Println("Configuration:")
	fmt.Printf("  Log Level: %s\n", cfg.Logging.Level)
	fmt.Printf("  Server:\n")
	fmt.Printf("    Host: %s\n", cfg.Server.Host)
	fmt.Printf("    Port: %d\n", cfg.Server.Port)
	if cfg.Server.TLS != nil {
		fmt.Printf("    TLS Enabled: %t\n", cfg.Server.TLS.Enabled)
	}
	fmt.Printf("  Telemetry:\n")
	fmt.Printf("    Enabled: %t\n", cfg.Telemetry.Enabled)
	fmt.Printf("    Metrics Enabled: %t\n", cfg.Telemetry.Metrics.Enabled)
	fmt.Printf("  Azure Authentication: %s\n", cfg.Azure.Authentication.Method)
	fmt.Printf("  Streams: %d configured\n", len(cfg.Streams))

	for i, stream := range cfg.Streams {
		fmt.Printf("    [%d] %s (%s -> %s)\n", 
			i+1, stream.Name, stream.Source.Type, stream.Target.Type)
		fmt.Printf("        Enabled: %t\n", stream.Enabled)
	}
}

// generateConfigTemplate generates a configuration template file
func generateConfigTemplate(filename string) error {
	loader := config.NewLoader()
	template := loader.GenerateTemplate()
	
	return loader.SaveToFile(template, filename)
}

// run starts and runs the replicator service
func run(cfg *config.Config, logger *logrus.Logger) error {
	// Create service
	service, err := replicator.NewService(replicator.ServiceOptions{
		Config:      cfg,
		Logger:      logger,
		EventBuffer: 10000,
	})
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	// Create shutdown handler
	shutdownHandler := replicator.NewShutdownHandler(replicator.ShutdownHandlerOptions{
		Service: service,
		Logger:  logger,
	})

	// Add default shutdown hooks
	for _, hook := range replicator.DefaultShutdownHooks() {
		shutdownHandler.AddHook(hook)
	}

	// Add custom hooks for position saving and cleanup
	if err := addCustomShutdownHooks(shutdownHandler, service, cfg, logger); err != nil {
		return fmt.Errorf("failed to add shutdown hooks: %w", err)
	}

	// Set up panic handler
	defer shutdownHandler.HandlePanic()

	// Start service
	ctx := context.Background()
	logger.Info("Starting replicator service")
	
	if err := service.Start(ctx); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Log service status
	logger.WithFields(logrus.Fields{
		"status": service.GetStatus(),
		"health": service.GetHealthStatus().Status,
	}).Info("Service started successfully")

	// Wait for shutdown signal
	logger.Info("Service is running. Press Ctrl+C to stop.")
	return shutdownHandler.Wait()
}

// addCustomShutdownHooks adds application-specific shutdown hooks
func addCustomShutdownHooks(shutdownHandler *replicator.ShutdownHandler, service *replicator.Service, cfg *config.Config, logger *logrus.Logger) error {
	// Position save hook
	positionSaveHook := replicator.CreatePositionSaveHook(func(ctx context.Context) error {
		logger.Info("Saving position data before shutdown")
		// TODO: Implement position saving logic
		return nil
	})
	shutdownHandler.AddHook(positionSaveHook)

	// Database cleanup hook
	dbCleanupHook := replicator.CreateDatabaseCleanupHook(func(ctx context.Context) error {
		logger.Info("Cleaning up database connections")
		// TODO: Implement database cleanup logic
		return nil
	})
	shutdownHandler.AddHook(dbCleanupHook)

	// Metrics flush hook
	metricsFlushHook := replicator.CreateMetricsFlushHook(func(ctx context.Context) error {
		logger.Info("Flushing metrics before shutdown")
		// TODO: Implement metrics flushing logic
		return nil
	})
	shutdownHandler.AddHook(metricsFlushHook)

	return nil
}

// Example usage information
func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Replicator - Database change data capture and replication tool\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                              # Run with default configuration\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -config=config.yaml          # Run with specific config file\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -log-level=debug             # Run with debug logging\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -show-config                 # Show loaded configuration\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -validate                    # Validate configuration only\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -generate-config=config.yaml # Generate config template\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -version                     # Show version information\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nEnvironment Variables:\n")
		fmt.Fprintf(os.Stderr, "  REPLICATOR_CONFIG_FILE          # Configuration file path\n")
		fmt.Fprintf(os.Stderr, "  REPLICATOR_LOG_LEVEL            # Log level\n")
		fmt.Fprintf(os.Stderr, "  REPLICATOR_SERVER_PORT          # Server port\n")
		fmt.Fprintf(os.Stderr, "  REPLICATOR_MONGODB_CONNECTION_STRING  # MongoDB connection\n")
		fmt.Fprintf(os.Stderr, "  REPLICATOR_POSTGRESQL_CONNECTION_STRING  # PostgreSQL connection\n")
		fmt.Fprintf(os.Stderr, "  ... and many more (see documentation)\n")
	}
}
