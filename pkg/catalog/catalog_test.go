package catalog_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/pkg/catalog"
	"github.com/StevenBuglione/oas-cli-go/pkg/config"
)

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestBuildProducesStableToolCatalogAndEffectiveViews(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, dir, "tickets.openapi.yaml", `
openapi: 3.1.0
info:
  title: Example Tickets API
  version: "2026-03-01"
servers:
  - url: https://api.example.com/v1
paths:
  /tickets:
    get:
      operationId: listTickets
      tags: [tickets]
      summary: List tickets
      parameters:
        - name: status
          in: query
          schema:
            type: string
      responses:
        "200":
          description: OK
  /tickets/{id}:
    get:
      operationId: getTicket
      tags: [tickets]
      summary: Get a ticket
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: OK
`)
	writeFile(t, dir, "overlays/tickets.overlay.yaml", `
overlay: 1.1.0
actions:
  - target: "$.paths['/tickets'].get"
    update:
      x-cli-name: list
      x-cli-safety:
        readOnly: true
        destructive: false
        requiresApproval: false
  - target: "$.paths['/tickets'].get.parameters[?(@.name=='status')]"
    update:
      x-cli-name: state
`)
	writeFile(t, dir, "skills/tickets.skill.json", `{
	  "oasCliSkill": "1.0.0",
	  "serviceId": "tickets",
	  "summary": "Guidance for using the Tickets API via OAS-CLI",
	  "toolGuidance": {
	    "tickets:listTickets": {
	      "whenToUse": ["Need to enumerate recent tickets"]
	    }
	  }
	}`)
	writeFile(t, dir, "workflows/tickets.arazzo.yaml", `
arazzo: 1.0.0
info:
  title: Ticket workflows
  version: 1.0.0
workflows:
  - workflowId: triageTicket
    steps:
      - stepId: list
        operationId: listTickets
      - stepId: fetch
        operationId: getTicket
`)

	cfg := config.Config{
		CLI:  "1.0.0",
		Mode: config.ModeConfig{Default: "discover"},
		Sources: map[string]config.Source{
			"ticketsSource": {
				Type:    "openapi",
				URI:     filepath.ToSlash(filepath.Join(dir, "tickets.openapi.yaml")),
				Enabled: true,
			},
		},
		Services: map[string]config.Service{
			"tickets": {
				Source:    "ticketsSource",
				Alias:     "tickets",
				Overlays:  []string{"./overlays/tickets.overlay.yaml"},
				Skills:    []string{"./skills/tickets.skill.json"},
				Workflows: []string{"./workflows/tickets.arazzo.yaml"},
			},
		},
		Curation: config.CurationConfig{
			ToolSets: map[string]config.ToolSet{
				"sandbox-default": {
					Allow: []string{"tickets:listTickets", "tickets:getTicket"},
					Deny:  []string{"**"},
				},
			},
		},
		Agents: config.AgentsConfig{
			DefaultProfile: "sandbox",
			Profiles: map[string]config.AgentProfile{
				"sandbox": {
					Mode:    "curated",
					ToolSet: "sandbox-default",
				},
			},
		},
	}

	ntc, err := catalog.Build(context.Background(), catalog.BuildOptions{
		Config:  cfg,
		BaseDir: dir,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if len(ntc.Services) != 1 {
		t.Fatalf("expected 1 service, got %#v", ntc.Services)
	}
	if len(ntc.Sources) != 1 {
		t.Fatalf("expected 1 source provenance record, got %#v", ntc.Sources)
	}
	if ntc.Sources[0].ID != "ticketsSource" || ntc.Sources[0].Provenance.Method != "explicit" {
		t.Fatalf("expected explicit source provenance for ticketsSource, got %#v", ntc.Sources[0])
	}
	if len(ntc.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %#v", ntc.Tools)
	}

	listTool := ntc.FindTool("tickets:listTickets")
	if listTool == nil {
		t.Fatalf("expected listTickets tool")
	}
	if listTool.Group != "tickets" || listTool.Command != "list" {
		t.Fatalf("unexpected command mapping: %#v", listTool)
	}
	if len(listTool.Flags) != 1 || listTool.Flags[0].Name != "state" {
		t.Fatalf("expected renamed state flag, got %#v", listTool.Flags)
	}
	if !listTool.Safety.ReadOnly {
		t.Fatalf("expected readOnly safety metadata")
	}
	if listTool.Guidance == nil || len(listTool.Guidance.WhenToUse) != 1 {
		t.Fatalf("expected tool guidance to merge in, got %#v", listTool.Guidance)
	}
	if len(ntc.Workflows) != 1 || ntc.Workflows[0].WorkflowID != "triageTicket" {
		t.Fatalf("expected workflow to load, got %#v", ntc.Workflows)
	}
	if ntc.SourceFingerprint == "" {
		t.Fatalf("expected source fingerprint")
	}

	curated := ntc.EffectiveView("sandbox")
	if curated == nil {
		t.Fatalf("expected sandbox effective view")
	}
	if len(curated.Tools) != 2 {
		t.Fatalf("expected 2 curated tools, got %#v", curated.Tools)
	}

	if _, err := json.Marshal(ntc); err != nil {
		t.Fatalf("catalog should be json serializable: %v", err)
	}
}
