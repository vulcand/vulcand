package etcdbackend

import (
	"encoding/json"
	. "github.com/mailgun/vulcand/backend"
)

func ParseConnLimit(in string) (*ConnLimit, error) {
	var conn *ConnLimit
	err := json.Unmarshal([]byte(in), &conn)
	if err != nil {
		return nil, err
	}
	return NewConnLimit(conn.Connections, conn.Variable)
}

func ParseRateLimit(in string) (*RateLimit, error) {
	var rate *RateLimit
	err := json.Unmarshal([]byte(in), &rate)
	if err != nil {
		return nil, err
	}
	return NewRateLimit(rate.Requests, rate.Variable, rate.Burst, rate.PeriodSeconds)
}
