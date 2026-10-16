// Package gemini answers AMI's questions (the awatch apps' assistant) with
// Google's Gemini API on its free tier, so the API key stays on the server.
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Model is a Gemini model AMI can answer with, and how much it thinks first.
// Thinking holds back the first word, and AMI's steps read and choose rather
// than reason, so it's kept low.
type Model struct {
	ID       string
	Thinking string
}

// FreeTier is tried in order. Each model has its own free quota, so when one
// runs out (429) or is down, the next answers. Flash-Lite goes first: on
// 2026-10-01 the free tier's Flash models mostly answered 503 "high demand",
// while 3.5 Flash-Lite answered every time in about 1.5 seconds.
var FreeTier = []Model{
	{"gemini-3.5-flash-lite", "MINIMAL"},
	{"gemini-3.1-flash-lite", "MINIMAL"},
	{"gemini-3.6-flash", "LOW"},
	{"gemini-3.8-flash", "LOW"},
}

// Request is one step of an AMI question: what the model is told, what it's
// asked and the JSON Schema its reply has to follow.
type Request struct {
	Instructions string          `json:"instructions"`
	Prompt       string          `json:"prompt"`
	Schema       json.RawMessage `json:"schema" swaggertype:"object"`
	MaxTokens    int             `json:"max_tokens"`
}

// Kind says why a call failed.
type Kind int

const (
	// Unauthorized: Google turned the key down.
	Unauthorized Kind = iota
	// Quota: over the free quota.
	Quota
	// Unavailable: overloaded, down, timed out, or the reply stopped partway.
	Unavailable
	// Blocked: the model wouldn't answer, for safety reasons.
	Blocked
	// Bad: any other 4xx, such as a model that's been retired.
	Bad
)

type Error struct {
	Kind       Kind
	RetryAfter time.Duration
	Status     int
}

func (e *Error) Error() string { return fmt.Sprintf("gemini: kind %d, status %d", e.Kind, e.Status) }

// triesAnotherModel: another model has its own quota and may be up.
func (e *Error) triesAnotherModel() bool { return e.Kind != Unauthorized && e.Kind != Blocked }

const (
	// A busy model can hang for most of a minute before its 503, and the next one is usually quicker.
	perModelTimeout = 25 * time.Second
	maxReplyBytes   = 1 << 20
)

type Client struct {
	key        string
	models     []Model
	httpClient *http.Client
	endpoint   string

	mu sync.Mutex
	// Models over their quota or down, and until when. They go last.
	resting map[string]time.Time
}

// NewClient answers with the free-tier models. An empty key leaves it unconfigured.
func NewClient(key string) *Client {
	if key == "" {
		log.Print("gemini: GEMINI_API_KEY is not set, AMI will answer 503")
	}
	return &Client{
		key:        key,
		models:     FreeTier,
		httpClient: &http.Client{},
		endpoint:   "https://generativelanguage.googleapis.com/v1beta/models/",
		resting:    map[string]time.Time{},
	}
}

func (c *Client) Configured() bool { return c.key != "" }

// Generate returns the model's JSON reply. One model failing hands over to the next.
func (c *Client) Generate(ctx context.Context, req Request) (json.RawMessage, error) {
	var failure *Error
	for _, m := range c.order(time.Now()) {
		reply, err := c.call(ctx, m, req)
		if err == nil {
			c.setResting(m.ID, time.Time{})
			return reply, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var e *Error
		if !errors.As(err, &e) || !e.triesAnotherModel() {
			return nil, err
		}
		log.Printf("gemini: %s failed, trying the next model: %v", m.ID, err)
		c.setResting(m.ID, restUntil(e, time.Now()))
		// Running out of quota says more than whatever the last model said.
		if failure == nil || failure.Kind != Quota {
			failure = e
		}
	}
	if failure == nil {
		return nil, &Error{Kind: Unavailable}
	}
	return nil, failure
}

// order puts resting models last.
func (c *Client) order(now time.Time) []Model {
	c.mu.Lock()
	defer c.mu.Unlock()
	resting := func(m Model) bool { return c.resting[m.ID].After(now) }
	ready := slices.DeleteFunc(slices.Clone(c.models), resting)
	return append(ready, slices.DeleteFunc(slices.Clone(c.models), func(m Model) bool { return !resting(m) })...)
}

func (c *Client) setResting(id string, until time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if until.IsZero() {
		delete(c.resting, id)
	} else {
		c.resting[id] = until
	}
}

// restUntil is when a model that failed comes back to the front of the line.
func restUntil(e *Error, now time.Time) time.Time {
	switch {
	case e.Kind == Quota && e.RetryAfter > 0:
		return now.Add(e.RetryAfter)
	case e.Kind == Quota:
		return now.Add(time.Minute)
	case e.Kind == Bad && e.Status == http.StatusNotFound:
		// Retired: not again until a restart. (A 400 can be the caller's
		// request rather than the model, so it only rests the usual while.)
		return now.Add(100 * 365 * 24 * time.Hour)
	default:
		return now.Add(30 * time.Second)
	}
}

type part struct {
	Text    string `json:"text"`
	Thought bool   `json:"thought,omitempty"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type response struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

func (c *Client) call(ctx context.Context, m Model, req Request) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, perModelTimeout)
	defer cancel()

	config := map[string]any{
		"responseMimeType":   "application/json",
		"responseJsonSchema": req.Schema,
		"maxOutputTokens":    req.MaxTokens,
	}
	if m.Thinking != "" {
		config["thinkingConfig"] = map[string]string{"thinkingLevel": m.Thinking}
	}
	// No temperature: Google recommends leaving Gemini 3 at its default, as lower ones make it loop.
	body, err := json.Marshal(map[string]any{
		"systemInstruction": content{Parts: []part{{Text: req.Instructions}}},
		"contents":          []content{{Role: "user", Parts: []part{{Text: req.Prompt}}}},
		"generationConfig":  config,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+m.ID+":generateContent", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", c.key)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Timed out or couldn't connect: the next model may do better.
		return nil, &Error{Kind: Unavailable}
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes))
	if err != nil {
		return nil, &Error{Kind: Unavailable}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errorFor(resp.StatusCode, data)
	}

	var r response
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, &Error{Kind: Unavailable}
	}
	if r.PromptFeedback.BlockReason != "" {
		return nil, &Error{Kind: Blocked}
	}
	if len(r.Candidates) == 0 {
		return nil, &Error{Kind: Unavailable}
	}
	switch r.Candidates[0].FinishReason {
	case "SAFETY", "PROHIBITED_CONTENT", "BLOCKLIST", "SPII", "RECITATION":
		return nil, &Error{Kind: Blocked}
	}
	var text strings.Builder
	for _, p := range r.Candidates[0].Content.Parts {
		if !p.Thought {
			text.WriteString(p.Text)
		}
	}
	reply := json.RawMessage(text.String())
	// Stopped at the token cap halfway through a string, or isn't a JSON object.
	if !json.Valid(reply) || !bytes.HasPrefix(bytes.TrimSpace(reply), []byte("{")) {
		return nil, &Error{Kind: Unavailable}
	}
	return reply, nil
}

type detail struct {
	Reason     string `json:"reason"`
	RetryDelay string `json:"retryDelay"`
}

// errorFor reads the API's `{ "error": { code, message, status, details } }`.
func errorFor(status int, body []byte) *Error {
	var e struct {
		Error struct {
			Status  string   `json:"status"`
			Details []detail `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &e)
	badKey := e.Error.Status == "FAILED_PRECONDITION" ||
		slices.ContainsFunc(e.Error.Details, func(d detail) bool { return d.Reason == "API_KEY_INVALID" })
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden,
		status == http.StatusBadRequest && badKey:
		return &Error{Kind: Unauthorized, Status: status}
	case status == http.StatusTooManyRequests:
		qe := &Error{Kind: Quota, Status: status}
		for _, d := range e.Error.Details {
			// "36s" or "1.5s"
			if secs, err := strconv.ParseFloat(strings.TrimSuffix(d.RetryDelay, "s"), 64); err == nil && strings.HasSuffix(d.RetryDelay, "s") {
				qe.RetryAfter = time.Duration(secs * float64(time.Second))
				break
			}
		}
		return qe
	case status >= 500:
		return &Error{Kind: Unavailable, Status: status}
	default:
		return &Error{Kind: Bad, Status: status}
	}
}
