package main

import (
	"context"
	"encoding/json"
	"os"

	"github.com/sqlc-dev/plugin-sdk-go/codegen"
	"github.com/sqlc-dev/plugin-sdk-go/plugin"
)

func main() {
	codegen.Run(func(ctx context.Context, req *plugin.GenerateRequest) (*plugin.GenerateResponse, error) {
		// Dump the entire request as JSON for inspection
		data, err := json.MarshalIndent(req, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile("spike_dump.json", data, 0644); err != nil {
			return nil, err
		}

		// Return empty response — no files generated
		return &plugin.GenerateResponse{}, nil
	})
}
