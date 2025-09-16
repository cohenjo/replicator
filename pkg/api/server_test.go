package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestServer() (*Server, error) {
	cfg := &config.Config{
		Global: config.GlobalConfig{
			MaxConcurrentStreams: 10,
			LogLevel:            "info",
		},
		Streams: []config.StreamConfig{},
		Monitoring: config.MonitoringConfig{
			PrometheusEnabled: true,
			PrometheusPort:    9090,
		},
	}

	serverCfg := DefaultServerConfig()
	serverCfg.Port = 0 // Use random port for testing

	telemetry := &metrics.TelemetryManager{}
	streamRunner := &mockStreamRunner{}

	return NewServer(cfg, serverCfg, telemetry, streamRunner)
}

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()

	assert.Equal(t, "0.0.0.0", cfg.Host)
	assert.Equal(t, 8080, cfg.Port)
	assert.Equal(t, 30*time.Second, cfg.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.WriteTimeout)
	assert.Equal(t, 60*time.Second, cfg.IdleTimeout)
	assert.False(t, cfg.EnableTLS)
	assert.True(t, cfg.EnableCORS)
	assert.Equal(t, []string{"*"}, cfg.CORSOrigins)
	assert.True(t, cfg.EnableMetrics)
	assert.False(t, cfg.EnableAuth)
}

func TestNewServer(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.config)
	assert.NotNil(t, server.httpServer)
	assert.NotNil(t, server.healthService)
	assert.NotNil(t, server.metricsService)
	assert.NotNil(t, server.streamService)
	assert.NotNil(t, server.configService)
	assert.NotNil(t, server.healthHandler)
	assert.NotNil(t, server.metricsHandler)
	assert.NotNil(t, server.streamsHandler)
	assert.NotNil(t, server.configHandler)
	assert.NotNil(t, server.metricsMiddleware)
}

func TestServer_GetAddr(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	addr := server.GetAddr()
	assert.Contains(t, addr, "0.0.0.0:0")
}

func TestServer_IsRunning(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Server should be considered "running" if httpServer is set
	assert.True(t, server.IsRunning())

	// Test with nil httpServer
	server.httpServer = nil
	assert.False(t, server.IsRunning())
}

func TestServer_HandleRoot(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	server.handleRoot(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "replicator-api", response["service"])
	assert.Equal(t, "running", response["status"])
	assert.Contains(t, response, "endpoints")
	assert.Contains(t, response, "timestamp")
}

func TestServer_HandleRootNotFound(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/invalid", nil)
	w := httptest.NewRecorder()

	server.handleRoot(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestServer_HandleAPIInfo(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()

	server.handleAPIInfo(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "v1", response["api_version"])
	assert.Equal(t, "replicator-api", response["service"])
	assert.Contains(t, response, "endpoints")
	assert.Contains(t, response, "authentication")

	// Verify endpoints structure
	endpoints := response["endpoints"].(map[string]interface{})
	assert.Contains(t, endpoints, "health")
	assert.Contains(t, endpoints, "metrics")
	assert.Contains(t, endpoints, "streams")
	assert.Contains(t, endpoints, "config")
}

func TestServer_CORSMiddleware(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	// Test with allowed origin
	corsHandler := server.corsMiddleware(testHandler, []string{"https://example.com"})
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	corsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))

	// Test with wildcard origin
	corsHandler = server.corsMiddleware(testHandler, []string{"*"})
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://any-origin.com")
	w = httptest.NewRecorder()

	corsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://any-origin.com", w.Header().Get("Access-Control-Allow-Origin"))

	// Test preflight request
	req = httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w = httptest.NewRecorder()

	corsHandler = server.corsMiddleware(testHandler, []string{"*"})
	corsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_AuthMiddleware(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test"))
	})

	validTokens := []string{"valid-token-123", "another-valid-token"}
	authHandler := server.authMiddleware(testHandler, validTokens)

	// Test without auth header
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Authorization header required")

	// Test with invalid format
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")
	w = httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid authorization format")

	// Test with invalid token
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w = httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid token")

	// Test with valid token
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token-123")
	w = httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test", w.Body.String())

	// Test health endpoint bypasses auth
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test metrics endpoint bypasses auth
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()

	authHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_LoggingMiddleware(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("test"))
	})

	loggingHandler := server.loggingMiddleware(testHandler)
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()

	loggingHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "test", w.Body.String())
}

func TestServer_RecoveryMiddleware(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	// Test handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	recoveryHandler := server.recoveryMiddleware(panicHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	// Should not panic
	recoveryHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Internal server error")
}

func TestResponseWriterWrapper(t *testing.T) {
	recorder := httptest.NewRecorder()
	wrapper := &responseWriterWrapper{
		ResponseWriter: recorder,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, wrapper.statusCode)
	assert.Equal(t, http.StatusCreated, recorder.Code)

	// Test Write
	data := []byte("test data")
	n, err := wrapper.Write(data)
	require.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, "test data", recorder.Body.String())
}

func TestServer_CreateMux(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	serverCfg := DefaultServerConfig()
	mux := server.createMux(serverCfg)
	assert.NotNil(t, mux)

	// Test various endpoints
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/"},
		{"GET", "/health"},
		{"GET", "/metrics"},
		{"GET", "/api"},
		{"GET", "/streams"},
		{"GET", "/config"},
		{"GET", "/api/v1/streams"},
		{"GET", "/api/v1/config"},
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint.method+" "+endpoint.path, func(t *testing.T) {
			req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
			w := httptest.NewRecorder()

			mux.ServeHTTP(w, req)

			// Should not return 404 (not found)
			assert.NotEqual(t, http.StatusNotFound, w.Code)
		})
	}
}

func TestServer_CreateMuxWithDisabledMetrics(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	serverCfg := DefaultServerConfig()
	serverCfg.EnableMetrics = false
	mux := server.createMux(serverCfg)

	// Test that metrics endpoint is not available
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Should return 404 since metrics are disabled
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestNewServerManager(t *testing.T) {
	manager := NewServerManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.servers)
	assert.Empty(t, manager.servers)
}

func TestServerManager_AddServer(t *testing.T) {
	manager := NewServerManager()
	server, err := createTestServer()
	require.NoError(t, err)

	manager.AddServer("test-server", server)

	retrievedServer, exists := manager.GetServer("test-server")
	assert.True(t, exists)
	assert.Equal(t, server, retrievedServer)
}

func TestServerManager_GetServer(t *testing.T) {
	manager := NewServerManager()

	// Test non-existent server
	_, exists := manager.GetServer("nonexistent")
	assert.False(t, exists)

	// Test existing server
	server, err := createTestServer()
	require.NoError(t, err)
	manager.AddServer("test-server", server)

	retrievedServer, exists := manager.GetServer("test-server")
	assert.True(t, exists)
	assert.Equal(t, server, retrievedServer)
}

func TestServerManager_GetServerStatus(t *testing.T) {
	manager := NewServerManager()
	server1, err := createTestServer()
	require.NoError(t, err)
	server2, err := createTestServer()
	require.NoError(t, err)

	manager.AddServer("server1", server1)
	manager.AddServer("server2", server2)

	status := manager.GetServerStatus()
	assert.Len(t, status, 2)
	assert.Contains(t, status, "server1")
	assert.Contains(t, status, "server2")
	assert.True(t, status["server1"])
	assert.True(t, status["server2"])
}

func TestServerManager_StopAll(t *testing.T) {
	manager := NewServerManager()
	server, err := createTestServer()
	require.NoError(t, err)

	manager.AddServer("test-server", server)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err = manager.StopAll(ctx)
	assert.NoError(t, err)
}

func TestServerIntegration(t *testing.T) {
	// Integration test for the entire server
	server, err := createTestServer()
	require.NoError(t, err)

	serverCfg := DefaultServerConfig()
	mux := server.createMux(serverCfg)

	// Test root endpoint
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test health endpoint
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test metrics endpoint
	req = httptest.NewRequest("GET", "/metrics", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test API info endpoint
	req = httptest.NewRequest("GET", "/api", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test streams endpoint
	req = httptest.NewRequest("GET", "/streams", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test config endpoint
	req = httptest.NewRequest("GET", "/config", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServerWithAuth(t *testing.T) {
	server, err := createTestServer()
	require.NoError(t, err)

	serverCfg := DefaultServerConfig()
	serverCfg.EnableAuth = true
	serverCfg.AuthTokens = []string{"test-token"}
	mux := server.createMux(serverCfg)

	// Test without auth
	req := httptest.NewRequest("GET", "/streams", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Test with auth
	req = httptest.NewRequest("GET", "/streams", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Test health endpoint bypasses auth
	req = httptest.NewRequest("GET", "/health", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func BenchmarkServer_HandleRoot(b *testing.B) {
	server, err := createTestServer()
	require.NoError(b, err)

	req := httptest.NewRequest("GET", "/", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		server.handleRoot(w, req)
	}
}

func BenchmarkServer_Middleware(b *testing.B) {
	server, err := createTestServer()
	require.NoError(b, err)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := server.loggingMiddleware(server.recoveryMiddleware(testHandler))
	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}