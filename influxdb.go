package main

import (
	"log"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

type InfluxConfig struct {
	// influx.UDPConfig
	Addr        string
	PayloadSize int

	// influx.BatchPointsConfig
	Precision        string
	Database         string
	RetentionPolicy  string
	WriteConsistency string
}

type InfluxDataClient struct {
	influx.Client
	conf    InfluxConfig
	udpConf influx.UDPConfig
	bpConf  influx.BatchPointsConfig
	err     error
}

func (c *InfluxDataClient) Errored() bool {
	return c.err != nil
}

func NewInfluxDataClient(conf InfluxConfig) (*InfluxDataClient, error) {
	udpConfig := influx.UDPConfig{Addr: conf.Addr, PayloadSize: conf.PayloadSize}
	udpClient, err := influx.NewUDPClient(udpConfig)

	bpConfig := influx.BatchPointsConfig{
		Database:         conf.Database,
		RetentionPolicy:  conf.RetentionPolicy,
		WriteConsistency: conf.WriteConsistency,
	}

	return &InfluxDataClient{
		Client:  udpClient,
		conf:    conf,
		udpConf: udpConfig,
		bpConf:  bpConfig,
		err:     err,
	}, err
}

type InfluxDataTracker struct {
	*InfluxDataClient
}

func NewInfluxDataTracker(client *InfluxDataClient) *InfluxDataTracker {
	return &InfluxDataTracker{client}
}

func (st *InfluxDataTracker) Track(event *Event) error {
	if st.Errored() {
		st.InfluxDataClient, err = NewInfluxDataClient(st.InfluxDataClient.conf)
		if nil != err {
			log.Println(err)
		}
	}

	tags := make(map[string]string)

	if event.Properties["Tags"] == nil {
		event.Properties["Tags"] = make(map[string]string)
	}

	for k, v := range event.Properties["Tags"].(map[string]string) {
		tags[k] = v
	}

	fields := make(map[string]interface{})
	for k, v := range event.Properties {
		if k != "Tags" {
			switch v := v.(type) {
			case time.Duration:
				fields[k] = int64(v / time.Millisecond)
			default:
				fields[k] = v
			}
		}
	}
	for k, v := range tags {
		if _, present := fields[k]; present {
			fields[k] = v
		}
	}

	point, err := influx.NewPoint(event.Event,
		tags,
		fields,
		time.Now(),
	)
	if err != nil {
		log.Println(err)
		return err
	}

	batchPoints, err := influx.NewBatchPoints(st.InfluxDataClient.bpConf)
	if err != nil {
		log.Println(err)
		return err
	}

	batchPoints.AddPoint(point)
	err = st.Write(batchPoints)

	return err
}
