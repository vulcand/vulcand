// Package metrics provides tools for emitting metrics to different backends.
// Currently only statsd is supported.
package metrics

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/go-statsd-client/statsd"
	"github.com/mailgun/vulcand/Godeps/_workspace/src/github.com/mailgun/log"
)

type Metrics interface {
	EmitGauge(string, int64) error
	EmitTimer(string, time.Duration) error
	EmitCounter(string, int64) error
}

type StatsdMetrics struct {
	// statsd remote endpoint
	client *statsd.Client
	url    string
}

func NewStatsdMetrics(host string, port int, id string) (*StatsdMetrics, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	// format parameters
	hostPort := fmt.Sprintf("%v:%v", host, port)
	prefix := fmt.Sprintf("%v.%v", id, strings.Replace(hostname, ".", "_", -1))

	// start service
	client, err := statsd.New(hostPort, prefix)
	if err != nil {
		return nil, err
	}

	ms := &StatsdMetrics{
		url:    hostPort,
		client: client,
	}

	log.Infof("Emitting statsd metrics to: %v", hostPort)

	return ms, nil
}

func (ms *StatsdMetrics) EmitGauge(bucket string, value int64) error {
	if ms.client == nil {
		return fmt.Errorf("metrics service is not started")
	}

	// send metric
	err := ms.client.Gauge(bucket, value, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func (ms *StatsdMetrics) EmitTimer(bucket string, value time.Duration) error {
	if ms.client == nil {
		return fmt.Errorf("metrics service is not started")
	}

	// send metric in milliseconds (time.Duration is nanoseconds)
	err := ms.client.Timing(bucket, int64(value)/1000000, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func (ms *StatsdMetrics) EmitCounter(bucket string, value int64) error {
	if ms.client == nil {
		return fmt.Errorf("metrics service is not started")
	}

	// send metric
	err := ms.client.Inc(bucket, value, 1.0)
	if err != nil {
		return err
	}

	return nil
}

func (ms *StatsdMetrics) Stop() error {
	return ms.client.Close()
}
