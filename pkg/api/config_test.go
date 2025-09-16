package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestConfig() *config.Config {
	return &config.Config{
		Global: config.GlobalConfig{
			MaxConcurrentStreams: 10,
			LogLevel:            "info",
		},
		Streams: []config.StreamConfig{
			{
				Name: "test-stream",
				Source: config.SourceConfig{
					Type:     "mysql",
					Host:     "localhost",
					Port:     3306,
					Database: "test_db",
					Username: "test_user",
					Password: "test_pass",
				},
				Target: config.TargetConfig{
					Type:     "mongo",
					Host:     "localhost",
					Port:     27017,
					Database: "test_target_db",
					Username: "test_target_user",
					Password: "test_target_pass",
				},
			},
		},
		Monitoring: config.MonitoringConfig{
			PrometheusEnabled: true,
			PrometheusPort:    9090,
		},
	}
}

func TestNewConfigService(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	assert.NotNil(t, service)
	assert.Equal(t, cfg, service.currentConfig)
	assert.Equal(t, "/path/to/config.yaml", service.configPath)
	assert.NotNil(t, service.backups)
	assert.Empty(t, service.backups)
	assert.NotNil(t, service.watchers)
	assert.Empty(t, service.watchers)
}

func TestConfigService_GetConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Test successful get
	result, err := service.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg, result)

	// Test with nil config
	service.currentConfig = nil
	_, err = service.GetConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration loaded")
}

func TestConfigService_UpdateConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Test successful update
	updateReq := ConfigUpdateRequest{
		Global: &config.GlobalConfig{
			MaxConcurrentStreams: 20,
			LogLevel:            "debug",
		},
	}

	result, err := service.UpdateConfig(updateReq)
	require.NoError(t, err)
	assert.Equal(t, 20, result.Global.MaxConcurrentStreams)
	assert.Equal(t, "debug", result.Global.LogLevel)

	// Test with nil config
	service.currentConfig = nil
	_, err = service.UpdateConfig(updateReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration loaded")
}

func TestConfigService_ValidateConfig(t *testing.T) {
	service := NewConfigService(nil, "/path/to/config.yaml")

	tests := []struct {
		name        string
		config      *config.Config
		expectValid bool
		expectError bool
	}{
		{
			name:        "valid config",
			config:      createTestConfig(),
			expectValid: true,
			expectError: false,
		},
		{
			name: "invalid max concurrent streams",
			config: &config.Config{
				Global: config.GlobalConfig{
					MaxConcurrentStreams: 0,
					LogLevel:            "info",
				},
				Streams: []config.StreamConfig{},
				Monitoring: config.MonitoringConfig{
					PrometheusEnabled: true,
					PrometheusPort:    9090,
				},
			},
			expectValid: false,
			expectError: false,
		},
		{
			name: "invalid prometheus port",
			config: &config.Config{
				Global: config.GlobalConfig{
					MaxConcurrentStreams: 10,
					LogLevel:            "info",
				},
				Streams: []config.StreamConfig{},
				Monitoring: config.MonitoringConfig{
					PrometheusEnabled: true,
					PrometheusPort:    0,
				},
			},
			expectValid: false,
			expectError: false,
		},
		{
			name: "duplicate stream names",
			config: &config.Config{
				Global: config.GlobalConfig{
					MaxConcurrentStreams: 10,
					LogLevel:            "info",
				},
				Streams: []config.StreamConfig{
					{
						Name: "duplicate",
						Source: config.SourceConfig{
							Type:     "mysql",
							Host:     "localhost",
							Port:     3306,
							Database: "test_db",
							Username: "test_user",
							Password: "test_pass",
						},
						Target: config.TargetConfig{
							Type:     "mongo",
							Host:     "localhost",
							Port:     27017,
							Database: "test_target_db",
							Username: "test_target_user",
							Password: "test_target_pass",
						},
					},
					{
						Name: "duplicate",
						Source: config.SourceConfig{
							Type:     "mysql",
							Host:     "localhost",
							Port:     3306,
							Database: "test_db2",
							Username: "test_user",
							Password: "test_pass",
						},
						Target: config.TargetConfig{
							Type:     "mongo",
							Host:     "localhost",
							Port:     27017,
							Database: "test_target_db2",
							Username: "test_target_user",
							Password: "test_target_pass",
						},
					},
				},
			},
			expectValid: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ValidateConfig(tt.config)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectValid, result.Valid)
				assert.WithinDuration(t, time.Now(), result.CheckedAt, time.Second)
				
				if !tt.expectValid {
					assert.NotEmpty(t, result.Errors)
				}
			}
		})
	}
}

func TestConfigService_ReloadConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	result, err := service.ReloadConfig()
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.Message)
	assert.WithinDuration(t, time.Now(), result.ReloadedAt, time.Second)
	assert.NotEmpty(t, result.PreviousHash)
	assert.NotEmpty(t, result.NewHash)
}

func TestConfigService_BackupConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Test successful backup
	backup, err := service.BackupConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, backup.ID)
	assert.Equal(t, "1.0", backup.Version)
	assert.WithinDuration(t, time.Now(), backup.CreatedAt, time.Second)
	assert.NotEmpty(t, backup.Hash)
	assert.Equal(t, len(cfg.Streams), len(backup.Config.Streams))

	// Verify backup is stored
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
	assert.Equal(t, backup.ID, backups[0].ID)

	// Test backup with nil config
	service.currentConfig = nil
	_, err = service.BackupConfig()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no configuration to backup")
}

func TestConfigService_RestoreConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Create a backup first
	backup, err := service.BackupConfig()
	require.NoError(t, err)

	// Test successful restore
	result, err := service.RestoreConfig(backup.ID)
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Test restore non-existent backup
	_, err = service.RestoreConfig("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "backup with ID nonexistent not found")
}

func TestConfigService_ListBackups(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Initially no backups
	backups, err := service.ListBackups()
	require.NoError(t, err)
	assert.Empty(t, backups)

	// Create some backups
	backup1, err := service.BackupConfig()
	require.NoError(t, err)
	
	time.Sleep(time.Millisecond) // Ensure different timestamps
	
	backup2, err := service.BackupConfig()
	require.NoError(t, err)

	// List backups
	backups, err = service.ListBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 2)

	// Verify backups are present
	backupIDs := []string{backups[0].ID, backups[1].ID}
	assert.Contains(t, backupIDs, backup1.ID)
	assert.Contains(t, backupIDs, backup2.ID)
}

func TestConfigService_GetConfigHash(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	hash1 := service.GetConfigHash()
	assert.NotEmpty(t, hash1)

	time.Sleep(time.Millisecond) // Ensure different timestamps
	
	hash2 := service.GetConfigHash()
	assert.NotEmpty(t, hash2)
	assert.NotEqual(t, hash1, hash2) // Hash includes timestamp

	// Test with nil config
	service.currentConfig = nil
	hash3 := service.GetConfigHash()
	assert.Empty(t, hash3)
}

func TestConfigService_WatchConfigChanges(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")

	// Create watcher
	watcher, err := service.WatchConfigChanges()
	require.NoError(t, err)
	assert.NotNil(t, watcher)

	// Update config to trigger notification
	updateReq := ConfigUpdateRequest{
		Global: &config.GlobalConfig{
			MaxConcurrentStreams: 20,
			LogLevel:            "debug",
		},
	}

	// Start goroutine to receive notification
	done := make(chan bool)
	var receivedConfig *config.Config
	go func() {
		select {
		case receivedConfig = <-watcher:
			done <- true
		case <-time.After(time.Second):
			done <- false
		}
	}()

	// Update config
	_, err = service.UpdateConfig(updateReq)
	require.NoError(t, err)

	// Wait for notification
	notified := <-done
	assert.True(t, notified)
	assert.NotNil(t, receivedConfig)
	assert.Equal(t, 20, receivedConfig.Global.MaxConcurrentStreams)
}

func TestNewConfigHandler(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.configService)
}

func TestConfigHandler_GetConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("GET", "/config", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "1.0", response.Version)
	assert.Equal(t, len(cfg.Streams), len(response.Streams))
	assert.Equal(t, cfg.Global.MaxConcurrentStreams, response.Global.MaxConcurrentStreams)
}

func TestConfigHandler_UpdateConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	updateReq := ConfigUpdateRequest{
		Global: &config.GlobalConfig{
			MaxConcurrentStreams: 25,
			LogLevel:            "warn",
		},
	}
	reqBody, _ := json.Marshal(updateReq)

	req := httptest.NewRequest("PUT", "/config", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 25, response.Global.MaxConcurrentStreams)
	assert.Equal(t, "warn", response.Global.LogLevel)
}

func TestConfigHandler_UpdateConfigInvalidJSON(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("PUT", "/config", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid JSON")
}

func TestConfigHandler_ReloadConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("POST", "/config/reload", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigReloadResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.NotEmpty(t, response.Message)
	assert.WithinDuration(t, time.Now(), response.ReloadedAt, time.Second)
}

func TestConfigHandler_ValidateConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	// Test valid config
	validConfig := createTestConfig()
	reqBody, _ := json.Marshal(validConfig)

	req := httptest.NewRequest("POST", "/config/validate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigValidationResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Valid)
	assert.WithinDuration(t, time.Now(), response.CheckedAt, time.Second)

	// Test invalid config
	invalidConfig := createTestConfig()
	invalidConfig.Global.MaxConcurrentStreams = 0
	reqBody, _ = json.Marshal(invalidConfig)

	req = httptest.NewRequest("POST", "/config/validate", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.False(t, response.Valid)
	assert.NotEmpty(t, response.Errors)
}

func TestConfigHandler_BackupConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("POST", "/config/backup", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigBackupResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "1.0", response.Version)
	assert.WithinDuration(t, time.Now(), response.CreatedAt, time.Second)
	assert.NotEmpty(t, response.Hash)
}

func TestConfigHandler_ListBackups(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	// Create some backups first
	_, err := service.BackupConfig()
	require.NoError(t, err)
	_, err = service.BackupConfig()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/config/backups", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Contains(t, response, "backups")
	assert.Contains(t, response, "total")
	assert.Equal(t, float64(2), response["total"])
}

func TestConfigHandler_RestoreConfig(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	// Create a backup first
	backup, err := service.BackupConfig()
	require.NoError(t, err)

	req := httptest.NewRequest("POST", fmt.Sprintf("/config/backups/%s/restore", backup.ID), nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ConfigResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "1.0", response.Version)
}

func TestConfigHandler_RestoreConfigNotFound(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("POST", "/config/backups/nonexistent/restore", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestConfigHandler_NotFound(t *testing.T) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	tests := []struct {
		method string
		path   string
	}{
		{"GET", "/config/invalid"},
		{"POST", "/config/invalid"},
		{"PUT", "/config/invalid"},
		{"DELETE", "/config"},
		{"PATCH", "/config"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestGenerateBackupID(t *testing.T) {
	id1 := generateBackupID()
	time.Sleep(time.Millisecond)
	id2 := generateBackupID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "backup_")
	assert.Contains(t, id2, "backup_")
}

func BenchmarkConfigHandler_GetConfig(b *testing.B) {
	cfg := createTestConfig()
	service := NewConfigService(cfg, "/path/to/config.yaml")
	handler := NewConfigHandler(service)

	req := httptest.NewRequest("GET", "/config", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkConfigService_ValidateConfig(b *testing.B) {
	service := NewConfigService(nil, "/path/to/config.yaml")
	cfg := createTestConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ValidateConfig(cfg)
	}
}