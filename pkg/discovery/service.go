package discovery

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrServiceLinksUnavailable = errors.New("service description links unavailable")

func DiscoverServiceRoot(ctx context.Context, client *http.Client, serviceRoot string) (*ServiceRootResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	methods := []string{http.MethodHead, http.MethodGet}
	for _, method := range methods {
		req, err := http.NewRequestWithContext(ctx, method, serviceRoot, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			if method == http.MethodHead {
				continue
			}
			return nil, err
		}

		links := parseLinkHeader(resp.Header.Values("Link"))
		resp.Body.Close()
		if resp.StatusCode >= 400 || len(links) == 0 {
			continue
		}

		result := &ServiceRootResult{
			Provenance: FetchRecord{
				URL:       serviceRoot,
				FetchedAt: time.Now().UTC(),
				Method:    ProvenanceRFC8631,
			},
		}
		for _, link := range links {
			switch {
			case contains(link.Rels, "service-desc"):
				result.OpenAPIURL = resolveLink(serviceRoot, link.URL)
			case contains(link.Rels, "service-meta"):
				result.MetadataURL = resolveLink(serviceRoot, link.URL)
			}
		}
		if result.OpenAPIURL != "" || result.MetadataURL != "" {
			return result, nil
		}
	}

	return nil, ErrServiceLinksUnavailable
}

type parsedLink struct {
	URL  string
	Rels []string
}

func parseLinkHeader(values []string) []parsedLink {
	var results []parsedLink
	for _, value := range values {
		for _, segment := range splitLinkHeader(value) {
			link, ok := parseLinkSegment(segment)
			if ok {
				results = append(results, link)
			}
		}
	}
	return results
}

func splitLinkHeader(value string) []string {
	var segments []string
	start := 0
	inQuotes := false
	inAngles := false
	for i, r := range value {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case '<':
			if !inQuotes {
				inAngles = true
			}
		case '>':
			if !inQuotes {
				inAngles = false
			}
		case ',':
			if !inQuotes && !inAngles {
				segments = append(segments, strings.TrimSpace(value[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(value) {
		segments = append(segments, strings.TrimSpace(value[start:]))
	}
	return segments
}

func parseLinkSegment(segment string) (parsedLink, bool) {
	if !strings.HasPrefix(segment, "<") {
		return parsedLink{}, false
	}
	end := strings.Index(segment, ">")
	if end <= 1 {
		return parsedLink{}, false
	}
	link := parsedLink{URL: segment[1:end]}
	for _, part := range strings.Split(segment[end+1:], ";") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "rel=") {
			continue
		}
		value := strings.Trim(strings.TrimPrefix(part, "rel="), `"`)
		link.Rels = strings.Fields(value)
	}
	return link, true
}

func resolveLink(baseURL, href string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return href
	}
	rel, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(rel).String()
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
