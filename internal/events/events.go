package events

import (
	"fmt"
	"log/slog"
)

type EventType int

const (
	OK EventType = iota
	ERROR
	WARNING
)

var eventTypeNames = [...]string{"INFO", "WARNING", "ERROR"}

func (t EventType) String() string {
	return eventTypeNames[t]
}

func (t EventType) Slog() slog.Level {
	switch t {
	case OK:
		return slog.LevelInfo
	case WARNING:
		return slog.LevelWarn
	case ERROR:
		return slog.LevelError
	}

	return slog.LevelInfo
}

type Event struct {
	Type EventType
	Data string
}

var eventStream chan *Event

func init() {
	eventStream = make(chan *Event, 128)
}

func Publish(eventType EventType, eventData string, args ...any) {
	eventStream <- &Event{Type: eventType, Data: fmt.Sprintf(eventData, args...)}
}

func Recv() *Event {
	return <-eventStream
}
