package ttlcache

type EventType byte

type Event struct {
	Sequence uint64
	EventType EventType
	Key string
	Value string
}

const (
	_ = iota
	EventDelete EventType = iota
	EventAdd
)
