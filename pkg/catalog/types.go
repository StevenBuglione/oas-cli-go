package catalog

import (
	"net/http"
	"time"
)

type BuildOptions struct {
	Config     any
	BaseDir    string
	HTTPClient *http.Client
}

type NormalizedCatalog struct {
	CatalogVersion    string          `json:"catalogVersion"`
	GeneratedAt       time.Time       `json:"generatedAt"`
	SourceFingerprint string          `json:"sourceFingerprint"`
	Services          []Service       `json:"services"`
	Tools             []Tool          `json:"tools"`
	Workflows         []Workflow      `json:"workflows,omitempty"`
	EffectiveViews    []EffectiveView `json:"effectiveViews"`
}

type Service struct {
	ID       string   `json:"id"`
	Alias    string   `json:"alias"`
	SourceID string   `json:"sourceId"`
	Title    string   `json:"title"`
	Servers  []string `json:"servers,omitempty"`
}

type Parameter struct {
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	Location     string `json:"location"`
	Required     bool   `json:"required"`
}

type Safety struct {
	Destructive      bool `json:"destructive"`
	ReadOnly         bool `json:"readOnly"`
	RequiresApproval bool `json:"requiresApproval"`
}

type Guidance struct {
	WhenToUse []string `json:"whenToUse,omitempty"`
	AvoidWhen []string `json:"avoidWhen,omitempty"`
}

type Tool struct {
	ID          string            `json:"id"`
	ServiceID   string            `json:"serviceId"`
	OperationID string            `json:"operationId,omitempty"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Group       string            `json:"group"`
	Command     string            `json:"command"`
	Summary     string            `json:"summary,omitempty"`
	PathParams  []Parameter       `json:"pathParams,omitempty"`
	Flags       []Parameter       `json:"flags,omitempty"`
	Auth        []AuthRequirement `json:"auth,omitempty"`
	Safety      Safety            `json:"safety"`
	Guidance    *Guidance         `json:"guidance,omitempty"`
	Servers     []string          `json:"servers,omitempty"`
}

type AuthRequirement struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Scheme    string `json:"scheme,omitempty"`
	In        string `json:"in,omitempty"`
	ParamName string `json:"paramName,omitempty"`
}

type Workflow struct {
	WorkflowID string         `json:"workflowId"`
	Steps      []WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	StepID string `json:"stepId"`
	ToolID string `json:"toolId"`
}

type EffectiveView struct {
	Name  string `json:"name"`
	Mode  string `json:"mode"`
	Tools []Tool `json:"tools"`
}

func (catalog *NormalizedCatalog) FindTool(id string) *Tool {
	for idx := range catalog.Tools {
		if catalog.Tools[idx].ID == id {
			return &catalog.Tools[idx]
		}
	}
	return nil
}

func (catalog *NormalizedCatalog) EffectiveView(name string) *EffectiveView {
	for idx := range catalog.EffectiveViews {
		if catalog.EffectiveViews[idx].Name == name {
			return &catalog.EffectiveViews[idx]
		}
	}
	return nil
}
