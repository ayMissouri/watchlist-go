// Package claude answers AMI's questions with Claude Haiku 5.5 when
// ANTHROPIC_API_KEY is set, in place of the gemini package. It takes the same
// requests and fails with the same errors, so the handler treats both alike.
package claude

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ayMissouri/watchlist-go.git/internal/gemini"
)

const Model = "claude-haiku-5-5"

type Client struct {
	key string
	api anthropic.Client
}

func NewClient(key string, opts ...option.RequestOption) *Client {
	return &Client{key: key, api: anthropic.NewClient(append([]option.RequestOption{option.WithAPIKey(key)}, opts...)...)}
}

func (c *Client) Configured() bool { return c.key != "" }

// Generate returns the model's JSON reply.
func (c *Client) Generate(ctx context.Context, req gemini.Request) (json.RawMessage, error) {
	var schema map[string]any
	if err := json.Unmarshal(req.Schema, &schema); err != nil {
		return nil, &gemini.Error{Kind: gemini.Bad}
	}
	msg, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     Model,
		MaxTokens: int64(req.MaxTokens),
		System:    []anthropic.TextBlockParam{{Text: req.Instructions}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt))},
		OutputConfig: anthropic.OutputConfigParam{
			// AMI's steps read and choose rather than reason, so it thinks little.
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: strict(schema)},
		},
	})
	if err != nil {
		return nil, errorFor(err)
	}
	if msg.StopReason == anthropic.StopReasonRefusal {
		return nil, &gemini.Error{Kind: gemini.Blocked}
	}
	var text strings.Builder
	for _, b := range msg.Content {
		if t, ok := b.AsAny().(anthropic.TextBlock); ok {
			text.WriteString(t.Text)
		}
	}
	reply := json.RawMessage(text.String())
	// Stopped at the token cap partway through.
	if msg.StopReason == anthropic.StopReasonMaxTokens || !json.Valid(reply) {
		return nil, &gemini.Error{Kind: gemini.Unavailable}
	}
	return reply, nil
}

// strict fits a schema the apps wrote for Gemini to Claude's structured
// outputs: every object closed to extra properties, and none of the length or
// range limits Claude turns down (minItems only up to 1).
func strict(s map[string]any) map[string]any {
	if s["type"] == "object" || s["properties"] != nil {
		s["additionalProperties"] = false
	}
	for _, k := range []string{"maxItems", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "multipleOf", "minLength", "maxLength"} {
		delete(s, k)
	}
	if n, ok := s["minItems"].(float64); ok && n > 1 {
		delete(s, "minItems")
	}
	each := func(v any) {
		if m, ok := v.(map[string]any); ok {
			strict(m)
		}
	}
	for k, v := range s {
		switch k {
		case "properties", "$defs", "definitions":
			// Keyed by name, so only the values are schemas.
			if m, ok := v.(map[string]any); ok {
				for _, sub := range m {
					each(sub)
				}
			}
		case "items", "not":
			each(v)
		case "anyOf", "allOf", "oneOf", "prefixItems":
			if l, ok := v.([]any); ok {
				for _, sub := range l {
					each(sub)
				}
			}
		}
	}
	return s
}

func errorFor(err error) *gemini.Error {
	var apiErr *anthropic.Error
	if !errors.As(err, &apiErr) {
		// Timed out or couldn't connect.
		return &gemini.Error{Kind: gemini.Unavailable}
	}
	status := apiErr.StatusCode
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &gemini.Error{Kind: gemini.Unauthorized, Status: status}
	case status == http.StatusPaymentRequired:
		// Out of credit, which doesn't come back on its own.
		return &gemini.Error{Kind: gemini.Quota, Status: status}
	case status == http.StatusTooManyRequests:
		e := &gemini.Error{Kind: gemini.Quota, Status: status}
		if apiErr.Response != nil {
			if secs, err := strconv.ParseFloat(apiErr.Response.Header.Get("retry-after"), 64); err == nil {
				e.RetryAfter = time.Duration(secs * float64(time.Second))
			}
		}
		return e
	case status >= 500:
		// Includes 529, overloaded.
		return &gemini.Error{Kind: gemini.Unavailable, Status: status}
	default:
		return &gemini.Error{Kind: gemini.Bad, Status: status}
	}
}
