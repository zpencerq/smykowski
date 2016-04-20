package main

import (
	"log"
	"os"
	"time"
)

type EventType int

const (
	Timer EventType = iota
	Counter
	Gauge
	Set
)

func (et *EventType) String() string {
	switch *et {
	case Timer:
		return "Timer"
	case Counter:
		return "Counter"
	case Gauge:
		return "Gauge"
	case Set:
		return "Set"
	default:
		return "Unknown"
	}
}

type Event struct {
	Event      string                 `json:"event"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

func NewEvent(event string, properties map[string]interface{}) *Event {
	if properties == nil {
		properties = make(map[string]interface{})
	}

	return &Event{Event: event, Properties: properties}
}

func NewDurationEvent(stat string, start time.Time, properties map[string]interface{}) *Event {
	if properties == nil {
		properties = make(map[string]interface{})
	}
	event := NewEvent(stat, properties)
	event.Properties["Value"] = time.Since(start)
	event.Properties["Type"] = Timer
	return event
}

type Tracker interface {
	Track(event *Event) error
}

func (wrapper *ProxyHttpServerWrapper) TrackEvent(event *Event) error {
	return wrapper.Tracker.Track(event)
}

func (wrapper *ProxyHttpServerWrapper) TrackDuration(event *Event, start time.Time) error {
	new_event := NewDurationEvent(event.Event, start, event.Properties)
	return wrapper.Tracker.Track(new_event)
}

type LogTracker struct {
	*log.Logger
}

func NewLogTracker(logger *log.Logger) *LogTracker {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags)
	}
	return &LogTracker{logger}
}

func (lt *LogTracker) Track(event *Event) error {
	lt.Printf("%s - %v", event.Event, event.Properties)
	return nil
}

type NoopTracker struct{}

func NewNoopTracker() *NoopTracker {
	return &NoopTracker{}
}

func (nt *NoopTracker) Track(event *Event) error {
	return nil
}

type CompositeTracker struct {
	trackers []Tracker
}

func NewCompositeTracker(trackers ...Tracker) *CompositeTracker {
	composite_tracker := &CompositeTracker{make([]Tracker, 0)}
	for _, tracker := range trackers {
		composite_tracker.trackers = append(composite_tracker.trackers,
			tracker)
	}
	return composite_tracker
}

func (st *CompositeTracker) Track(event *Event) error {
	var err error
	for _, tracker := range st.trackers {
		err = tracker.Track(event)
	}
	return err
}
