package position

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// MySQLPosition implements Position for MySQL binlog positions
type MySQLPosition struct {
	// File is the binlog file name
	File string `json:"file"`
	
	// Position is the position within the binlog file
	Position uint32 `json:"position"`
	
	// GTID is the Global Transaction ID (optional)
	GTID string `json:"gtid,omitempty"`
	
	// ServerID is the MySQL server ID
	ServerID uint32 `json:"server_id,omitempty"`
	
	// Timestamp when the position was captured
	Timestamp int64 `json:"timestamp"`
}

// NewMySQLPosition creates a new MySQL position
func NewMySQLPosition(file string, position uint32) *MySQLPosition {
	return &MySQLPosition{
		File:     file,
		Position: position,
	}
}

// NewMySQLPositionFromMySQL creates a MySQL position from go-mysql Position
func NewMySQLPositionFromMySQL(pos mysql.Position) *MySQLPosition {
	return &MySQLPosition{
		File:     pos.Name,
		Position: pos.Pos,
	}
}

// ToMySQLPosition converts to go-mysql Position
func (mp *MySQLPosition) ToMySQLPosition() mysql.Position {
	return mysql.Position{
		Name: mp.File,
		Pos:  mp.Position,
	}
}

// Serialize converts the position to JSON bytes
func (mp *MySQLPosition) Serialize() ([]byte, error) {
	return json.Marshal(mp)
}

// Deserialize restores the position from JSON bytes
func (mp *MySQLPosition) Deserialize(data []byte) error {
	return json.Unmarshal(data, mp)
}

// String returns a human-readable representation
func (mp *MySQLPosition) String() string {
	if mp.GTID != "" {
		return fmt.Sprintf("file=%s, pos=%d, gtid=%s", mp.File, mp.Position, mp.GTID)
	}
	return fmt.Sprintf("file=%s, pos=%d", mp.File, mp.Position)
}

// IsValid checks if the position is valid
func (mp *MySQLPosition) IsValid() bool {
	return mp.File != "" && mp.Position > 0
}

// Compare compares this position with another MySQL position
func (mp *MySQLPosition) Compare(other Position) int {
	otherMySQL, ok := other.(*MySQLPosition)
	if !ok {
		return -1 // Different types, this is considered "less than"
	}
	
	// Compare by file first
	fileComp := compareFiles(mp.File, otherMySQL.File)
	if fileComp != 0 {
		return fileComp
	}
	
	// If files are the same, compare positions
	if mp.Position < otherMySQL.Position {
		return -1
	} else if mp.Position > otherMySQL.Position {
		return 1
	}
	
	return 0
}

// SetGTID sets the GTID for this position
func (mp *MySQLPosition) SetGTID(gtid string) {
	mp.GTID = gtid
}

// SetServerID sets the server ID for this position
func (mp *MySQLPosition) SetServerID(serverID uint32) {
	mp.ServerID = serverID
}

// SetTimestamp sets the timestamp for this position
func (mp *MySQLPosition) SetTimestamp(timestamp int64) {
	mp.Timestamp = timestamp
}

// Clone creates a deep copy of this position
func (mp *MySQLPosition) Clone() *MySQLPosition {
	return &MySQLPosition{
		File:      mp.File,
		Position:  mp.Position,
		GTID:      mp.GTID,
		ServerID:  mp.ServerID,
		Timestamp: mp.Timestamp,
	}
}

// Advance creates a new position advanced by the given amount
func (mp *MySQLPosition) Advance(bytes uint32) *MySQLPosition {
	newPos := mp.Clone()
	newPos.Position += bytes
	return newPos
}

// IsAfter checks if this position is after the other position
func (mp *MySQLPosition) IsAfter(other *MySQLPosition) bool {
	return mp.Compare(other) > 0
}

// IsBefore checks if this position is before the other position
func (mp *MySQLPosition) IsBefore(other *MySQLPosition) bool {
	return mp.Compare(other) < 0
}

// IsEqual checks if this position equals the other position
func (mp *MySQLPosition) IsEqual(other *MySQLPosition) bool {
	return mp.Compare(other) == 0
}

// compareFiles compares two MySQL binlog file names
// Handles formats like mysql-bin.000001, mysql-bin.000002, etc.
func compareFiles(file1, file2 string) int {
	// Extract numeric parts for comparison
	num1 := extractFileNumber(file1)
	num2 := extractFileNumber(file2)
	
	if num1 < num2 {
		return -1
	} else if num1 > num2 {
		return 1
	}
	
	// If numbers are equal, compare strings lexicographically
	if file1 < file2 {
		return -1
	} else if file1 > file2 {
		return 1
	}
	
	return 0
}

// extractFileNumber extracts the numeric part from a binlog file name
func extractFileNumber(filename string) int {
	// Find the last dot and extract the number after it
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return 0
	}
	
	numberStr := filename[lastDot+1:]
	if number, err := strconv.Atoi(numberStr); err == nil {
		return number
	}
	
	return 0
}

// MySQLPositionFactory creates MySQL positions from serialized data
type MySQLPositionFactory struct{}

// CreatePosition creates a MySQL position from serialized data
func (f *MySQLPositionFactory) CreatePosition(data []byte) (Position, error) {
	var pos MySQLPosition
	if err := pos.Deserialize(data); err != nil {
		return nil, err
	}
	return &pos, nil
}

// GetPositionType returns the position type identifier
func (f *MySQLPositionFactory) GetPositionType() string {
	return "mysql"
}

// RegisterMySQLPositionFactory registers the MySQL position factory
func RegisterMySQLPositionFactory() {
	// In a real implementation, this would register with a global factory registry
	// For now, this is a placeholder for the registration mechanism
}