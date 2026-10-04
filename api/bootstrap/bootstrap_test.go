package bootstrap

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Yeti47/frozenfortress/frozenfortress/api/server"
	"github.com/danielgtaylor/huma/v2"
)

func operationIDs(item *huma.PathItem) []string {
	var ids []string
	for _, op := range []*huma.Operation{item.Get, item.Put, item.Post, item.Delete, item.Patch, item.Head, item.Options, item.Trace} {
		if op != nil {
			ids = append(ids, op.OperationID)
		}
	}
	return ids
}

// TestHandlersWithEmptyServices guards the contract cmd/openapi relies on: constructing and
// registering every handler works without any service. Extend the expected list with each new handler.
func TestHandlersWithEmptyServices(t *testing.T) {
	_, api, err := server.NewRouter(nil, server.Options{}, Handlers(Services{})...)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, item := range api.OpenAPI().Paths {
		got = append(got, operationIDs(item)...)
	}
	sort.Strings(got)

	want := []string{"getHealth", "getInfo"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("operations = %v, want %v", got, want)
	}
}
