package claude

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/ayMissouri/watchlist-go.git/internal/gemini"
)

// The apps' reply schema: an array with Gemini-only limits, and objects left open.
var step = gemini.Request{
	Instructions: "Answer.",
	Prompt:       "Request: hi",
	Schema: json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"},
		"picks":{"type":"array","minItems":3,"maxItems":6,"items":{"type":"object","properties":{"title":{"type":"string"}}}}}}`),
	MaxTokens: 100,
}

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient("test-key", option.WithBaseURL(srv.URL), option.WithMaxRetries(0))
}

func TestGenerateSendsAStrictSchemaAndReadsTheText(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		for _, want := range []string{`"model":"claude-haiku-5-5"`, `"effort":"low"`, `"max_tokens":100`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("request body missing %s: %s", want, body)
			}
		}
		if strings.Count(string(body), `"additionalProperties":false`) != 2 || strings.Contains(string(body), "Items") {
			t.Errorf("schema not made strict: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-5-5","stop_reason":"end_turn",
			"content":[{"type":"thinking","thinking":"","signature":"s"},{"type":"text","text":"{\"message\":\"hi\"}"}],
			"usage":{"input_tokens":1,"output_tokens":1}}`)
	})

	got, err := c.Generate(t.Context(), step)
	if err != nil || string(got) != `{"message":"hi"}` {
		t.Fatalf("got %s, %v", got, err)
	}
}

func TestErrorsMapToGeminiKinds(t *testing.T) {
	cases := []struct {
		status int
		header string
		want   gemini.Error
	}{
		{429, "36", gemini.Error{Kind: gemini.Quota, RetryAfter: 36 * time.Second, Status: 429}},
		{402, "", gemini.Error{Kind: gemini.Quota, Status: 402}},
		{401, "", gemini.Error{Kind: gemini.Unauthorized, Status: 401}},
		{529, "", gemini.Error{Kind: gemini.Unavailable, Status: 529}},
		{400, "", gemini.Error{Kind: gemini.Bad, Status: 400}},
	}
	for _, tc := range cases {
		c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
			if tc.header != "" {
				w.Header().Set("retry-after", tc.header)
			}
			w.WriteHeader(tc.status)
			_, _ = io.WriteString(w, `{"type":"error","error":{"type":"x","message":"x"}}`)
		})
		_, err := c.Generate(t.Context(), step)
		var e *gemini.Error
		if !errors.As(err, &e) || *e != tc.want {
			t.Errorf("%d: got %+v", tc.status, err)
		}
	}
}
