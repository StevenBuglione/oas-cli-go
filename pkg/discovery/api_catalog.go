package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

type linksetDocument struct {
	Linkset []linksetLink `json:"linkset"`
	Links   []linksetLink `json:"links"`
}

type linksetLink struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
}

func DiscoverAPICatalog(ctx context.Context, client *http.Client, catalogURL string) (*APICatalogResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	result := &APICatalogResult{
		Provenance: CatalogProvenance{Method: ProvenanceRFC9727},
	}

	serviceSet := map[string]struct{}{}
	visited := map[string]bool{}
	stack := map[string]bool{}

	var visit func(string) error
	visit = func(current string) error {
		if stack[current] {
			result.Warnings = append(result.Warnings, Warning{
				Code:    "api_catalog_cycle",
				Message: fmt.Sprintf("cycle detected for %s", current),
			})
			return nil
		}
		if visited[current] {
			return nil
		}

		stack[current] = true
		defer delete(stack, current)
		visited[current] = true

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/linkset+json")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var doc linksetDocument
		if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
			return err
		}

		result.Provenance.Fetches = append(result.Provenance.Fetches, FetchRecord{
			URL:       current,
			FetchedAt: time.Now().UTC(),
			Method:    ProvenanceRFC9727,
		})

		links := doc.Linkset
		if len(links) == 0 {
			links = doc.Links
		}

		for _, link := range links {
			href, err := resolveURL(current, link.Href)
			if err != nil {
				return err
			}

			rels := strings.Fields(link.Rel)
			switch {
			case slices.Contains(rels, "item"):
				if _, ok := serviceSet[href]; ok {
					continue
				}
				serviceSet[href] = struct{}{}
				result.Services = append(result.Services, ServiceReference{URL: href})
			case slices.Contains(rels, "api-catalog"):
				if err := visit(href); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := visit(catalogURL); err != nil {
		return nil, err
	}

	return result, nil
}

func resolveURL(baseURL, href string) (string, error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	parsedHref, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return parsedBase.ResolveReference(parsedHref).String(), nil
}
