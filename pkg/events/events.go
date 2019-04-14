package events

// The action name for sync.
const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

type RecordKey struct {
	ID string `json:"id" bson:"_id"`
}

/*
RecordEvent is a record change event.
It's an attempt to allign all record change events to a unified structure.
Action: <insert|update|delete>
schema & collection are mapped to the equivilent terms for the other databases (e.g. db & table)
OldData contains the key to the previous record being changes (used for updates & deletes)
Data olds the full document in JSON format.
*/
type RecordEvent struct {
	Action     string
	Schema     string
	Collection string

	OldData []byte // Used for updates.
	Data    []byte // let's keep a json here to use Kazaam
}
