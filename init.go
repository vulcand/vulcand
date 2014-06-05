package main

import (
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/ratelimit"
)

func GetRegistry() (*plugin.Registry, error) {
	r := plugin.NewRegistry()

	if err := r.AddSpec(ratelimit.GetSpec()); err != nil {
		return nil, err
	}

	if err := r.AddSpec(connlimit.GetSpec()); err != nil {
		return nil, err
	}
	return r, nil
}
