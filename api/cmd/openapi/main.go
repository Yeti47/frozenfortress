// Command openapi prints the OpenAPI spec of the API as YAML to stdout:
//
//	go run ./api/cmd/openapi > api/openapi.yaml
//
// It builds the router without any database, Redis or environment.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/gin-gonic/gin"
)

// Spec renders the OpenAPI document of the production router as YAML.
func Spec() ([]byte, error) {
	gin.SetMode(gin.ReleaseMode) // keep gin's route debug output off stdout, which carries the spec
	_, api, err := server.NewRouter(server.Deps{}, server.Options{})
	if err != nil {
		return nil, err
	}
	return api.OpenAPI().YAML()
}

func run(w io.Writer) error {
	spec, err := Spec()
	if err != nil {
		return err
	}
	_, err = w.Write(spec)
	return err
}

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
