package overlay

import (
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}

	var doc Document
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}
