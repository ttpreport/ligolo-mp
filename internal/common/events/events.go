package events

type Event struct {
	Type  string
	Data  interface{}
	Error error
}

var EventStream chan Event

func init() {
	EventStream = make(chan Event, 10)
}
