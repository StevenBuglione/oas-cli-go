package overlay

type Document struct {
	Overlay string   `json:"overlay" yaml:"overlay"`
	Extends string   `json:"extends" yaml:"extends"`
	Actions []Action `json:"actions" yaml:"actions"`
}

type Action struct {
	Target string         `json:"target" yaml:"target"`
	Update map[string]any `json:"update,omitempty" yaml:"update,omitempty"`
	Remove bool           `json:"remove,omitempty" yaml:"remove,omitempty"`
	Copy   *Copy          `json:"copy,omitempty" yaml:"copy,omitempty"`
}

type Copy struct {
	To string `json:"to" yaml:"to"`
}
