package exec

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/StevenBuglione/oas-cli-go/pkg/catalog"
	"github.com/StevenBuglione/oas-cli-go/pkg/config"
	mcpclient "github.com/StevenBuglione/oas-cli-go/pkg/mcp/client"
)

type MCPRequest struct {
	Tool   catalog.Tool
	Source config.Source
	Body   []byte
}

func ExecuteMCP(ctx context.Context, request MCPRequest) (*Result, error) {
	client, err := mcpclient.Open(request.Source, ctx)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	args, err := decodeMCPArguments(request.Body)
	if err != nil {
		return nil, err
	}

	result, err := client.CallTool(ctx, request.Tool.OperationID, args)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &Result{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	}, nil
}

func decodeMCPArguments(body []byte) (any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var args any
	if err := json.Unmarshal(body, &args); err != nil {
		return nil, err
	}
	return args, nil
}
