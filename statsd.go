package main

import (
	"log"
	"time"

	"github.com/alexcesaro/statsd"
)

type StatsdClient struct {
	*statsd.Client
	options []statsd.Option
	muted   bool
}

func (sc *StatsdClient) Muted() bool {
	return sc.muted
}

func NewStatsdClient(opts ...statsd.Option) (*StatsdClient, error) {
	statsd_client, err := statsd.New(opts...)
	return &StatsdClient{
		Client:  statsd_client,
		options: opts,
		muted:   err != nil,
	}, err
}

type StatsdTracker struct {
	*StatsdClient
}

func NewStatsdTracker(client *StatsdClient) *StatsdTracker {
	return &StatsdTracker{client}
}

func (st *StatsdTracker) Track(event *Event) error {
	if st.Muted() {
		st.StatsdClient, err = NewStatsdClient(st.StatsdClient.options...)
		if nil != err {
			log.Println(err)
		}
	}

	switch event.Properties["Type"] {
	case Timer:
		st.Timing(event.Event, int(event.Properties["Value"].(time.Duration)/time.Millisecond))
	case Counter:
		st.Count(event.Event, event.Properties["Value"])
	case Gauge:
		st.Gauge(event.Event, event.Properties["Value"])
	case Set:
		st.Unique(event.Event, event.Properties["Value"].(string))
	}

	return nil
}
