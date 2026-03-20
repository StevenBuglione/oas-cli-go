package commands

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/StevenBuglione/open-cli/pkg/catalog"
)

// WriteDryRun prints the HTTP request that would be sent, without executing.
func WriteDryRun(w io.Writer, tool catalog.Tool, pathArgs []string, flags map[string]string, body []byte) {
	if tool.Backend != nil && tool.Backend.Kind == "mcp" {
		writeMCPDryRun(w, tool, body)
		return
	}

	path := tool.Path
	for i, param := range tool.PathParams {
		if i < len(pathArgs) {
			path = strings.ReplaceAll(path, "{"+param.OriginalName+"}", pathArgs[i])
		}
	}

	query := url.Values{}
	for _, param := range tool.Flags {
		if param.Location == "query" {
			if value := flags[param.Name]; value != "" {
				query.Set(param.OriginalName, value)
			}
		}
	}

	baseURL, baseResolved := dryRunBaseURL(tool)
	fullURL := "<base-unresolved>" + path
	if baseResolved {
		fullURL = baseURL + path
	}
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	fmt.Fprintf(w, "%s %s\n", strings.ToUpper(tool.Method), fullURL)
	if !baseResolved {
		fmt.Fprintln(w, "Base: unresolved")
	}
	fmt.Fprintf(w, "Auth: %s\n", dryRunAuthStatus(tool))
	fmt.Fprintf(w, "Approval: %s\n", dryRunApprovalStatus(tool))
	if len(body) > 0 {
		fmt.Fprintln(w, "Content-Type: application/json")
	}
	fmt.Fprintln(w)
	if len(body) > 0 {
		fmt.Fprintln(w, string(body))
	}
}

func dryRunBaseURL(tool catalog.Tool) (string, bool) {
	if len(tool.Servers) == 0 || strings.TrimSpace(tool.Servers[0]) == "" {
		return "", false
	}
	return tool.Servers[0], true
}

func dryRunAuthStatus(tool catalog.Tool) string {
	if len(tool.Auth) > 0 || len(tool.AuthAlternatives) > 0 {
		return "required"
	}
	return "not_required"
}

func dryRunApprovalStatus(tool catalog.Tool) string {
	if tool.Safety.RequiresApproval {
		return "required"
	}
	return "not_required"
}

func writeMCPDryRun(w io.Writer, tool catalog.Tool, body []byte) {
	toolName := tool.ID
	if tool.Backend != nil && strings.TrimSpace(tool.Backend.ToolName) != "" {
		toolName = tool.Backend.ToolName
	}
	fmt.Fprintf(w, "MCP %s\n", toolName)
	fmt.Fprintf(w, "Auth: %s\n", dryRunAuthStatus(tool))
	fmt.Fprintf(w, "Approval: %s\n", dryRunApprovalStatus(tool))
	if len(body) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, string(body))
	}
}
