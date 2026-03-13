package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/StevenBuglione/oas-cli-go/pkg/audit"
	"github.com/StevenBuglione/oas-cli-go/pkg/catalog"
	"github.com/StevenBuglione/oas-cli-go/pkg/config"
	httpexec "github.com/StevenBuglione/oas-cli-go/pkg/exec"
	"github.com/StevenBuglione/oas-cli-go/pkg/policy"
)

type Options struct {
	AuditPath  string
	HTTPClient *http.Client
}

type Server struct {
	auditStore *audit.FileStore
	client     *http.Client
}

type effectiveCatalogResponse struct {
	Catalog *catalog.NormalizedCatalog `json:"catalog"`
	View    *catalog.EffectiveView     `json:"view"`
}

type executeToolRequest struct {
	ConfigPath   string            `json:"configPath"`
	Mode         string            `json:"mode,omitempty"`
	AgentProfile string            `json:"agentProfile,omitempty"`
	ToolID       string            `json:"toolId"`
	PathArgs     []string          `json:"pathArgs,omitempty"`
	Flags        map[string]string `json:"flags,omitempty"`
	Body         []byte            `json:"body,omitempty"`
	Approval     bool              `json:"approval,omitempty"`
}

type executeToolResponse struct {
	StatusCode int             `json:"statusCode"`
	Body       json.RawMessage `json:"body,omitempty"`
	Text       string          `json:"text,omitempty"`
}

type workflowRunRequest struct {
	ConfigPath   string `json:"configPath"`
	Mode         string `json:"mode,omitempty"`
	AgentProfile string `json:"agentProfile,omitempty"`
	WorkflowID   string `json:"workflowId"`
	Approval     bool   `json:"approval,omitempty"`
}

func NewServer(options Options) *Server {
	if options.AuditPath == "" {
		options.AuditPath = filepath.Join(".cache", "audit.log")
	}
	if options.HTTPClient == nil {
		options.HTTPClient = http.DefaultClient
	}
	return &Server{
		auditStore: audit.NewFileStore(options.AuditPath),
		client:     options.HTTPClient,
	}
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/catalog/effective", server.handleEffectiveCatalog)
	mux.HandleFunc("/v1/tools/execute", server.handleExecuteTool)
	mux.HandleFunc("/v1/workflows/run", server.handleWorkflowRun)
	mux.HandleFunc("/v1/refresh", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/v1/audit/events", server.handleAuditEvents)
	return mux
}

func (server *Server) handleEffectiveCatalog(w http.ResponseWriter, r *http.Request) {
	cfg, ntc, err := server.loadCatalog(r.Context(), r.URL.Query().Get("config"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mode := r.URL.Query().Get("mode")
	agentProfile := r.URL.Query().Get("agentProfile")
	view := selectView(cfg.Config, ntc, mode, agentProfile)
	writeJSON(w, http.StatusOK, effectiveCatalogResponse{Catalog: ntc, View: view})
}

func (server *Server) handleExecuteTool(w http.ResponseWriter, r *http.Request) {
	var request executeToolRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, ntc, err := server.loadCatalog(r.Context(), request.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tool := ntc.FindTool(request.ToolID)
	if tool == nil {
		http.Error(w, "tool not found", http.StatusNotFound)
		return
	}

	decision := policy.Decide(cfg.Config, *tool, policy.Context{
		Mode:            request.Mode,
		AgentProfile:    request.AgentProfile,
		ApprovalGranted: request.Approval,
	})
	if !decision.Allowed {
		server.recordEvent(*tool, request.AgentProfile, decision, 0, 0, 0)
		http.Error(w, decision.ReasonCode, http.StatusForbidden)
		return
	}

	start := time.Now()
	result, err := httpexec.Execute(r.Context(), server.client, httpexec.Request{
		Tool:     *tool,
		PathArgs: request.PathArgs,
		Flags:    request.Flags,
		Body:     request.Body,
		Auth:     resolveAuth(cfg.Config, *tool),
	})
	if err != nil {
		server.recordEvent(*tool, request.AgentProfile, policy.Decision{Allowed: false, ReasonCode: "execution_error"}, 0, 0, 0)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	server.recordEvent(*tool, request.AgentProfile, decision, result.StatusCode, result.RetryCount, time.Since(start))

	response := executeToolResponse{StatusCode: result.StatusCode}
	if json.Valid(result.Body) {
		response.Body = append([]byte(nil), result.Body...)
	} else {
		response.Text = string(result.Body)
	}
	writeJSON(w, http.StatusOK, response)
}

func (server *Server) handleWorkflowRun(w http.ResponseWriter, r *http.Request) {
	var request workflowRunRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, ntc, err := server.loadCatalog(r.Context(), request.ConfigPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, workflow := range ntc.Workflows {
		if workflow.WorkflowID != request.WorkflowID {
			continue
		}
		var steps []string
		for _, step := range workflow.Steps {
			tool := ntc.FindTool(step.ToolID)
			if tool == nil {
				http.Error(w, "workflow references unknown tool", http.StatusBadRequest)
				return
			}
			decision := policy.Decide(cfg.Config, *tool, policy.Context{
				Mode:            request.Mode,
				AgentProfile:    request.AgentProfile,
				ApprovalGranted: request.Approval,
			})
			if !decision.Allowed {
				http.Error(w, decision.ReasonCode, http.StatusForbidden)
				return
			}
			steps = append(steps, step.StepID)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workflowId": workflow.WorkflowID,
			"steps":      steps,
		})
		return
	}

	http.Error(w, "workflow not found", http.StatusNotFound)
}

func (server *Server) handleAuditEvents(w http.ResponseWriter, _ *http.Request) {
	events, err := server.auditStore.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (server *Server) loadCatalog(ctx context.Context, configPath string) (*config.EffectiveConfig, *catalog.NormalizedCatalog, error) {
	if configPath == "" {
		return nil, nil, fmt.Errorf("config query parameter is required")
	}
	if _, err := os.Stat(configPath); err != nil {
		return nil, nil, err
	}

	cfg, err := config.LoadEffective(config.LoadOptions{ProjectPath: configPath})
	if err != nil {
		return nil, nil, err
	}
	ntc, err := catalog.Build(ctx, catalog.BuildOptions{
		Config:  cfg.Config,
		BaseDir: cfg.BaseDir,
	})
	if err != nil {
		return nil, nil, err
	}
	return cfg, ntc, nil
}

func selectView(cfg config.Config, ntc *catalog.NormalizedCatalog, mode, agentProfile string) *catalog.EffectiveView {
	if agentProfile != "" {
		if view := ntc.EffectiveView(agentProfile); view != nil {
			return view
		}
	}
	if mode == "" {
		mode = cfg.Mode.Default
	}
	if mode == "curated" && cfg.Agents.DefaultProfile != "" {
		if view := ntc.EffectiveView(cfg.Agents.DefaultProfile); view != nil {
			return view
		}
	}
	return ntc.EffectiveView("discover")
}

func (server *Server) recordEvent(tool catalog.Tool, agentProfile string, decision policy.Decision, statusCode, retryCount int, latency time.Duration) {
	_ = server.auditStore.Append(audit.Event{
		Timestamp:     time.Now().UTC(),
		AgentProfile:  agentProfile,
		ToolID:        tool.ID,
		ServiceID:     tool.ServiceID,
		TargetBaseURL: first(tool.Servers),
		Decision:      map[bool]string{true: "allowed", false: "denied"}[decision.Allowed],
		ReasonCode:    decision.ReasonCode,
		Method:        tool.Method,
		Path:          tool.Path,
		RequestSize:   0,
		StatusCode:    statusCode,
		RetryCount:    retryCount,
		LatencyMS:     latency.Milliseconds(),
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func resolveAuth(cfg config.Config, tool catalog.Tool) []httpexec.AuthScheme {
	var auth []httpexec.AuthScheme
	for _, requirement := range tool.Auth {
		secret, ok := cfg.Secrets[requirement.Name]
		if !ok {
			continue
		}
		value, err := resolveSecret(cfg.Policy, secret)
		if err != nil {
			continue
		}
		auth = append(auth, httpexec.AuthScheme{
			Type:   requirement.Type,
			Scheme: requirement.Scheme,
			In:     requirement.In,
			Name:   requirement.ParamName,
			Value:  value,
		})
	}
	return auth
}

func resolveSecret(policyConfig config.PolicyConfig, secret config.SecretRef) (string, error) {
	switch secret.Type {
	case "env":
		return os.Getenv(secret.Value), nil
	case "file":
		data, err := os.ReadFile(secret.Value)
		return string(data), err
	case "exec":
		if !policyConfig.AllowExecSecrets {
			return "", fmt.Errorf("exec secrets are disabled")
		}
		output, err := exec.Command(secret.Value).Output()
		return string(output), err
	default:
		return "", fmt.Errorf("unsupported secret type %q", secret.Type)
	}
}
