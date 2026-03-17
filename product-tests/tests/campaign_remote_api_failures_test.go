package tests_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenBuglione/oas-cli-go/internal/runtime"
	"github.com/StevenBuglione/oas-cli-go/pkg/audit"
	"github.com/StevenBuglione/oas-cli-go/product-tests/tests/helpers"
)

func TestCampaignRemoteAPIFailures(t *testing.T) {
	api := httptest.NewServer(newRestFixtureHandler(newFixtureStore()))
	t.Cleanup(api.Close)

	dir := t.TempDir()
	openapiPath := writeFile(t, dir, "remote-api-failures.openapi.yaml", restOpenAPIYAML(api.URL))
	configPath := writeFile(t, dir, "remote-api-failures.cli.json", restCLIConfig(openapiPath))

	runtimeSrv := runtime.NewServer(runtime.Options{
		AuditPath: filepath.Join(dir, "audit.log"),
	})
	srv := httptest.NewServer(runtimeSrv.Handler())
	t.Cleanup(srv.Close)

	fr := helpers.NewFindingsRecorder("remote-api-failures")
	fr.SetLaneMetadata("product-validation", "remote-api", "ci-containerized", "none")
	defer fr.MustEmitToTest(t)

	t.Run("upstream-forbidden", func(t *testing.T) {
		result := executeTool(t, srv.URL, configPath, "testapi:triggerForbidden", nil)
		statusCode, _ := result["statusCode"].(float64)
		fr.Check("forbidden-status", "forbidden upstream response is surfaced", "403", fmt.Sprintf("%.0f", statusCode), statusCode == 403, "")

		events := (&helpers.Instance{URL: srv.URL, AuditPath: filepath.Join(dir, "audit.log")}).AuditEvents(t)
		event := findAuditEvent(events, "testapi:triggerForbidden")
		fr.CheckBool("forbidden-audit-present", "forbidden execution is recorded in the audit log", event != nil, "")
		if event != nil {
			fr.Check("forbidden-audit-status", "forbidden audit event records the upstream status", "403", fmt.Sprintf("%d", event.StatusCode), event.StatusCode == 403, "")
			fr.Check("forbidden-audit-decision", "forbidden upstream execution remains an allowed runtime decision", "allowed", event.Decision, event.Decision == "allowed", "")
		}
	})

	t.Run("upstream-internal-error", func(t *testing.T) {
		result := executeTool(t, srv.URL, configPath, "testapi:triggerInternalError", nil)
		statusCode, _ := result["statusCode"].(float64)
		fr.Check("internal-status", "internal upstream response is surfaced", "500", fmt.Sprintf("%.0f", statusCode), statusCode == 500, "")

		events := (&helpers.Instance{URL: srv.URL, AuditPath: filepath.Join(dir, "audit.log")}).AuditEvents(t)
		event := findAuditEvent(events, "testapi:triggerInternalError")
		fr.CheckBool("internal-audit-present", "internal-error execution is recorded in the audit log", event != nil, "")
		if event != nil {
			fr.Check("internal-audit-status", "internal-error audit event records the upstream status", "500", fmt.Sprintf("%d", event.StatusCode), event.StatusCode == 500, "")
		}
	})

	t.Run("non-json-upstream-response", func(t *testing.T) {
		textAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/plaintext" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("plain failure body"))
		}))
		t.Cleanup(textAPI.Close)

		textSpec := writeFile(t, dir, "plaintext.openapi.yaml", `openapi: 3.1.0
info:
  title: Plaintext API
  version: "1.0.0"
servers:
  - url: `+textAPI.URL+`
paths:
  /plaintext:
    get:
      operationId: getPlaintext
      tags: [plaintext]
      responses:
        "502":
          description: Bad Gateway
`)
		textConfig := writeFile(t, dir, "plaintext.cli.json", restCLIConfig(textSpec))

		payload, _ := json.Marshal(map[string]any{"configPath": textConfig, "toolId": "testapi:getPlaintext"})
		resp, err := http.Post(srv.URL+"/v1/tools/execute", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("execute plaintext tool: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode plaintext execute response: %v", err)
		}
		statusCode, _ := result["statusCode"].(float64)
		fr.Check("plaintext-status", "non-JSON upstream status is surfaced", "502", fmt.Sprintf("%.0f", statusCode), statusCode == 502, "")
		body, _ := result["body"].(string)
		fr.Check("plaintext-body-omitted", "non-JSON upstream body is currently omitted from the structured response", "", strings.TrimSpace(body), strings.TrimSpace(body) == "", "plaintext body preservation remains a separate gap to address later")
	})
}

func findAuditEvent(events []audit.Event, toolID string) *audit.Event {
	for i := range events {
		if events[i].ToolID == toolID {
			return &events[i]
		}
	}
	return nil
}
