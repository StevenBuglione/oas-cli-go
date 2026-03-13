package discovery

import "time"

type ProvenanceMethod string

const (
	ProvenanceExplicit  ProvenanceMethod = "explicit"
	ProvenanceRFC9727   ProvenanceMethod = "rfc9727"
	ProvenanceRFC8631   ProvenanceMethod = "rfc8631"
	ProvenanceHeuristic ProvenanceMethod = "heuristic"
)

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type FetchRecord struct {
	URL       string           `json:"url"`
	FetchedAt time.Time        `json:"fetchedAt"`
	Method    ProvenanceMethod `json:"method"`
}

type CatalogProvenance struct {
	Method  ProvenanceMethod `json:"method"`
	Fetches []FetchRecord    `json:"fetches"`
}

type ServiceReference struct {
	URL string `json:"url"`
}

type APICatalogResult struct {
	Services   []ServiceReference `json:"services"`
	Provenance CatalogProvenance  `json:"provenance"`
	Warnings   []Warning          `json:"warnings,omitempty"`
}

type ServiceRootResult struct {
	OpenAPIURL  string      `json:"openapiUrl"`
	MetadataURL string      `json:"metadataUrl"`
	Provenance  FetchRecord `json:"provenance"`
}
