package main

import (
	"github.com/mailgun/vulcand/plugin"
	"github.com/mailgun/vulcand/plugin/connlimit"
	"github.com/mailgun/vulcand/plugin/ratelimit"
	"github.com/mailgun/vulcand/plugin/rewrite"
)

func GetRegistry() (*plugin.Registry, error) {
	r := plugin.NewRegistry()

	if err := r.AddSpec(ratelimit.GetSpec()); err != nil {
		return nil, err
	}

	if err := r.AddSpec(connlimit.GetSpec()); err != nil {
		return nil, err
	}

	if err := r.AddSpec(rewrite.GetSpec()); err != nil {
		return nil, err
	}
	return r, nil
}
