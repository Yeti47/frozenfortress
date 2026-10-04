// Package bootstrap is the one place that decides which handlers the API serves. It lives
// outside package main so that both the server and cmd/openapi use the same list.
package bootstrap

import (
	"github.com/Yeti47/frozenfortress/frozenfortress/api/handlers/system"
	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
)

// Handlers returns every handler of the API, wired with the given services.
//
// cmd/openapi calls this with an empty Services to generate the spec, so handler
// constructors must only store their dependencies and never use them.
func Handlers(svc Services) []server.Registrar {
	return []server.Registrar{
		system.NewHandler(svc.Logger, svc.DB, svc.UpdateChecker),
	}
}
