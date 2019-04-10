package events

// The action name for sync.
const (
	UpdateAction = "update"
	InsertAction = "insert"
	DeleteAction = "delete"
)

type RecordEvent struct {
	Action     string
	Schema     string
	Collection string

	Data []byte // let's keep a json here to use Kazaam
}
