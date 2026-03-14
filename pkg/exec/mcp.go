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

	args, err := decodeMCPArguments(request.Tool, request.Body)
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

func decodeMCPArguments(tool catalog.Tool, body []byte) (any, error) {
	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var args any
	if err := json.Unmarshal(body, &args); err != nil {
		return nil, err
	}
	if shouldUnwrapMCPInput(tool, args) {
		return args.(map[string]any)["input"], nil
	}
	return args, nil
}

func shouldUnwrapMCPInput(tool catalog.Tool, args any) bool {
	objectArgs, ok := args.(map[string]any)
	if !ok {
		return false
	}
	if len(objectArgs) != 1 {
		return false
	}
	if _, ok := objectArgs["input"]; !ok {
		return false
	}
	if tool.RequestBody == nil || len(tool.RequestBody.ContentTypes) == 0 {
		return false
	}
	schema := tool.RequestBody.ContentTypes[0].Schema
	if schema == nil {
		return false
	}
	if schemaType, _ := schema["type"].(string); schemaType != "object" {
		return false
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || len(properties) != 1 {
		return false
	}
	if _, ok := properties["input"]; !ok {
		return false
	}
	required, ok := schema["required"].([]any)
	return ok && len(required) == 1 && required[0] == "input"
}
