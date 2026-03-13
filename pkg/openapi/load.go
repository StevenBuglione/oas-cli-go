package openapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenBuglione/oas-cli-go/pkg/overlay"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

type LoadedDocument struct {
	Raw         map[string]any
	Document    *openapi3.T
	Fingerprint string
}

func LoadDocument(ctx context.Context, baseDir, ref string, overlays []string) (*LoadedDocument, error) {
	raw, fingerprint, err := loadAny(ctx, resolveReference(baseDir, ref))
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	hash.Write([]byte(fingerprint))

	for _, overlayRef := range overlays {
		path := resolveReference(baseDir, overlayRef)
		doc, err := overlay.Load(path)
		if err != nil {
			return nil, err
		}
		raw, err = overlay.Apply(raw, doc)
		if err != nil {
			return nil, err
		}
		hash.Write([]byte(path))
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	document, err := loader.LoadFromData(data)
	if err != nil {
		return nil, err
	}
	if err := document.Validate(ctx); err != nil {
		return nil, err
	}

	return &LoadedDocument{
		Raw:         raw,
		Document:    document,
		Fingerprint: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func ResolveReference(baseDir, ref string) string {
	return resolveReference(baseDir, ref)
}

func resolveReference(baseDir, ref string) string {
	if ref == "" {
		return ref
	}
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "file://") {
		return ref
	}
	if filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(baseDir, ref)
}

func loadAny(ctx context.Context, ref string) (map[string]any, string, error) {
	data, err := ReadReference(ctx, ref)
	if err != nil {
		return nil, "", err
	}

	var decoded any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return nil, "", err
	}
	normalized := normalize(decoded)
	object, ok := normalized.(map[string]any)
	if !ok {
		return nil, "", fmt.Errorf("expected object document at %s", ref)
	}
	return object, string(data), nil
}

func ReadReference(ctx context.Context, ref string) ([]byte, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ref, nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		return io.ReadAll(resp.Body)
	}
	if strings.HasPrefix(ref, "file://") {
		parsed, err := url.Parse(ref)
		if err != nil {
			return nil, err
		}
		ref = parsed.Path
	}
	return os.ReadFile(ref)
}

func normalize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, inner := range typed {
			normalized[key] = normalize(inner)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, inner := range typed {
			normalized[fmt.Sprint(key)] = normalize(inner)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for idx, inner := range typed {
			normalized[idx] = normalize(inner)
		}
		return normalized
	default:
		return typed
	}
}
