package position

import "errors"

// Common errors for position tracking
var (
	// ErrPositionNotFound indicates no position was found for the stream
	ErrPositionNotFound = errors.New("position not found")
	
	// ErrInvalidPosition indicates the position data is invalid
	ErrInvalidPosition = errors.New("invalid position")
	
	// ErrUnsupportedTrackerType indicates an unsupported tracker type
	ErrUnsupportedTrackerType = errors.New("unsupported tracker type")
	
	// ErrTrackerClosed indicates the tracker has been closed
	ErrTrackerClosed = errors.New("tracker is closed")
	
	// ErrPositionCorrupted indicates the stored position is corrupted
	ErrPositionCorrupted = errors.New("position data corrupted")
	
	// ErrStreamLocked indicates the stream is locked by another process
	ErrStreamLocked = errors.New("stream is locked")
	
	// ErrConnectionFailed indicates connection to storage backend failed
	ErrConnectionFailed = errors.New("connection to storage backend failed")
	
	// ErrPermissionDenied indicates insufficient permissions
	ErrPermissionDenied = errors.New("permission denied")
	
	// ErrStorageFull indicates storage is full
	ErrStorageFull = errors.New("storage is full")
	
	// ErrTimeout indicates operation timed out
	ErrTimeout = errors.New("operation timed out")
)