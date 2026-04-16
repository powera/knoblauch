package integration

import (
	"context"
	"strings"
)

// Integration is implemented by each bot/service integration.
type Integration interface {
	Name() string
	Handle(ctx context.Context, query string) (string, error)
}

// Registry holds the allowlisted integrations and dispatches @mention messages.
type Registry struct {
	integrations map[string]Integration
}

func NewRegistry() *Registry {
	return &Registry{integrations: make(map[string]Integration)}
}

func (r *Registry) Register(i Integration) {
	r.integrations[i.Name()] = i
}

// Names returns the sorted list of registered integration names.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.integrations))
	for name := range r.integrations {
		names = append(names, name)
	}
	return names
}

// ParseMention parses a message body of the form "@name rest of query".
// Returns (name, query, true) on success; ("", "", false) if not a mention.
func ParseMention(body string) (name, query string, ok bool) {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "@") {
		return "", "", false
	}
	rest := body[1:]
	parts := strings.SplitN(rest, " ", 2)
	name = strings.ToLower(parts[0])
	if name == "" {
		return "", "", false
	}
	if len(parts) == 2 {
		query = strings.TrimSpace(parts[1])
	}
	return name, query, true
}

// Dispatch checks whether the message body is an @mention for a known integration.
// If so, it calls the integration and returns (responseBody, true).
// If not a mention or the name is not registered, returns ("", false).
func (r *Registry) Dispatch(ctx context.Context, body string) (string, bool) {
	name, query, ok := ParseMention(body)
	if !ok {
		return "", false
	}
	intg, found := r.integrations[name]
	if !found {
		return "", false
	}
	response, err := intg.Handle(ctx, query)
	if err != nil {
		return "[" + name + "] error: " + err.Error(), true
	}
	return response, true
}
