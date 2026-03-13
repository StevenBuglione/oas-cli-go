package obs

import (
	"context"
	"testing"
	"time"
)

func TestObserverCapturesStructuredEventFields(t *testing.T) {
	recorder := NewRecorder()
	ctx, finish := recorder.StartSpan(context.Background(), "runtime.refresh", map[string]string{
		"service": "tickets",
	})
	recorder.Emit(ctx, Event{
		Name:         "cache.fetch",
		Service:      "tickets",
		Operation:    "refresh",
		URL:          "https://api.example.com/openapi.json",
		CacheOutcome: "revalidated_hit",
		StatusCode:   304,
		Duration:     25 * time.Millisecond,
		RequestID:    "req-123",
	})
	finish(nil)

	events := recorder.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %#v", events)
	}
	if events[0].Name != "cache.fetch" || events[0].RequestID != "req-123" {
		t.Fatalf("unexpected event payload: %#v", events[0])
	}

	spans := recorder.Spans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %#v", spans)
	}
	if spans[0].Name != "runtime.refresh" || spans[0].Attributes["service"] != "tickets" {
		t.Fatalf("unexpected span payload: %#v", spans[0])
	}
}
