package position

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// PostgreSQLPosition implements Position for PostgreSQL WAL positions
type PostgreSQLPosition struct {
	// LSN is the Log Sequence Number
	LSN uint64 `json:"lsn"`
	
	// TxID is the transaction ID (optional)
	TxID uint64 `json:"tx_id,omitempty"`
	
	// Timeline is the WAL timeline ID
	Timeline uint32 `json:"timeline,omitempty"`
	
	// SlotName is the replication slot name
	SlotName string `json:"slot_name,omitempty"`
	
	// Database is the database name
	Database string `json:"database,omitempty"`
	
	// Timestamp when the position was captured
	Timestamp int64 `json:"timestamp"`
}

// NewPostgreSQLPosition creates a new PostgreSQL position
func NewPostgreSQLPosition(lsn uint64) *PostgreSQLPosition {
	return &PostgreSQLPosition{
		LSN: lsn,
	}
}

// NewPostgreSQLPositionFromString creates a PostgreSQL position from LSN string
func NewPostgreSQLPositionFromString(lsnStr string) (*PostgreSQLPosition, error) {
	lsn, err := ParseLSN(lsnStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LSN: %w", err)
	}
	
	return &PostgreSQLPosition{
		LSN: lsn,
	}, nil
}

// Serialize converts the position to JSON bytes
func (pp *PostgreSQLPosition) Serialize() ([]byte, error) {
	return json.Marshal(pp)
}

// Deserialize restores the position from JSON bytes
func (pp *PostgreSQLPosition) Deserialize(data []byte) error {
	return json.Unmarshal(data, pp)
}

// String returns a human-readable representation
func (pp *PostgreSQLPosition) String() string {
	lsnStr := FormatLSN(pp.LSN)
	if pp.SlotName != "" {
		return fmt.Sprintf("lsn=%s, slot=%s", lsnStr, pp.SlotName)
	}
	return fmt.Sprintf("lsn=%s", lsnStr)
}

// IsValid checks if the position is valid
func (pp *PostgreSQLPosition) IsValid() bool {
	return pp.LSN > 0
}

// Compare compares this position with another PostgreSQL position
func (pp *PostgreSQLPosition) Compare(other Position) int {
	otherPG, ok := other.(*PostgreSQLPosition)
	if !ok {
		return -1 // Different types, this is considered "less than"
	}
	
	// Compare LSNs
	if pp.LSN < otherPG.LSN {
		return -1
	} else if pp.LSN > otherPG.LSN {
		return 1
	}
	
	return 0
}

// SetTxID sets the transaction ID for this position
func (pp *PostgreSQLPosition) SetTxID(txID uint64) {
	pp.TxID = txID
}

// SetTimeline sets the timeline for this position
func (pp *PostgreSQLPosition) SetTimeline(timeline uint32) {
	pp.Timeline = timeline
}

// SetSlotName sets the replication slot name for this position
func (pp *PostgreSQLPosition) SetSlotName(slotName string) {
	pp.SlotName = slotName
}

// SetDatabase sets the database name for this position
func (pp *PostgreSQLPosition) SetDatabase(database string) {
	pp.Database = database
}

// SetTimestamp sets the timestamp for this position
func (pp *PostgreSQLPosition) SetTimestamp(timestamp int64) {
	pp.Timestamp = timestamp
}

// Clone creates a deep copy of this position
func (pp *PostgreSQLPosition) Clone() *PostgreSQLPosition {
	return &PostgreSQLPosition{
		LSN:       pp.LSN,
		TxID:      pp.TxID,
		Timeline:  pp.Timeline,
		SlotName:  pp.SlotName,
		Database:  pp.Database,
		Timestamp: pp.Timestamp,
	}
}

// Advance creates a new position advanced by the given LSN offset
func (pp *PostgreSQLPosition) Advance(offset uint64) *PostgreSQLPosition {
	newPos := pp.Clone()
	newPos.LSN += offset
	return newPos
}

// IsAfter checks if this position is after the other position
func (pp *PostgreSQLPosition) IsAfter(other *PostgreSQLPosition) bool {
	return pp.Compare(other) > 0
}

// IsBefore checks if this position is before the other position
func (pp *PostgreSQLPosition) IsBefore(other *PostgreSQLPosition) bool {
	return pp.Compare(other) < 0
}

// IsEqual checks if this position equals the other position
func (pp *PostgreSQLPosition) IsEqual(other *PostgreSQLPosition) bool {
	return pp.Compare(other) == 0
}

// GetLSNString returns the LSN as a PostgreSQL-formatted string (XX/XXXXXXXX)
func (pp *PostgreSQLPosition) GetLSNString() string {
	return FormatLSN(pp.LSN)
}

// ParseLSN parses a PostgreSQL LSN string (XX/XXXXXXXX format) to uint64
func ParseLSN(lsnStr string) (uint64, error) {
	parts := strings.Split(lsnStr, "/")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid LSN format: %s", lsnStr)
	}
	
	high, err := strconv.ParseUint(parts[0], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid LSN high part: %s", parts[0])
	}
	
	low, err := strconv.ParseUint(parts[1], 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid LSN low part: %s", parts[1])
	}
	
	return (high << 32) | low, nil
}

// FormatLSN formats a uint64 LSN to PostgreSQL string format (XX/XXXXXXXX)
func FormatLSN(lsn uint64) string {
	high := uint32(lsn >> 32)
	low := uint32(lsn & 0xFFFFFFFF)
	return fmt.Sprintf("%X/%X", high, low)
}

// PostgreSQLPositionFactory creates PostgreSQL positions from serialized data
type PostgreSQLPositionFactory struct{}

// CreatePosition creates a PostgreSQL position from serialized data
func (f *PostgreSQLPositionFactory) CreatePosition(data []byte) (Position, error) {
	var pos PostgreSQLPosition
	if err := pos.Deserialize(data); err != nil {
		return nil, err
	}
	return &pos, nil
}

// GetPositionType returns the position type identifier
func (f *PostgreSQLPositionFactory) GetPositionType() string {
	return "postgresql"
}

// RegisterPostgreSQLPositionFactory registers the PostgreSQL position factory
func RegisterPostgreSQLPositionFactory() {
	// In a real implementation, this would register with a global factory registry
	// For now, this is a placeholder for the registration mechanism
}