package main

import (
	"time"

	"github.com/alexcesaro/statsd"
)

type StatsdTracker struct {
	*statsd.Client
}

func NewStatsdTracker(client *statsd.Client) *StatsdTracker {
	return &StatsdTracker{client}
}

func (st *StatsdTracker) Track(event *Event) error {
	switch event.Properties["Type"] {
	case Timer:
		st.Timing(event.Event, event.Properties["Value"].(time.Duration))
	case Counter:
		st.Count(event.Event, event.Properties["Value"].(int64))
	case Gauge:
		st.Gauge(event.Event, event.Properties["Value"].(int64))
	case Set:
		st.Unique(event.Event, event.Properties["Value"].(string))
	}

	return nil
}
