package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/rs/zerolog/log"
)

// StreamRunner represents a stream management interface
type StreamRunner interface {
	Start(ctx context.Context, stream config.StreamConfig) error
	Stop(ctx context.Context, name string) error
	Pause(ctx context.Context, name string) error
	Resume(ctx context.Context, name string) error
	GetStatus(name string) StreamStatus
	ListStreams() []StreamInfo
}

// StreamStatus represents the status of a stream
type StreamStatus struct {
	Name     string                `json:"name"`
	Status   config.StreamStatus   `json:"status"`
	Uptime   time.Duration         `json:"uptime"`
	Metrics  map[string]interface{} `json:"metrics"`
	Error    string                `json:"error,omitempty"`
}

// StreamInfo provides information about a stream
type StreamInfo struct {
	Name          string                `json:"name"`
	Status        config.StreamStatus   `json:"status"`
	Source        config.SourceConfig   `json:"source"`
	Target        config.TargetConfig   `json:"target"`
	Transformation *config.TransformationRulesConfig `json:"transformation,omitempty"`
	Enabled       bool                  `json:"enabled"`
	Uptime        time.Duration         `json:"uptime"`
	LastError     string                `json:"last_error,omitempty"`
}

// DefaultStreamRunner implements StreamRunner
type DefaultStreamRunner struct {
	streams map[string]*StreamStatus
	mutex   sync.RWMutex
}

// NewDefaultStreamRunner creates a new default stream runner
func NewDefaultStreamRunner() *DefaultStreamRunner {
	return &DefaultStreamRunner{
		streams: make(map[string]*StreamStatus),
	}
}

// Start starts a stream
func (sr *DefaultStreamRunner) Start(ctx context.Context, stream config.StreamConfig) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	sr.streams[stream.Name] = &StreamStatus{
		Name:   stream.Name,
		Status: config.StreamStatusStarting,
		Metrics: make(map[string]interface{}),
	}
	
	// Simulate starting
	go func() {
		time.Sleep(100 * time.Millisecond)
		sr.mutex.Lock()
		defer sr.mutex.Unlock()
		if status, exists := sr.streams[stream.Name]; exists {
			status.Status = config.StreamStatusRunning
		}
	}()
	
	return nil
}

// Stop stops a stream
func (sr *DefaultStreamRunner) Stop(ctx context.Context, name string) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	if status, exists := sr.streams[name]; exists {
		status.Status = config.StreamStatusStopped
	}
	return nil
}

// Pause pauses a stream
func (sr *DefaultStreamRunner) Pause(ctx context.Context, name string) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	if status, exists := sr.streams[name]; exists {
		status.Status = config.StreamStatusPaused
	}
	return nil
}

// Resume resumes a stream
func (sr *DefaultStreamRunner) Resume(ctx context.Context, name string) error {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	if status, exists := sr.streams[name]; exists {
		status.Status = config.StreamStatusRunning
	}
	return nil
}

// GetStatus gets stream status
func (sr *DefaultStreamRunner) GetStatus(name string) StreamStatus {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	
	if status, exists := sr.streams[name]; exists {
		return *status
	}
	return StreamStatus{Name: name, Status: config.StreamStatusStopped}
}

// ListStreams lists all streams
func (sr *DefaultStreamRunner) ListStreams() []StreamInfo {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	
	var streams []StreamInfo
	for _, status := range sr.streams {
		streams = append(streams, StreamInfo{
			Name:   status.Name,
			Status: status.Status,
			Uptime: status.Uptime,
		})
	}
	return streams
}

// StreamListResponse represents the response for listing streams
type StreamListResponse struct {
	Streams []StreamInfo `json:"streams"`
	Total   int          `json:"total"`
	Page    int          `json:"page,omitempty"`
	Limit   int          `json:"limit,omitempty"`
}

// StreamActionRequest represents a request to perform an action on a stream
type StreamActionRequest struct {
	Action string `json:"action"` // start, stop, pause, resume, restart
}

// StreamActionResponse represents the response for a stream action
type StreamActionResponse struct {
	Success   bool        `json:"success"`
	Message   string      `json:"message"`
	Stream    StreamInfo  `json:"stream"`
	Timestamp time.Time   `json:"timestamp"`
}

// StreamManager represents the stream management interface
type StreamManager interface {
	ListStreams(page, limit int) (*StreamListResponse, error)
	GetStream(id string) (*StreamInfo, error)
	CreateStream(config config.StreamConfig) (*StreamInfo, error)
	UpdateStream(id string, config config.StreamConfig) (*StreamInfo, error)
	DeleteStream(id string) error
	ExecuteAction(id string, action string) (*StreamActionResponse, error)
	GetStreamMetrics(id string) (map[string]interface{}, error)
}

// DefaultStreamManager implements StreamManager
type DefaultStreamManager struct {
	runner StreamRunner
	config *config.Config
}

// NewStreamService creates a new stream manager
func NewStreamService(cfg *config.Config, runner StreamRunner) StreamManager {
	return &DefaultStreamManager{
		runner: runner,
		config: cfg,
	}
}

// ListStreams lists all streams
func (sm *DefaultStreamManager) ListStreams(page, limit int) (*StreamListResponse, error) {
	streams := sm.runner.ListStreams()
	
	// Apply pagination if specified
	total := len(streams)
	if limit > 0 && page > 0 {
		start := (page - 1) * limit
		end := start + limit
		if start < total {
			if end > total {
				end = total
			}
			streams = streams[start:end]
		} else {
			streams = []StreamInfo{}
		}
	}
	
	return &StreamListResponse{
		Streams: streams,
		Total:   total,
		Page:    page,
		Limit:   limit,
	}, nil
}

// GetStream gets a specific stream
func (sm *DefaultStreamManager) GetStream(id string) (*StreamInfo, error) {
	streams := sm.runner.ListStreams()
	for _, stream := range streams {
		if stream.Name == id {
			return &stream, nil
		}
	}
	return nil, fmt.Errorf("stream not found: %s", id)
}

// CreateStream creates a new stream
func (sm *DefaultStreamManager) CreateStream(streamConfig config.StreamConfig) (*StreamInfo, error) {
	ctx := context.Background()
	if err := sm.runner.Start(ctx, streamConfig); err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}
	
	return &StreamInfo{
		Name:    streamConfig.Name,
		Status:  config.StreamStatusStarting,
		Source:  streamConfig.Source,
		Target:  streamConfig.Target,
		Enabled: streamConfig.Enabled,
	}, nil
}

// UpdateStream updates an existing stream
func (sm *DefaultStreamManager) UpdateStream(id string, streamConfig config.StreamConfig) (*StreamInfo, error) {
	// Stop existing stream
	ctx := context.Background()
	_ = sm.runner.Stop(ctx, id)
	
	// Start with new config
	if err := sm.runner.Start(ctx, streamConfig); err != nil {
		return nil, fmt.Errorf("failed to update stream: %w", err)
	}
	
	return &StreamInfo{
		Name:    streamConfig.Name,
		Status:  config.StreamStatusStarting,
		Source:  streamConfig.Source,
		Target:  streamConfig.Target,
		Enabled: streamConfig.Enabled,
	}, nil
}

// DeleteStream deletes a stream
func (sm *DefaultStreamManager) DeleteStream(id string) error {
	ctx := context.Background()
	return sm.runner.Stop(ctx, id)
}

// ExecuteAction executes an action on a stream
func (sm *DefaultStreamManager) ExecuteAction(id string, action string) (*StreamActionResponse, error) {
	ctx := context.Background()
	var err error
	
	switch strings.ToLower(action) {
	case "start":
		// Get stream config from configuration
		for _, streamConfig := range sm.config.Streams {
			if streamConfig.Name == id {
				err = sm.runner.Start(ctx, streamConfig)
				break
			}
		}
	case "stop":
		err = sm.runner.Stop(ctx, id)
	case "pause":
		err = sm.runner.Pause(ctx, id)
	case "resume":
		err = sm.runner.Resume(ctx, id)
	case "restart":
		_ = sm.runner.Stop(ctx, id)
		time.Sleep(100 * time.Millisecond)
		for _, streamConfig := range sm.config.Streams {
			if streamConfig.Name == id {
				err = sm.runner.Start(ctx, streamConfig)
				break
			}
		}
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
	
	if err != nil {
		return &StreamActionResponse{
			Success:   false,
			Message:   err.Error(),
			Timestamp: time.Now(),
		}, err
	}
	
	// Get updated stream info
	stream, _ := sm.GetStream(id)
	
	return &StreamActionResponse{
		Success:   true,
		Message:   fmt.Sprintf("Action '%s' executed successfully", action),
		Stream:    *stream,
		Timestamp: time.Now(),
	}, nil
}

// GetStreamMetrics gets metrics for a stream
func (sm *DefaultStreamManager) GetStreamMetrics(id string) (map[string]interface{}, error) {
	status := sm.runner.GetStatus(id)
	return status.Metrics, nil
}

// StreamsHandler handles stream-related HTTP requests
type StreamsHandler struct {
	streamService StreamManager
}

// NewStreamsHandler creates a new streams handler
func NewStreamsHandler(streamService StreamManager) *StreamsHandler {
	return &StreamsHandler{
		streamService: streamService,
	}
}

// ServeHTTP handles HTTP requests for streams
func (h *StreamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/streams")
	if path == "" || path == "/" {
		switch r.Method {
		case http.MethodGet:
			h.handleListStreams(w, r)
		case http.MethodPost:
			h.handleCreateStream(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	
	// Extract stream ID and action
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	
	streamID := parts[0]
	
	if len(parts) == 1 {
		// Operations on specific stream
		switch r.Method {
		case http.MethodGet:
			h.handleGetStream(w, r, streamID)
		case http.MethodPut:
			h.handleUpdateStream(w, r, streamID)
		case http.MethodDelete:
			h.handleDeleteStream(w, r, streamID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	} else if len(parts) == 2 {
		action := parts[1]
		switch action {
		case "actions":
			if r.Method == http.MethodPost {
				h.handleStreamAction(w, r, streamID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		case "metrics":
			if r.Method == http.MethodGet {
				h.handleGetStreamMetrics(w, r, streamID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// handleListStreams handles GET /streams
func (h *StreamsHandler) handleListStreams(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	
	response, err := h.streamService.ListStreams(page, limit)
	if err != nil {
		log.Error().Err(err).Msg("Failed to list streams")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode streams list response")
	}
}

// handleGetStream handles GET /streams/{id}
func (h *StreamsHandler) handleGetStream(w http.ResponseWriter, r *http.Request, streamID string) {
	stream, err := h.streamService.GetStream(streamID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			log.Error().Err(err).Msg("Failed to get stream")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stream); err != nil {
		log.Error().Err(err).Msg("Failed to encode stream response")
	}
}

// handleCreateStream handles POST /streams
func (h *StreamsHandler) handleCreateStream(w http.ResponseWriter, r *http.Request) {
	var streamConfig config.StreamConfig
	if err := json.NewDecoder(r.Body).Decode(&streamConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	stream, err := h.streamService.CreateStream(streamConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create stream")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(stream); err != nil {
		log.Error().Err(err).Msg("Failed to encode stream creation response")
	}
}

// handleUpdateStream handles PUT /streams/{id}
func (h *StreamsHandler) handleUpdateStream(w http.ResponseWriter, r *http.Request, streamID string) {
	var streamConfig config.StreamConfig
	if err := json.NewDecoder(r.Body).Decode(&streamConfig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	stream, err := h.streamService.UpdateStream(streamID, streamConfig)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update stream")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(stream); err != nil {
		log.Error().Err(err).Msg("Failed to encode stream update response")
	}
}

// handleDeleteStream handles DELETE /streams/{id}
func (h *StreamsHandler) handleDeleteStream(w http.ResponseWriter, r *http.Request, streamID string) {
	if err := h.streamService.DeleteStream(streamID); err != nil {
		log.Error().Err(err).Msg("Failed to delete stream")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusNoContent)
}

// handleStreamAction handles POST /streams/{id}/actions
func (h *StreamsHandler) handleStreamAction(w http.ResponseWriter, r *http.Request, streamID string) {
	var req StreamActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	response, err := h.streamService.ExecuteAction(streamID, req.Action)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if strings.Contains(err.Error(), "unsupported") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			log.Error().Err(err).Msg("Failed to execute stream action")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode stream action response")
	}
}

// handleGetStreamMetrics handles GET /streams/{id}/metrics
func (h *StreamsHandler) handleGetStreamMetrics(w http.ResponseWriter, r *http.Request, streamID string) {
	metrics, err := h.streamService.GetStreamMetrics(streamID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			log.Error().Err(err).Msg("Failed to get stream metrics")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		log.Error().Err(err).Msg("Failed to encode stream metrics response")
	}
}