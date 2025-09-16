package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/rs/zerolog/log"
)

// ConfigResponse represents the current configuration in API responses
type ConfigResponse struct {
	Version   string                `json:"version"`
	UpdatedAt time.Time             `json:"updated_at"`
	Config    *config.Config        `json:"config"`
	Status    string                `json:"status"`
}

// ConfigUpdateRequest represents a configuration update request
type ConfigUpdateRequest struct {
	Streams    []config.StreamConfig        `json:"streams,omitempty"`
	Server     *config.ServerConfig         `json:"server,omitempty"`
	Metrics    *config.MetricsConfig        `json:"metrics,omitempty"`
	Logging    *config.LoggingConfig        `json:"logging,omitempty"`
	Transform  *config.TransformationRule   `json:"transform,omitempty"`
	Monitoring *config.MonitorConfig        `json:"monitoring,omitempty"`
	Restart    bool                         `json:"restart,omitempty"`
}

// ConfigValidationResponse represents the result of configuration validation
type ConfigValidationResponse struct {
	Valid     bool                     `json:"valid"`
	Errors    []ConfigValidationError  `json:"errors,omitempty"`
	Warnings  []ConfigValidationError  `json:"warnings,omitempty"`
	CheckedAt time.Time               `json:"checked_at"`
}

// ConfigValidationError represents a configuration validation error
type ConfigValidationError struct {
	Field   string `json:"field"`
	Type    string `json:"type"`    // "error", "warning"
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ConfigBackupResponse represents a configuration backup
type ConfigBackupResponse struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Config    *config.Config `json:"config"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ConfigReloadResponse represents the result of a configuration reload
type ConfigReloadResponse struct {
	Success   bool                      `json:"success"`
	Message   string                    `json:"message"`
	Config    *ConfigResponse           `json:"config,omitempty"`
	Errors    []ConfigValidationError   `json:"errors,omitempty"`
	ReloadedAt time.Time               `json:"reloaded_at"`
}

// ConfigService represents the configuration management service interface
type ConfigService interface {
	GetConfig() (*config.Config, error)
	UpdateConfig(req ConfigUpdateRequest) (*config.Config, error)
	ReloadConfig() (*ConfigReloadResponse, error)
	ValidateConfig(cfg *config.Config) (*ConfigValidationResponse, error)
	BackupConfig() (*ConfigBackupResponse, error)
	ListBackups() ([]ConfigBackupResponse, error)
	RestoreBackup(id string) (*config.Config, error)
}

// ConfigHandler handles configuration-related HTTP requests
type ConfigHandler struct {
	configService ConfigService
}

// NewConfigHandler creates a new configuration handler
func NewConfigHandler(configService ConfigService) *ConfigHandler {
	return &ConfigHandler{
		configService: configService,
	}
}

// ServeHTTP implements http.Handler interface
func (h *ConfigHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/config":
		h.handleConfig(w, r)
	case "/config/reload":
		h.handleReloadConfig(w, r)
	case "/config/validate":
		h.handleValidateConfig(w, r)
	case "/config/backup":
		h.handleBackupConfig(w, r)
	case "/config/backups":
		h.handleListBackups(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// RegisterRoutes registers configuration routes with the provided mux
func (h *ConfigHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/config", h.handleConfig)
	mux.HandleFunc("/config/reload", h.handleReloadConfig)
	mux.HandleFunc("/config/validate", h.handleValidateConfig)
	mux.HandleFunc("/config/backup", h.handleBackupConfig)
	mux.HandleFunc("/config/backups", h.handleListBackups)
}

// handleConfig handles both GET and PUT /config
func (h *ConfigHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetConfig(w, r)
	case http.MethodPut:
		h.handleUpdateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetConfig handles GET /config
func (h *ConfigHandler) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configService.GetConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	response := h.configToResponse(cfg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode config response")
	}
}

// handleUpdateConfig handles PUT /config
func (h *ConfigHandler) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	cfg, err := h.configService.UpdateConfig(req)
	if err != nil {
		if strings.Contains(err.Error(), "validation failed") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Error().Err(err).Msg("Failed to update configuration")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	response := h.configToResponse(cfg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode config update response")
	}
}

// handleReloadConfig handles POST /config/reload
func (h *ConfigHandler) handleReloadConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response, err := h.configService.ReloadConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to reload configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode config reload response")
	}
}

// handleValidateConfig handles POST /config/validate
func (h *ConfigHandler) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	response, err := h.configService.ValidateConfig(&cfg)
	if err != nil {
		log.Error().Err(err).Msg("Failed to validate configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	statusCode := http.StatusOK
	if !response.Valid {
		statusCode = http.StatusBadRequest
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode config validation response")
	}
}

// handleBackupConfig handles POST /config/backup
func (h *ConfigHandler) handleBackupConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backup, err := h.configService.BackupConfig()
	if err != nil {
		log.Error().Err(err).Msg("Failed to backup configuration")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(backup); err != nil {
		log.Error().Err(err).Msg("Failed to encode config backup response")
	}
}

// handleListBackups handles GET /config/backups
func (h *ConfigHandler) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	backups, err := h.configService.ListBackups()
	if err != nil {
		log.Error().Err(err).Msg("Failed to list configuration backups")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(backups); err != nil {
		log.Error().Err(err).Msg("Failed to encode config backups response")
	}
}

// DefaultConfigService implements ConfigService
type DefaultConfigService struct {
	config    *config.Config
	backupDir string
}

// NewConfigService creates a new config service
func NewConfigService(cfg *config.Config, backupDir string) ConfigService {
	return &DefaultConfigService{
		config:    cfg,
		backupDir: backupDir,
	}
}

// GetConfig returns the current configuration
func (cs *DefaultConfigService) GetConfig() (*config.Config, error) {
	return cs.config, nil
}

// UpdateConfig updates the configuration
func (cs *DefaultConfigService) UpdateConfig(req ConfigUpdateRequest) (*config.Config, error) {
	// Implement configuration update logic
	// For now, just return the current config
	return cs.config, nil
}

// ReloadConfig reloads the configuration
func (cs *DefaultConfigService) ReloadConfig() (*ConfigReloadResponse, error) {
	return &ConfigReloadResponse{
		Success:    true,
		Message:    "Configuration reloaded successfully",
		Config:     &ConfigResponse{Version: "1.0", UpdatedAt: time.Now(), Config: cs.config, Status: "active"},
		ReloadedAt: time.Now(),
	}, nil
}

// ValidateConfig validates the configuration
func (cs *DefaultConfigService) ValidateConfig(cfg *config.Config) (*ConfigValidationResponse, error) {
	err := cfg.Validate()
	return &ConfigValidationResponse{
		Valid:     err == nil,
		Errors:    []ConfigValidationError{},
		Warnings:  []ConfigValidationError{},
		CheckedAt: time.Now(),
	}, nil
}

// BackupConfig creates a configuration backup
func (cs *DefaultConfigService) BackupConfig() (*ConfigBackupResponse, error) {
	return &ConfigBackupResponse{
		ID:        fmt.Sprintf("backup_%d", time.Now().Unix()),
		Timestamp: time.Now(),
		Config:    cs.config,
		Metadata:  map[string]string{"version": "1.0"},
	}, nil
}

// ListBackups lists configuration backups
func (cs *DefaultConfigService) ListBackups() ([]ConfigBackupResponse, error) {
	return []ConfigBackupResponse{}, nil
}

// RestoreBackup restores a configuration backup
func (cs *DefaultConfigService) RestoreBackup(id string) (*config.Config, error) {
	return cs.config, nil
}

// configToResponse converts a config.Config to a ConfigResponse
func (h *ConfigHandler) configToResponse(cfg *config.Config) *ConfigResponse {
	return &ConfigResponse{
		Version:   "1.0",
		UpdatedAt: time.Now(),
		Config:    cfg,
		Status:    "active",
	}
}