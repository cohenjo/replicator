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
	"github.com/cohenjo/replicator/pkg/streams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock StreamRunner for testing
type mockStreamRunner struct{}

func (m *mockStreamRunner) Start() error               { return nil }
func (m *mockStreamRunner) Stop() error                { return nil }
func (m *mockStreamRunner) IsRunning() bool            { return false }
func (m *mockStreamRunner) GetConfig() *config.Config  { return nil }
func (m *mockStreamRunner) GetMetrics() interface{}    { return nil }

func createTestStreamService() *StreamService {
	cfg := &config.Config{}
	runner := &mockStreamRunner{}
	return NewStreamService(cfg, runner)
}

func createTestStreamCreateRequest() StreamCreateRequest {
	return StreamCreateRequest{
		Name: "test-stream",
		Type: "mysql-to-mongo",
		Source: config.SourceConfig{
			Type:   "mysql",
			Host:   "localhost",
			Port:   3306,
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
		AutoStart: false,
	}
}

func TestNewStreamService(t *testing.T) {
	cfg := &config.Config{}
	runner := &mockStreamRunner{}
	service := NewStreamService(cfg, runner)

	assert.NotNil(t, service)
	assert.Equal(t, cfg, service.config)
	assert.Equal(t, runner, service.streamRunner)
	assert.NotNil(t, service.streamStore)
	assert.Empty(t, service.streamStore)
}

func TestStreamService_CreateStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Test successful creation
	stream, err := service.CreateStream(req)
	require.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, req.Name, stream.Name)
	assert.Equal(t, req.Type, stream.Type)
	assert.Equal(t, "created", stream.Status)
	assert.Equal(t, req.Source, stream.Source)
	assert.Equal(t, req.Target, stream.Target)
	assert.NotEmpty(t, stream.ID)
	assert.WithinDuration(t, time.Now(), stream.CreatedAt, time.Second)
	assert.WithinDuration(t, time.Now(), stream.UpdatedAt, time.Second)

	// Test duplicate creation
	_, err = service.CreateStream(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestStreamService_CreateStreamValidation(t *testing.T) {
	service := createTestStreamService()

	tests := []struct {
		name        string
		modifyReq   func(*StreamCreateRequest)
		expectedErr string
	}{
		{
			name: "missing name",
			modifyReq: func(req *StreamCreateRequest) {
				req.Name = ""
			},
			expectedErr: "stream name is required",
		},
		{
			name: "missing type",
			modifyReq: func(req *StreamCreateRequest) {
				req.Type = ""
			},
			expectedErr: "stream type is required",
		},
		{
			name: "invalid source",
			modifyReq: func(req *StreamCreateRequest) {
				req.Source.Type = ""
			},
			expectedErr: "invalid source config",
		},
		{
			name: "invalid target",
			modifyReq: func(req *StreamCreateRequest) {
				req.Target.Type = ""
			},
			expectedErr: "invalid target config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := createTestStreamCreateRequest()
			tt.modifyReq(&req)

			_, err := service.CreateStream(req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestStreamService_CreateStreamWithAutoStart(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()
	req.AutoStart = true

	stream, err := service.CreateStream(req)
	require.NoError(t, err)
	assert.Equal(t, "running", stream.Status)
}

func TestStreamService_GetStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test successful get
	stream, err := service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, createdStream.ID, stream.ID)
	assert.Equal(t, createdStream.Name, stream.Name)

	// Test stream not found
	_, err = service.GetStream("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamService_ListStreams(t *testing.T) {
	service := createTestStreamService()

	// Create multiple streams
	for i := 0; i < 5; i++ {
		req := createTestStreamCreateRequest()
		req.Name = fmt.Sprintf("test-stream-%d", i)
		_, err := service.CreateStream(req)
		require.NoError(t, err)
	}

	// Test listing all streams
	streams, total, err := service.ListStreams(1, 10, "")
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, streams, 5)

	// Test pagination
	streams, total, err = service.ListStreams(1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, streams, 2)

	streams, total, err = service.ListStreams(2, 2, "")
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, streams, 2)

	// Test filtering
	streams, total, err = service.ListStreams(1, 10, "test-stream-1")
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, streams, 1)
	assert.Contains(t, streams[0].Name, "test-stream-1")

	// Test empty result
	streams, total, err = service.ListStreams(10, 10, "")
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, streams, 0)
}

func TestStreamService_UpdateStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test successful update
	updateReq := StreamUpdateRequest{
		Name: stringPtr("updated-stream"),
	}
	updatedStream, err := service.UpdateStream(createdStream.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "updated-stream", updatedStream.Name)
	assert.True(t, updatedStream.UpdatedAt.After(createdStream.UpdatedAt))

	// Test update non-existent stream
	_, err = service.UpdateStream("nonexistent", updateReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test update running stream
	err = service.StartStream(createdStream.ID)
	require.NoError(t, err)
	_, err = service.UpdateStream(createdStream.ID, updateReq)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot update running stream")
}

func TestStreamService_DeleteStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test successful deletion
	err = service.DeleteStream(createdStream.ID)
	require.NoError(t, err)

	// Verify stream is deleted
	_, err = service.GetStream(createdStream.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test delete non-existent stream
	err = service.DeleteStream("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamService_StartStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test successful start
	err = service.StartStream(createdStream.ID)
	require.NoError(t, err)

	// Verify status
	stream, err := service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", stream.Status)

	// Test start already running stream
	err = service.StartStream(createdStream.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test start non-existent stream
	err = service.StartStream("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamService_StopStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create and start a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)
	err = service.StartStream(createdStream.ID)
	require.NoError(t, err)

	// Test successful stop
	err = service.StopStream(createdStream.ID)
	require.NoError(t, err)

	// Verify status
	stream, err := service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, "stopped", stream.Status)

	// Test stop already stopped stream
	err = service.StopStream(createdStream.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already stopped")

	// Test stop non-existent stream
	err = service.StopStream("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamService_PauseResumeStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create and start a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)
	err = service.StartStream(createdStream.ID)
	require.NoError(t, err)

	// Test pause
	err = service.PauseStream(createdStream.ID)
	require.NoError(t, err)

	stream, err := service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, "paused", stream.Status)

	// Test resume
	err = service.ResumeStream(createdStream.ID)
	require.NoError(t, err)

	stream, err = service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", stream.Status)

	// Test pause non-running stream
	err = service.StopStream(createdStream.ID)
	require.NoError(t, err)
	err = service.PauseStream(createdStream.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can only pause running streams")

	// Test resume non-paused stream
	err = service.ResumeStream(createdStream.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "can only resume paused streams")
}

func TestStreamService_RestartStream(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create and start a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)
	err = service.StartStream(createdStream.ID)
	require.NoError(t, err)

	// Test restart
	err = service.RestartStream(createdStream.ID)
	require.NoError(t, err)

	stream, err := service.GetStream(createdStream.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", stream.Status)

	// Test restart non-existent stream
	err = service.RestartStream("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStreamService_GetStreamMetrics(t *testing.T) {
	service := createTestStreamService()
	req := createTestStreamCreateRequest()

	// Create a stream first
	createdStream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test get metrics
	metrics, err := service.GetStreamMetrics(createdStream.ID)
	require.NoError(t, err)
	assert.NotNil(t, metrics)
	assert.GreaterOrEqual(t, metrics.EventsProcessed, int64(0))
	assert.GreaterOrEqual(t, metrics.UptimeSeconds, int64(0))

	// Test get metrics for non-existent stream
	_, err = service.GetStreamMetrics("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestNewStreamsHandler(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	assert.NotNil(t, handler)
	assert.Equal(t, service, handler.streamService)
}

func TestStreamsHandler_ListStreams(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create some test streams
	for i := 0; i < 3; i++ {
		req := createTestStreamCreateRequest()
		req.Name = fmt.Sprintf("test-stream-%d", i)
		_, err := service.CreateStream(req)
		require.NoError(t, err)
	}

	// Test GET /streams
	req := httptest.NewRequest("GET", "/streams", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response StreamListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 3, response.Total)
	assert.Len(t, response.Streams, 3)
	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 20, response.Limit)
}

func TestStreamsHandler_ListStreamsWithPagination(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create some test streams
	for i := 0; i < 5; i++ {
		req := createTestStreamCreateRequest()
		req.Name = fmt.Sprintf("test-stream-%d", i)
		_, err := service.CreateStream(req)
		require.NoError(t, err)
	}

	// Test with pagination
	req := httptest.NewRequest("GET", "/streams?page=2&limit=2", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response StreamListResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, 5, response.Total)
	assert.Len(t, response.Streams, 2)
	assert.Equal(t, 2, response.Page)
	assert.Equal(t, 2, response.Limit)
}

func TestStreamsHandler_CreateStream(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	req := createTestStreamCreateRequest()
	reqBody, _ := json.Marshal(req)

	httpReq := httptest.NewRequest("POST", "/streams", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response StreamResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, req.Name, response.Name)
	assert.Equal(t, req.Type, response.Type)
	assert.Equal(t, "created", response.Status)
}

func TestStreamsHandler_CreateStreamInvalidJSON(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	httpReq := httptest.NewRequest("POST", "/streams", strings.NewReader("invalid json"))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid JSON")
}

func TestStreamsHandler_GetStream(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test GET /streams/{id}
	httpReq := httptest.NewRequest("GET", fmt.Sprintf("/streams/%s", stream.ID), nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response StreamResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, stream.ID, response.ID)
	assert.Equal(t, stream.Name, response.Name)
}

func TestStreamsHandler_GetStreamNotFound(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	httpReq := httptest.NewRequest("GET", "/streams/nonexistent", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "not found")
}

func TestStreamsHandler_UpdateStream(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Update request
	updateReq := StreamUpdateRequest{
		Name: stringPtr("updated-stream"),
	}
	reqBody, _ := json.Marshal(updateReq)

	httpReq := httptest.NewRequest("PUT", fmt.Sprintf("/streams/%s", stream.ID), bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response StreamResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "updated-stream", response.Name)
}

func TestStreamsHandler_DeleteStream(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test DELETE /streams/{id}
	httpReq := httptest.NewRequest("DELETE", fmt.Sprintf("/streams/%s", stream.ID), nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)

	// Verify stream is deleted
	_, err = service.GetStream(stream.ID)
	assert.Error(t, err)
}

func TestStreamsHandler_StreamActions(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	actions := []string{"start", "pause", "resume", "stop", "restart"}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			actionReq := StreamActionRequest{Action: action}
			reqBody, _ := json.Marshal(actionReq)

			httpReq := httptest.NewRequest("POST", fmt.Sprintf("/streams/%s/actions", stream.ID), bytes.NewReader(reqBody))
			httpReq.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, httpReq)

			// Some actions might fail due to state, but should not be internal server error
			assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusBadRequest)
		})
	}
}

func TestStreamsHandler_InvalidAction(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	actionReq := StreamActionRequest{Action: "invalid"}
	reqBody, _ := json.Marshal(actionReq)

	httpReq := httptest.NewRequest("POST", fmt.Sprintf("/streams/%s/actions", stream.ID), bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid action")
}

func TestStreamsHandler_GetStreamMetrics(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create a stream first
	req := createTestStreamCreateRequest()
	stream, err := service.CreateStream(req)
	require.NoError(t, err)

	// Test GET /streams/{id}/metrics
	httpReq := httptest.NewRequest("GET", fmt.Sprintf("/streams/%s/metrics", stream.ID), nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response StreamMetrics
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, response.EventsProcessed, int64(0))
}

func TestStreamsHandler_MethodNotAllowed(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	tests := []struct {
		method string
		path   string
	}{
		{"PATCH", "/streams"},
		{"DELETE", "/streams"},
		{"POST", "/streams/test/metrics"},
		{"PUT", "/streams/test/actions"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.method, tt.path), func(t *testing.T) {
			httpReq := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, httpReq)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestStreamsHandler_NotFound(t *testing.T) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	httpReq := httptest.NewRequest("GET", "/invalid/path", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGenerateStreamID(t *testing.T) {
	id1 := generateStreamID("Test Stream")
	id2 := generateStreamID("Test Stream")

	// IDs should be different due to timestamp
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "test_stream")
	assert.Contains(t, id2, "test_stream")
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func BenchmarkStreamsHandler_ListStreams(b *testing.B) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	// Create some test data
	for i := 0; i < 100; i++ {
		req := createTestStreamCreateRequest()
		req.Name = fmt.Sprintf("test-stream-%d", i)
		service.CreateStream(req)
	}

	httpReq := httptest.NewRequest("GET", "/streams", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httpReq)
	}
}

func BenchmarkStreamsHandler_CreateStream(b *testing.B) {
	service := createTestStreamService()
	handler := NewStreamsHandler(service)

	req := createTestStreamCreateRequest()
	reqBody, _ := json.Marshal(req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		// Reset service for each iteration to avoid conflicts
		service = createTestStreamService()
		handler = NewStreamsHandler(service)
		req.Name = fmt.Sprintf("test-stream-%d", i)
		reqBody, _ = json.Marshal(req)
		b.StartTimer()

		httpReq := httptest.NewRequest("POST", "/streams", bytes.NewReader(reqBody))
		httpReq.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httpReq)
	}
}