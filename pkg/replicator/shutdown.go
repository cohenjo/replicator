package replicator

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// ShutdownHandler manages graceful shutdown of the replicator service
type ShutdownHandler struct {
	service         *Service
	logger          *logrus.Logger
	shutdownTimeout time.Duration
	signals         []os.Signal
	hooks           []ShutdownHook
	mu              sync.RWMutex
	isShuttingDown  bool
}

// ShutdownHook represents a function to call during shutdown
type ShutdownHook struct {
	Name     string
	Priority int // Lower numbers execute first
	Timeout  time.Duration
	Fn       func(ctx context.Context) error
}

// ShutdownHandlerOptions configures the shutdown handler
type ShutdownHandlerOptions struct {
	Service         *Service
	Logger          *logrus.Logger
	ShutdownTimeout time.Duration
	Signals         []os.Signal
}

// NewShutdownHandler creates a new shutdown handler
func NewShutdownHandler(opts ShutdownHandlerOptions) *ShutdownHandler {
	if opts.Logger == nil {
		opts.Logger = logrus.New()
	}
	
	if opts.ShutdownTimeout == 0 {
		opts.ShutdownTimeout = 30 * time.Second
	}
	
	if opts.Signals == nil {
		opts.Signals = []os.Signal{
			syscall.SIGINT,  // Ctrl+C
			syscall.SIGTERM, // Termination signal
			syscall.SIGQUIT, // Quit signal
		}
	}
	
	return &ShutdownHandler{
		service:         opts.Service,
		logger:          opts.Logger,
		shutdownTimeout: opts.ShutdownTimeout,
		signals:         opts.Signals,
		hooks:           make([]ShutdownHook, 0),
	}
}

// AddHook adds a shutdown hook to be executed during graceful shutdown
func (sh *ShutdownHandler) AddHook(hook ShutdownHook) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	
	if hook.Timeout == 0 {
		hook.Timeout = 10 * time.Second
	}
	
	sh.hooks = append(sh.hooks, hook)
	
	// Sort hooks by priority (lower numbers first)
	for i := len(sh.hooks) - 1; i > 0; i-- {
		if sh.hooks[i].Priority < sh.hooks[i-1].Priority {
			sh.hooks[i], sh.hooks[i-1] = sh.hooks[i-1], sh.hooks[i]
		} else {
			break
		}
	}
	
	sh.logger.WithFields(logrus.Fields{
		"hook":     hook.Name,
		"priority": hook.Priority,
		"timeout":  hook.Timeout,
	}).Debug("Added shutdown hook")
}

// Wait waits for shutdown signals and handles graceful shutdown
func (sh *ShutdownHandler) Wait() error {
	// Create signal channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, sh.signals...)
	
	sh.logger.WithField("signals", sh.signals).Info("Waiting for shutdown signal")
	
	// Wait for signal
	sig := <-sigChan
	sh.logger.WithField("signal", sig).Info("Received shutdown signal")
	
	return sh.Shutdown()
}

// Shutdown performs graceful shutdown
func (sh *ShutdownHandler) Shutdown() error {
	sh.mu.Lock()
	if sh.isShuttingDown {
		sh.mu.Unlock()
		return fmt.Errorf("shutdown already in progress")
	}
	sh.isShuttingDown = true
	sh.mu.Unlock()
	
	sh.logger.Info("Starting graceful shutdown")
	startTime := time.Now()
	
	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), sh.shutdownTimeout)
	defer cancel()
	
	var shutdownError error
	
	// Execute shutdown hooks first
	if err := sh.executeHooks(ctx); err != nil {
		sh.logger.WithError(err).Error("Some shutdown hooks failed")
		shutdownError = err
	}
	
	// Stop the main service
	if sh.service != nil {
		sh.logger.Info("Stopping main service")
		if err := sh.service.Stop(ctx); err != nil {
			sh.logger.WithError(err).Error("Failed to stop main service")
			if shutdownError == nil {
				shutdownError = err
			}
		}
	}
	
	duration := time.Since(startTime)
	if shutdownError == nil {
		sh.logger.WithField("duration", duration).Info("Graceful shutdown completed successfully")
	} else {
		sh.logger.WithFields(logrus.Fields{
			"duration": duration,
			"error":    shutdownError,
		}).Error("Graceful shutdown completed with errors")
	}
	
	return shutdownError
}

// executeHooks executes all registered shutdown hooks
func (sh *ShutdownHandler) executeHooks(ctx context.Context) error {
	sh.mu.RLock()
	hooks := make([]ShutdownHook, len(sh.hooks))
	copy(hooks, sh.hooks)
	sh.mu.RUnlock()
	
	if len(hooks) == 0 {
		sh.logger.Debug("No shutdown hooks to execute")
		return nil
	}
	
	sh.logger.WithField("count", len(hooks)).Info("Executing shutdown hooks")
	
	var errors []error
	
	for _, hook := range hooks {
		sh.logger.WithFields(logrus.Fields{
			"hook":     hook.Name,
			"priority": hook.Priority,
		}).Debug("Executing shutdown hook")
		
		hookCtx, hookCancel := context.WithTimeout(ctx, hook.Timeout)
		
		hookStart := time.Now()
		err := hook.Fn(hookCtx)
		hookDuration := time.Since(hookStart)
		
		hookCancel()
		
		if err != nil {
			sh.logger.WithFields(logrus.Fields{
				"hook":     hook.Name,
				"duration": hookDuration,
				"error":    err,
			}).Error("Shutdown hook failed")
			errors = append(errors, fmt.Errorf("hook %s failed: %w", hook.Name, err))
		} else {
			sh.logger.WithFields(logrus.Fields{
				"hook":     hook.Name,
				"duration": hookDuration,
			}).Debug("Shutdown hook completed successfully")
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("shutdown hooks failed: %v", errors)
	}
	
	return nil
}

// IsShuttingDown returns true if shutdown is in progress
func (sh *ShutdownHandler) IsShuttingDown() bool {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.isShuttingDown
}

// GetHooks returns a copy of all registered hooks
func (sh *ShutdownHandler) GetHooks() []ShutdownHook {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	
	hooks := make([]ShutdownHook, len(sh.hooks))
	copy(hooks, sh.hooks)
	return hooks
}

// DefaultShutdownHooks returns commonly used shutdown hooks
func DefaultShutdownHooks() []ShutdownHook {
	return []ShutdownHook{
		{
			Name:     "cleanup_temp_files",
			Priority: 10,
			Timeout:  5 * time.Second,
			Fn: func(ctx context.Context) error {
				// Clean up temporary files
				// This is a placeholder - implement based on your needs
				return nil
			},
		},
		{
			Name:     "flush_buffers",
			Priority: 20,
			Timeout:  10 * time.Second,
			Fn: func(ctx context.Context) error {
				// Flush any buffered data
				// This is a placeholder - implement based on your needs
				return nil
			},
		},
		{
			Name:     "close_connections",
			Priority: 30,
			Timeout:  15 * time.Second,
			Fn: func(ctx context.Context) error {
				// Close database connections, HTTP clients, etc.
				// This is a placeholder - implement based on your needs
				return nil
			},
		},
	}
}

// WithTimeout creates a new shutdown handler with a different timeout
func (sh *ShutdownHandler) WithTimeout(timeout time.Duration) *ShutdownHandler {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	
	newHandler := *sh
	newHandler.shutdownTimeout = timeout
	return &newHandler
}

// WithSignals creates a new shutdown handler that listens for different signals
func (sh *ShutdownHandler) WithSignals(signals ...os.Signal) *ShutdownHandler {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	
	newHandler := *sh
	newHandler.signals = signals
	return &newHandler
}

// CreateDatabaseCleanupHook creates a hook for cleaning up database connections
func CreateDatabaseCleanupHook(cleanup func(ctx context.Context) error) ShutdownHook {
	return ShutdownHook{
		Name:     "database_cleanup",
		Priority: 25,
		Timeout:  15 * time.Second,
		Fn:       cleanup,
	}
}

// CreateMetricsFlushHook creates a hook for flushing metrics
func CreateMetricsFlushHook(flush func(ctx context.Context) error) ShutdownHook {
	return ShutdownHook{
		Name:     "metrics_flush",
		Priority: 15,
		Timeout:  10 * time.Second,
		Fn:       flush,
	}
}

// CreatePositionSaveHook creates a hook for saving position data
func CreatePositionSaveHook(save func(ctx context.Context) error) ShutdownHook {
	return ShutdownHook{
		Name:     "position_save",
		Priority: 5, // High priority - save positions first
		Timeout:  20 * time.Second,
		Fn:       save,
	}
}

// CreateStreamStopHook creates a hook for stopping individual streams
func CreateStreamStopHook(streamName string, stop func(ctx context.Context) error) ShutdownHook {
	return ShutdownHook{
		Name:     fmt.Sprintf("stream_%s_stop", streamName),
		Priority: 35,
		Timeout:  30 * time.Second,
		Fn:       stop,
	}
}

// HandlePanic recovers from panics and initiates graceful shutdown
func (sh *ShutdownHandler) HandlePanic() {
	if r := recover(); r != nil {
		sh.logger.WithField("panic", r).Error("Panic occurred, initiating graceful shutdown")
		
		// Try to shutdown gracefully
		go func() {
			if err := sh.Shutdown(); err != nil {
				sh.logger.WithError(err).Error("Failed to shutdown gracefully after panic")
				os.Exit(1)
			}
			os.Exit(1)
		}()
		
		// Give it some time, then force exit
		time.Sleep(sh.shutdownTimeout + 5*time.Second)
		sh.logger.Error("Forced exit after panic")
		os.Exit(1)
	}
}

// WaitForShutdownComplete waits for shutdown to complete or timeout
func (sh *ShutdownHandler) WaitForShutdownComplete(timeout time.Duration) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	timeoutChan := time.After(timeout)
	
	for {
		select {
		case <-timeoutChan:
			return fmt.Errorf("timeout waiting for shutdown to complete")
		case <-ticker.C:
			if !sh.IsShuttingDown() {
				// If we're not shutting down, we might not have started yet
				continue
			}
			
			// Check if service is stopped
			if sh.service != nil && sh.service.GetStatus() == StatusStopped {
				return nil
			}
		}
	}
}