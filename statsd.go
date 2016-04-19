package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/quipo/statsd"
	"github.com/zpencerq/goproxy"
)

type StatsdTracker struct {
	*statsd.StatsdClient
}

func NewStatsdTracker(host string, prefix string) *StatsdTracker {
	tracker := &StatsdTracker{statsd.NewStatsdClient(host, prefix)}
	err := tracker.CreateSocket()
	if nil != err {
		log.Println(err)
		os.Exit(1)
	}
	return tracker
}

func (st *StatsdTracker) Track(event *goproxy.Event) error {
	stat := fmt.Sprintf("%s.%s", event.Event, event.Properties["Host"].(string))
	switch event.Properties["Type"] {
	case goproxy.Duration:
		return st.PrecisionTiming(stat, event.Properties["Value"].(time.Duration))
	case goproxy.Count:
		return st.Incr(stat, event.Properties["Value"].(int64))
	case goproxy.Gauge:
		return st.Gauge(stat, event.Properties["Value"].(int64))
	default:
		return nil
	}
}
