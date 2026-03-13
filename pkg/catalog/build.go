package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/StevenBuglione/oas-cli-go/pkg/config"
	"github.com/StevenBuglione/oas-cli-go/pkg/discovery"
	"github.com/StevenBuglione/oas-cli-go/pkg/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type skillManifest struct {
	ToolGuidance map[string]Guidance `json:"toolGuidance"`
}

type workflowDocument struct {
	Workflows []workflowSpec `json:"workflows" yaml:"workflows"`
}

type workflowSpec struct {
	WorkflowID string             `json:"workflowId" yaml:"workflowId"`
	Steps      []workflowStepSpec `json:"steps" yaml:"steps"`
}

type workflowStepSpec struct {
	StepID        string `json:"stepId" yaml:"stepId"`
	OperationID   string `json:"operationId" yaml:"operationId"`
	OperationPath string `json:"operationPath" yaml:"operationPath"`
}

func Build(ctx context.Context, options BuildOptions) (*NormalizedCatalog, error) {
	cfg, ok := options.Config.(config.Config)
	if !ok {
		return nil, fmt.Errorf("build options require config.Config")
	}

	catalog := &NormalizedCatalog{
		CatalogVersion: "1.0.0",
		GeneratedAt:    time.Now().UTC(),
	}
	fingerprint := sha256.New()
	recordedSources := map[string]struct{}{}

	referencedSources := map[string]bool{}
	serviceIDs := sortedKeys(cfg.Services)
	for _, serviceID := range serviceIDs {
		serviceConfig := cfg.Services[serviceID]
		sourceConfig, ok := cfg.Sources[serviceConfig.Source]
		if !ok || !sourceConfig.Enabled {
			continue
		}
		referencedSources[serviceConfig.Source] = true
		recordSource(catalog, recordedSources, serviceConfig.Source, sourceConfig.Type, sourceConfig.URI, provenanceMethodForSourceType(sourceConfig.Type))
		if err := buildServiceCatalog(ctx, catalog, &cfg, options.BaseDir, serviceID, serviceConfig, sourceConfig, fingerprint); err != nil {
			return nil, err
		}
	}

	for sourceID, sourceConfig := range cfg.Sources {
		if referencedSources[sourceID] || !sourceConfig.Enabled {
			continue
		}
		switch sourceConfig.Type {
		case "apiCatalog":
			recordSource(catalog, recordedSources, sourceID, sourceConfig.Type, sourceConfig.URI, string(discovery.ProvenanceRFC9727))
			result, err := discovery.DiscoverAPICatalog(ctx, options.HTTPClient, sourceConfig.URI)
			if err != nil {
				return nil, err
			}
			for _, discoveredService := range result.Services {
				discoveredConfig := config.Service{Source: sourceID}
				if err := buildServiceCatalog(ctx, catalog, &cfg, options.BaseDir, "", discoveredConfig, config.Source{
					Type:    "serviceRoot",
					URI:     discoveredService.URL,
					Enabled: true,
				}, fingerprint); err != nil {
					return nil, err
				}
			}
		case "serviceRoot":
			recordSource(catalog, recordedSources, sourceID, sourceConfig.Type, sourceConfig.URI, string(discovery.ProvenanceRFC8631))
			if err := buildServiceCatalog(ctx, catalog, &cfg, options.BaseDir, "", config.Service{Source: sourceID}, sourceConfig, fingerprint); err != nil {
				return nil, err
			}
		case "openapi":
			recordSource(catalog, recordedSources, sourceID, sourceConfig.Type, sourceConfig.URI, string(discovery.ProvenanceExplicit))
			if err := buildServiceCatalog(ctx, catalog, &cfg, options.BaseDir, "", config.Service{Source: sourceID}, sourceConfig, fingerprint); err != nil {
				return nil, err
			}
		}
	}

	sort.Slice(catalog.Tools, func(i, j int) bool {
		return catalog.Tools[i].ID < catalog.Tools[j].ID
	})

	catalog.SourceFingerprint = hex.EncodeToString(fingerprint.Sum(nil))
	catalog.EffectiveViews = buildEffectiveViews(cfg, catalog.Tools)
	return catalog, nil
}

func recordSource(ntc *NormalizedCatalog, recorded map[string]struct{}, id, sourceType, uri, method string) {
	if _, ok := recorded[id]; ok {
		return
	}
	recorded[id] = struct{}{}
	ntc.Sources = append(ntc.Sources, SourceRecord{
		ID:   id,
		Type: sourceType,
		URI:  uri,
		Provenance: SourceProvenance{
			Method: method,
			At:     time.Now().UTC(),
		},
	})
}

func provenanceMethodForSourceType(sourceType string) string {
	switch sourceType {
	case "apiCatalog":
		return string(discovery.ProvenanceRFC9727)
	case "serviceRoot":
		return string(discovery.ProvenanceRFC8631)
	default:
		return string(discovery.ProvenanceExplicit)
	}
}

func loadGuidance(baseDir string, refs []string) (map[string]Guidance, error) {
	guidance := map[string]Guidance{}
	for _, ref := range refs {
		data, err := openapi.ReadReference(context.Background(), openapi.ResolveReference(baseDir, ref))
		if err != nil {
			return nil, err
		}

		var manifest skillManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		for key, value := range manifest.ToolGuidance {
			guidance[key] = value
		}
	}
	return guidance, nil
}

func loadWorkflows(baseDir string, refs []string, bindings map[string]string) ([]Workflow, error) {
	var workflows []Workflow
	for _, ref := range refs {
		data, err := openapi.ReadReference(context.Background(), openapi.ResolveReference(baseDir, ref))
		if err != nil {
			return nil, err
		}

		var document workflowDocument
		if err := yaml.Unmarshal(data, &document); err != nil {
			return nil, err
		}
		for _, workflow := range document.Workflows {
			current := Workflow{WorkflowID: workflow.WorkflowID}
			for _, step := range workflow.Steps {
				toolID := bindings[step.OperationID]
				if toolID == "" {
					toolID = bindings[step.OperationPath]
				}
				current.Steps = append(current.Steps, WorkflowStep{
					StepID: step.StepID,
					ToolID: toolID,
				})
			}
			workflows = append(workflows, current)
		}
	}
	return workflows, nil
}

func buildTools(service Service, document *openapi3.T, guidance map[string]Guidance, bindings map[string]string) ([]Tool, error) {
	var tools []Tool
	paths := document.Paths.Map()
	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	for _, rawPath := range sortedPaths {
		item := paths[rawPath]
		ops := []struct {
			method string
			op     *openapi3.Operation
		}{
			{method: httpMethod("GET"), op: item.Get},
			{method: httpMethod("POST"), op: item.Post},
			{method: httpMethod("PUT"), op: item.Put},
			{method: httpMethod("PATCH"), op: item.Patch},
			{method: httpMethod("DELETE"), op: item.Delete},
		}
		for _, entry := range ops {
			if entry.op == nil {
				continue
			}
			operationID := entry.op.OperationID
			if operationID == "" {
				operationID = strings.ToLower(entry.method) + ":" + rawPath
			}
			toolID := service.ID + ":" + operationID
			bindings[operationID] = toolID
			bindings[entry.method+" "+rawPath] = toolID

			group := operationExtension(entry.op, "x-cli-group")
			if group == "" && len(entry.op.Tags) > 0 {
				group = slugify(entry.op.Tags[0])
			}
			if group == "" {
				group = firstPathSegment(rawPath)
			}
			if group == "" {
				group = "misc"
			}

			command := operationExtension(entry.op, "x-cli-name")
			if command == "" {
				command = normalizeCommandName(operationID)
			}
			if command == "" {
				command = inferCommand(entry.method, rawPath)
			}

			pathParams, flags := extractParameters(item.Parameters, entry.op.Parameters)
			safety := deriveSafety(entry.method, entry.op)

			tool := Tool{
				ID:          toolID,
				ServiceID:   service.ID,
				OperationID: entry.op.OperationID,
				Method:      entry.method,
				Path:        rawPath,
				Group:       group,
				Command:     command,
				Summary:     entry.op.Summary,
				PathParams:  pathParams,
				Flags:       flags,
				Auth:        extractAuth(document, entry.op),
				Safety:      safety,
				Servers:     service.Servers,
			}
			if currentGuidance, ok := guidance[tool.ID]; ok {
				tool.Guidance = &currentGuidance
			}
			tools = append(tools, tool)
		}
	}

	return tools, nil
}

func buildServiceCatalog(ctx context.Context, ntc *NormalizedCatalog, cfg *config.Config, baseDir, serviceID string, serviceConfig config.Service, sourceConfig config.Source, fingerprint hashWriter) error {
	openapiRef, metadataRefs, err := resolveServiceSource(ctx, baseDir, sourceConfig)
	if err != nil {
		return err
	}

	serviceConfig.Overlays = append([]string(nil), serviceConfig.Overlays...)
	serviceConfig.Skills = uniqueRefs(serviceConfig.Skills, metadataRefs.skills)
	serviceConfig.Workflows = uniqueRefs(serviceConfig.Workflows, metadataRefs.workflows)
	serviceConfig.Overlays = uniqueRefs(serviceConfig.Overlays, metadataRefs.overlays)

	document, err := openapi.LoadDocument(ctx, baseDir, openapiRef, serviceConfig.Overlays)
	if err != nil {
		return err
	}
	fingerprint.Write([]byte(document.Fingerprint))

	if serviceID == "" {
		serviceID = deriveServiceID(document.Document, sourceConfig.URI, sourceConfig.Type)
	}

	guidance, err := loadGuidance(baseDir, serviceConfig.Skills)
	if err != nil {
		return err
	}
	alias := serviceConfig.Alias
	if alias == "" {
		alias = serviceID
	}
	service := Service{
		ID:       serviceID,
		Alias:    alias,
		SourceID: serviceConfig.Source,
		Title:    document.Document.Info.Title,
		Servers:  extractServers(document.Document),
	}
	ntc.Services = append(ntc.Services, service)

	operationBindings := map[string]string{}
	tools, err := buildTools(service, document.Document, guidance, operationBindings)
	if err != nil {
		return err
	}
	ntc.Tools = append(ntc.Tools, tools...)

	workflows, err := loadWorkflows(baseDir, serviceConfig.Workflows, operationBindings)
	if err != nil {
		return err
	}
	ntc.Workflows = append(ntc.Workflows, workflows...)
	return nil
}

type metadataReferences struct {
	overlays  []string
	skills    []string
	workflows []string
}

func resolveServiceSource(ctx context.Context, baseDir string, source config.Source) (string, metadataReferences, error) {
	switch source.Type {
	case "openapi":
		return source.URI, metadataReferences{}, nil
	case "serviceRoot":
		result, err := discovery.DiscoverServiceRoot(ctx, nil, source.URI)
		if err != nil {
			return "", metadataReferences{}, err
		}
		refs, err := loadMetadataReferences(ctx, result.MetadataURL)
		if err != nil {
			return "", metadataReferences{}, err
		}
		return result.OpenAPIURL, refs, nil
	case "apiCatalog":
		return "", metadataReferences{}, fmt.Errorf("apiCatalog sources must be expanded before resolution")
	default:
		return "", metadataReferences{}, fmt.Errorf("unsupported source type %q", source.Type)
	}
}

func loadMetadataReferences(ctx context.Context, ref string) (metadataReferences, error) {
	if ref == "" {
		return metadataReferences{}, nil
	}
	data, err := openapi.ReadReference(ctx, ref)
	if err != nil {
		return metadataReferences{}, err
	}

	var document struct {
		Linkset []struct {
			Href string `json:"href"`
			Rel  string `json:"rel"`
		} `json:"linkset"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return metadataReferences{}, err
	}

	var refs metadataReferences
	for _, link := range document.Linkset {
		switch {
		case strings.Contains(link.Rel, "skill-manifest"):
			refs.skills = append(refs.skills, link.Href)
		case strings.Contains(link.Rel, "workflows"):
			refs.workflows = append(refs.workflows, link.Href)
		case strings.Contains(link.Rel, "schema-overlay"):
			refs.overlays = append(refs.overlays, link.Href)
		}
	}
	return refs, nil
}

func deriveServiceID(document *openapi3.T, sourceURI, sourceType string) string {
	for _, path := range sortedPathKeys(document.Paths.Map()) {
		segment := firstPathSegment(path)
		if segment != "" {
			return segment
		}
	}
	title := slugify(document.Info.Title)
	if title != "" {
		return title
	}
	return slugify(path.Base(sourceURI))
}

func sortedPathKeys(items map[string]*openapi3.PathItem) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func uniqueRefs(existing, additional []string) []string {
	seen := map[string]struct{}{}
	var values []string
	for _, ref := range append(existing, additional...) {
		if ref == "" {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen[ref] = struct{}{}
		values = append(values, ref)
	}
	return values
}

func extractAuth(document *openapi3.T, operation *openapi3.Operation) []AuthRequirement {
	security := operation.Security
	if security == nil {
		security = &document.Security
	}
	if security == nil {
		return nil
	}

	var requirements []AuthRequirement
	seen := map[string]struct{}{}
	for _, item := range *security {
		for schemeName := range item {
			if _, ok := seen[schemeName]; ok {
				continue
			}
			seen[schemeName] = struct{}{}
			schemeRef := document.Components.SecuritySchemes[schemeName]
			if schemeRef == nil || schemeRef.Value == nil {
				continue
			}
			requirements = append(requirements, AuthRequirement{
				Name:      schemeName,
				Type:      schemeRef.Value.Type,
				Scheme:    schemeRef.Value.Scheme,
				In:        schemeRef.Value.In,
				ParamName: schemeRef.Value.Name,
			})
		}
	}
	return requirements
}

func extractServers(document *openapi3.T) []string {
	var servers []string
	for _, server := range document.Servers {
		servers = append(servers, server.URL)
	}
	return servers
}

func extractParameters(pathParameters, operationParameters openapi3.Parameters) ([]Parameter, []Parameter) {
	var pathParams []Parameter
	var flags []Parameter
	for _, parameter := range append(pathParameters, operationParameters...) {
		if parameter == nil || parameter.Value == nil {
			continue
		}

		name := parameter.Value.Name
		if override := parameterExtension(parameter.Value, "x-cli-name"); override != "" {
			name = override
		}

		current := Parameter{
			Name:         slugify(name),
			OriginalName: parameter.Value.Name,
			Location:     parameter.Value.In,
			Required:     parameter.Value.Required,
		}

		if parameter.Value.In == "path" {
			pathParams = append(pathParams, current)
			continue
		}
		flags = append(flags, current)
	}
	return pathParams, flags
}

func buildEffectiveViews(cfg config.Config, tools []Tool) []EffectiveView {
	views := []EffectiveView{
		{
			Name:  "discover",
			Mode:  "discover",
			Tools: append([]Tool(nil), tools...),
		},
	}

	profileNames := sortedKeys(cfg.Agents.Profiles)
	for _, name := range profileNames {
		profile := cfg.Agents.Profiles[name]
		toolSet := cfg.Curation.ToolSets[profile.ToolSet]
		view := EffectiveView{Name: name, Mode: profile.Mode}
		for _, tool := range tools {
			if toolAllowed(tool.ID, toolSet) {
				view.Tools = append(view.Tools, tool)
			}
		}
		views = append(views, view)
	}

	return views
}

func toolAllowed(toolID string, toolSet config.ToolSet) bool {
	if len(toolSet.Allow) > 0 && !matchesAny(toolSet.Allow, toolID) {
		return false
	}
	for _, pattern := range toolSet.Deny {
		if pattern == "**" && len(toolSet.Allow) > 0 {
			continue
		}
		if matchPattern(pattern, toolID) {
			return false
		}
	}
	return true
}

func matchesAny(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if matchPattern(pattern, value) {
			return true
		}
	}
	return false
}

func matchPattern(pattern, value string) bool {
	if pattern == "**" {
		return true
	}
	matched, err := path.Match(pattern, value)
	if err != nil {
		return pattern == value
	}
	return matched
}

func deriveSafety(method string, operation *openapi3.Operation) Safety {
	safety := Safety{
		ReadOnly:         method == "GET" || method == "HEAD" || method == "OPTIONS",
		Destructive:      method == "DELETE",
		RequiresApproval: false,
	}

	if raw, ok := operation.Extensions["x-cli-safety"]; ok {
		data, err := json.Marshal(raw)
		if err == nil {
			_ = json.Unmarshal(data, &safety)
		}
	}
	return safety
}

func operationExtension(operation *openapi3.Operation, key string) string {
	if raw, ok := operation.Extensions[key]; ok {
		switch typed := raw.(type) {
		case string:
			return typed
		}
	}
	return ""
}

func parameterExtension(parameter *openapi3.Parameter, key string) string {
	if raw, ok := parameter.Extensions[key]; ok {
		switch typed := raw.(type) {
		case string:
			return typed
		}
	}
	return ""
}

func normalizeCommandName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return slugify(value)
}

func inferCommand(method, rawPath string) string {
	switch method {
	case "GET":
		if strings.Contains(rawPath, "{") {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT":
		return "update"
	case "PATCH":
		return "patch"
	case "DELETE":
		return "delete"
	default:
		return "run"
	}
}

func firstPathSegment(rawPath string) string {
	parts := strings.Split(strings.Trim(rawPath, "/"), "/")
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, "{") {
			continue
		}
		return slugify(part)
	}
	return ""
}

func slugify(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	value = strings.ReplaceAll(value, "_", "-")
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
			if builder.Len() > 0 {
				builder.WriteByte('-')
			}
			builder.WriteRune(char + ('a' - 'A'))
		case (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'):
			builder.WriteRune(char)
		case char == '-' || char == ' ' || char == '.':
			if builder.Len() > 0 && builder.String()[builder.Len()-1] != '-' {
				builder.WriteByte('-')
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func sortedKeys[T any](items map[string]T) []string {
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func httpMethod(method string) string {
	return method
}
