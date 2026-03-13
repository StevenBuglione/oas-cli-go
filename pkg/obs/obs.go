package obs

import (
	"context"
	"sync"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

type Event struct {
	Name          string        `json:"name"`
	Service       string        `json:"service,omitempty"`
	Operation     string        `json:"operation,omitempty"`
	URL           string        `json:"url,omitempty"`
	CacheOutcome  string        `json:"cacheOutcome,omitempty"`
	StatusCode    int           `json:"statusCode,omitempty"`
	Duration      time.Duration `json:"duration,omitempty"`
	ErrorCategory string        `json:"errorCategory,omitempty"`
	RequestID     string        `json:"requestId,omitempty"`
}

type Span struct {
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes,omitempty"`
	StartedAt  time.Time         `json:"startedAt"`
	FinishedAt time.Time         `json:"finishedAt"`
	Error      string            `json:"error,omitempty"`
}

type Observer interface {
	Emit(context.Context, Event)
	StartSpan(context.Context, string, map[string]string) (context.Context, func(error))
}

type nopObserver struct{}

func NewNop() Observer {
	return nopObserver{}
}

func (nopObserver) Emit(context.Context, Event) {}

func (nopObserver) StartSpan(ctx context.Context, _ string, _ map[string]string) (context.Context, func(error)) {
	return ctx, func(error) {}
}

type Recorder struct {
	mu     sync.Mutex
	events []Event
	spans  []Span
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (recorder *Recorder) Emit(_ context.Context, event Event) {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
}

func (recorder *Recorder) StartSpan(ctx context.Context, name string, attributes map[string]string) (context.Context, func(error)) {
	startedAt := time.Now().UTC()
	return ctx, func(err error) {
		recorder.mu.Lock()
		defer recorder.mu.Unlock()

		span := Span{
			Name:       name,
			Attributes: cloneAttributes(attributes),
			StartedAt:  startedAt,
			FinishedAt: time.Now().UTC(),
		}
		if err != nil {
			span.Error = err.Error()
		}
		recorder.spans = append(recorder.spans, span)
	}
}

func (recorder *Recorder) Events() []Event {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]Event(nil), recorder.events...)
}

func (recorder *Recorder) Spans() []Span {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]Span(nil), recorder.spans...)
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(attributes))
	for key, value := range attributes {
		cloned[key] = value
	}
	return cloned
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
