package api

import (
	"errors"
)

// common state for the entire api package.

var Endpoint         string
var FilteredEndpoint string
var StaticPrefix     string

type settings interface {
	GetApiEndpoint() string
	GetApiFilteredEndpoint() string
	GetApiStaticPrefix() string
}

func Init(s settings) error {
	Endpoint = s.GetApiEndpoint()
	FilteredEndpoint = s.GetApiFilteredEndpoint()
	StaticPrefix = s.GetApiStaticPrefix()

	if Endpoint == "" || FilteredEndpoint == "" || StaticPrefix == "" {
		return errors.New("missing required parameter")
	}

	return nil
}
