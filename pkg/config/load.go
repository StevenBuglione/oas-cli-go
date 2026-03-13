package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

type rawConfig struct {
	CLI      string                `json:"cli"`
	Mode     *ModeConfig           `json:"mode,omitempty"`
	Sources  map[string]rawSource  `json:"sources,omitempty"`
	Services map[string]rawService `json:"services,omitempty"`
	Curation *rawCurationConfig    `json:"curation,omitempty"`
	Agents   *rawAgentsConfig      `json:"agents,omitempty"`
	Policy   *rawPolicyConfig      `json:"policy,omitempty"`
	Secrets  map[string]SecretRef  `json:"secrets,omitempty"`
}

type rawSource struct {
	Type    *string        `json:"type,omitempty"`
	URI     *string        `json:"uri,omitempty"`
	Enabled *bool          `json:"enabled,omitempty"`
	Refresh *RefreshPolicy `json:"refresh,omitempty"`
}

type rawService struct {
	Source    *string  `json:"source,omitempty"`
	Alias     *string  `json:"alias,omitempty"`
	Overlays  []string `json:"overlays,omitempty"`
	Skills    []string `json:"skills,omitempty"`
	Workflows []string `json:"workflows,omitempty"`
}

type rawCurationConfig struct {
	ToolSets map[string]rawToolSet `json:"toolSets,omitempty"`
}

type rawToolSet struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

type rawAgentsConfig struct {
	Profiles       map[string]rawAgentProfile `json:"profiles,omitempty"`
	DefaultProfile *string                    `json:"defaultProfile,omitempty"`
}

type rawAgentProfile struct {
	Mode    *string `json:"mode,omitempty"`
	ToolSet *string `json:"toolSet,omitempty"`
}

type rawPolicyConfig struct {
	Deny             []string `json:"deny,omitempty"`
	ApprovalRequired []string `json:"approvalRequired,omitempty"`
	AllowExecSecrets *bool    `json:"allowExecSecrets,omitempty"`
}

func LoadEffective(options LoadOptions) (*EffectiveConfig, error) {
	effective := &EffectiveConfig{
		Config: Config{
			Sources:  map[string]Source{},
			Services: map[string]Service{},
			Curation: CurationConfig{ToolSets: map[string]ToolSet{}},
			Agents:   AgentsConfig{Profiles: map[string]AgentProfile{}},
			Secrets:  map[string]SecretRef{},
		},
		ScopePaths: map[Scope]string{},
	}

	scopedPaths := []struct {
		scope Scope
		path  string
	}{
		{scope: ScopeManaged, path: options.ManagedPath},
		{scope: ScopeUser, path: options.UserPath},
		{scope: ScopeProject, path: options.ProjectPath},
		{scope: ScopeLocal, path: options.LocalPath},
	}

	for _, entry := range scopedPaths {
		if entry.path == "" {
			continue
		}

		raw, err := loadRaw(entry.path)
		if err != nil {
			return nil, err
		}
		effective.ScopePaths[entry.scope] = entry.path
		effective.Config.merge(entry.scope, raw)
	}

	if options.LocalPath != "" {
		effective.BaseDir = filepath.Dir(options.LocalPath)
	} else if options.ProjectPath != "" {
		effective.BaseDir = filepath.Dir(options.ProjectPath)
	}

	if err := validate(effective.Config); err != nil {
		return nil, err
	}

	return effective, nil
}

func loadRaw(path string) (rawConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rawConfig{}, err
	}

	var raw rawConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return rawConfig{}, err
	}

	return raw, nil
}

func (cfg *Config) merge(scope Scope, raw rawConfig) {
	if raw.CLI != "" {
		cfg.CLI = raw.CLI
	}
	if raw.Mode != nil && raw.Mode.Default != "" {
		cfg.Mode = *raw.Mode
	}

	for key, source := range raw.Sources {
		current := cfg.Sources[key]
		if source.Type != nil {
			current.Type = *source.Type
		}
		if source.URI != nil {
			current.URI = *source.URI
		}
		if source.Enabled != nil {
			current.Enabled = *source.Enabled
		} else if current.Type == "" && current.URI == "" && !current.Enabled {
			current.Enabled = true
		}
		if source.Refresh != nil {
			current.Refresh = source.Refresh
		}
		cfg.Sources[key] = current
	}

	for key, service := range raw.Services {
		current := cfg.Services[key]
		if service.Source != nil {
			current.Source = *service.Source
		}
		if service.Alias != nil {
			current.Alias = *service.Alias
		}
		if service.Overlays != nil {
			current.Overlays = append([]string(nil), service.Overlays...)
		}
		if service.Skills != nil {
			current.Skills = append([]string(nil), service.Skills...)
		}
		if service.Workflows != nil {
			current.Workflows = append([]string(nil), service.Workflows...)
		}
		cfg.Services[key] = current
	}

	if raw.Curation != nil {
		if cfg.Curation.ToolSets == nil {
			cfg.Curation.ToolSets = map[string]ToolSet{}
		}
		for key, rawToolSet := range raw.Curation.ToolSets {
			current := cfg.Curation.ToolSets[key]
			current.Allow = uniqueStrings(current.Allow, rawToolSet.Allow)
			current.Deny = uniqueStrings(current.Deny, rawToolSet.Deny)
			cfg.Curation.ToolSets[key] = current
		}
	}

	if raw.Agents != nil {
		if cfg.Agents.Profiles == nil {
			cfg.Agents.Profiles = map[string]AgentProfile{}
		}
		if raw.Agents.DefaultProfile != nil {
			cfg.Agents.DefaultProfile = *raw.Agents.DefaultProfile
		}
		for key, rawProfile := range raw.Agents.Profiles {
			current := cfg.Agents.Profiles[key]
			if rawProfile.Mode != nil {
				current.Mode = *rawProfile.Mode
			}
			if rawProfile.ToolSet != nil {
				current.ToolSet = *rawProfile.ToolSet
			}
			cfg.Agents.Profiles[key] = current
		}
	}

	if raw.Policy != nil {
		cfg.Policy.Deny = uniqueStrings(cfg.Policy.Deny, raw.Policy.Deny)
		if scope == ScopeManaged {
			cfg.Policy.ManagedDeny = uniqueStrings(cfg.Policy.ManagedDeny, raw.Policy.Deny)
		}
		cfg.Policy.ApprovalRequired = uniqueStrings(cfg.Policy.ApprovalRequired, raw.Policy.ApprovalRequired)
		if raw.Policy.AllowExecSecrets != nil {
			cfg.Policy.AllowExecSecrets = *raw.Policy.AllowExecSecrets
		}
	}

	for key, secret := range raw.Secrets {
		cfg.Secrets[key] = secret
	}
}

func validate(cfg Config) error {
	var diagnostics []Diagnostic

	if cfg.CLI == "" {
		diagnostics = append(diagnostics, Diagnostic{Path: "cli", Message: "is required"})
	}
	if cfg.Mode.Default == "" {
		diagnostics = append(diagnostics, Diagnostic{Path: "mode.default", Message: "is required"})
	}
	if len(cfg.Sources) == 0 {
		diagnostics = append(diagnostics, Diagnostic{Path: "sources", Message: "must define at least one source"})
	}

	for key, source := range cfg.Sources {
		prefix := "sources." + key
		if source.Type == "" {
			diagnostics = append(diagnostics, Diagnostic{Path: prefix + ".type", Message: "is required"})
		}
		if source.URI == "" {
			diagnostics = append(diagnostics, Diagnostic{Path: prefix + ".uri", Message: "is required"})
		}
	}

	for key, service := range cfg.Services {
		if service.Source == "" {
			diagnostics = append(diagnostics, Diagnostic{Path: "services." + key + ".source", Message: "is required"})
			continue
		}
		if _, ok := cfg.Sources[service.Source]; !ok {
			diagnostics = append(diagnostics, Diagnostic{Path: "services." + key + ".source", Message: "references unknown source"})
		}
	}

	if len(diagnostics) > 0 {
		return &ValidationError{Diagnostics: diagnostics}
	}

	return nil
}

func uniqueStrings(existing, additional []string) []string {
	seen := map[string]struct{}{}
	var merged []string

	for _, item := range existing {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}
	for _, item := range additional {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		merged = append(merged, item)
	}

	return merged
}
