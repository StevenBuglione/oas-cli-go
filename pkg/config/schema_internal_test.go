package config

import "testing"

func TestValidateDocumentRejectsTransportOnOpenAPISource(t *testing.T) {
	document := []byte(`{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "tickets": {
	      "type": "openapi",
	      "uri": "https://example.com/openapi.json",
	      "transport": {
	        "type": "stdio",
	        "command": "npx"
	      }
	    }
	  }
	}`)

	err := validateDocument(document, false)
	if err == nil {
		t.Fatal("expected schema validation error")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	for _, diagnostic := range validationErr.Diagnostics {
		if diagnostic.Path == "sources.tickets.transport" {
			return
		}
	}

	t.Fatalf("expected schema diagnostic for sources.tickets.transport, got %#v", validationErr.Diagnostics)
}

func TestValidateDocumentRejectsOAuthOnOpenAPISource(t *testing.T) {
	document := []byte(`{
	  "cli": "1.0.0",
	  "mode": { "default": "discover" },
	  "sources": {
	    "tickets": {
	      "type": "openapi",
	      "uri": "https://example.com/openapi.json",
	      "oauth": {
	        "mode": "clientCredentials",
	        "tokenURL": "https://auth.example.com/oauth/token",
	        "clientId": { "type": "env", "value": "CLIENT_ID" },
	        "clientSecret": { "type": "env", "value": "CLIENT_SECRET" }
	      }
	    }
	  }
	}`)

	err := validateDocument(document, false)
	if err == nil {
		t.Fatal("expected schema validation error")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	for _, diagnostic := range validationErr.Diagnostics {
		if diagnostic.Path == "sources.tickets.oauth" {
			return
		}
	}

	t.Fatalf("expected schema diagnostic for sources.tickets.oauth, got %#v", validationErr.Diagnostics)
}
