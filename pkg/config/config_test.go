package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/pkg/config"
)

func writeJSON(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestLoadEffectiveMergesScopesAndPreservesManagedDenies(t *testing.T) {
	dir := t.TempDir()

	managedPath := writeJSON(t, dir, "managed.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "ticketsService": {
	      "type": "serviceRoot",
	      "uri": "https://managed.example.com/api",
	      "enabled": true
	    }
	  },
	  "curation": {
	    "toolSets": {
	      "sandbox-default": {
	        "allow": ["tickets:listTickets", "tickets:getTicket"],
	        "deny": ["tickets:deleteTicket"]
	      }
	    }
	  },
	  "policy": {
	    "deny": ["tickets:deleteTicket"]
	  }
	}`)
	userPath := writeJSON(t, dir, "user.json", `{
	  "sources": {
	    "ticketsService": { "enabled": false },
	    "billingService": {
	      "type": "openapi",
	      "uri": "file:///tmp/billing.openapi.json"
	    }
	  },
	  "policy": {
	    "deny": ["billing:deleteInvoice"]
	  }
	}`)
	projectPath := writeJSON(t, dir, "project.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "curated" },
	  "services": {
	    "tickets": {
	      "source": "ticketsService",
	      "alias": "tickets"
	    }
	  },
	  "agents": {
	    "profiles": {
	      "sandbox": {
	        "mode": "curated",
	        "toolSet": "sandbox-default"
	      }
	    },
	    "defaultProfile": "sandbox"
	  }
	}`)
	localPath := writeJSON(t, dir, "local.json", `{
	  "sources": {
	    "ticketsService": { "enabled": true }
	  },
	  "services": {
	    "tickets": {
	      "overlays": ["./tickets.overlay.yaml"]
	    }
	  }
	}`)

	effective, err := config.LoadEffective(config.LoadOptions{
		ManagedPath: managedPath,
		UserPath:    userPath,
		ProjectPath: projectPath,
		LocalPath:   localPath,
	})
	if err != nil {
		t.Fatalf("LoadEffective returned error: %v", err)
	}

	if effective.Config.Mode.Default != "curated" {
		t.Fatalf("expected curated default mode, got %q", effective.Config.Mode.Default)
	}
	if !effective.Config.Sources["ticketsService"].Enabled {
		t.Fatalf("expected ticketsService to be re-enabled by local scope")
	}
	if effective.Config.Sources["billingService"].URI != "file:///tmp/billing.openapi.json" {
		t.Fatalf("expected billing source to merge in from user scope")
	}
	if got := effective.Config.Policy.ManagedDeny; len(got) != 1 || got[0] != "tickets:deleteTicket" {
		t.Fatalf("expected managed deny to be preserved, got %#v", got)
	}
	if got := effective.Config.Policy.Deny; len(got) != 2 {
		t.Fatalf("expected 2 deny patterns after merge, got %#v", got)
	}
	if got := effective.Config.Services["tickets"].Overlays; len(got) != 1 || got[0] != "./tickets.overlay.yaml" {
		t.Fatalf("expected overlay override to merge, got %#v", got)
	}
	if effective.Config.Agents.DefaultProfile != "sandbox" {
		t.Fatalf("expected default profile sandbox, got %q", effective.Config.Agents.DefaultProfile)
	}
}

func TestLoadEffectiveReturnsFieldDiagnostics(t *testing.T) {
	dir := t.TempDir()
	projectPath := writeJSON(t, dir, "project.json", `{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "broken": {
	      "type": "openapi"
	    }
	  }
	}`)

	_, err := config.LoadEffective(config.LoadOptions{ProjectPath: projectPath})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	validationErr, ok := err.(*config.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(validationErr.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics")
	}
	if validationErr.Diagnostics[0].Path != "sources.broken.uri" {
		t.Fatalf("expected sources.broken.uri diagnostic, got %#v", validationErr.Diagnostics)
	}
}
