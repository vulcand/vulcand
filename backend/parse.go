package backend

import (
	"encoding/json"
	"fmt"
	"github.com/mailgun/vulcan/limit"
	"strings"
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

func VariableToMapper(variable string) (limit.MapperFn, error) {
	if variable == "client.ip" {
		return limit.MapClientIp, nil
	}
	if variable == "request.host" {
		return limit.MapRequestHost, nil
	}
	if strings.HasPrefix(variable, "request.header.") {
		header := strings.TrimPrefix(variable, "request.header.")
		if len(header) == 0 {
			return nil, fmt.Errorf("Wrong header: %s", header)
		}
		return limit.MakeMapRequestHeader(header), nil
	}
	return nil, fmt.Errorf("Unsupported limiting varuable: '%s'", variable)
}
