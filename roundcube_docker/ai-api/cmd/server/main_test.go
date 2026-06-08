package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildPromptForSummarize(t *testing.T) {
	req := AIRequest{Action: "summarize", Text: "Please attend the meeting tomorrow."}
	_, userPrompt := buildPrompt(req)
	if !strings.Contains(userPrompt, "Summarize") {
		t.Fatalf("expected summarize prompt, got %q", userPrompt)
	}
	if !strings.Contains(userPrompt, req.Text) {
		t.Fatalf("expected email text inside prompt")
	}
}

func TestBuildPromptForChatIncludesMailboxContext(t *testing.T) {
	req := AIRequest{
		Action: "chat",
		Prompt: "What are my recent emails?",
		Context: map[string]any{
			"mailbox": map[string]any{
				"recent_messages": []any{
					map[string]any{"subject": "Local queue write test"},
				},
			},
		},
	}

	_, userPrompt := buildPrompt(req)
	if !strings.Contains(userPrompt, "MAILBOX CONTEXT JSON") {
		t.Fatalf("expected chat prompt to include mailbox context label")
	}
	if !strings.Contains(userPrompt, "Local queue write test") {
		t.Fatalf("expected chat prompt to include mailbox context")
	}
}

func TestLimitString(t *testing.T) {
	got := limitString("abcdef", 3)
	if got != "abc" {
		t.Fatalf("expected abc, got %q", got)
	}
}

func TestMockResult(t *testing.T) {
	got := mockResult(AIRequest{Action: "phishing"})
	if !strings.Contains(got, "Risk Level") {
		t.Fatalf("expected phishing risk text, got %q", got)
	}
}

func TestGetMockModeAuto(t *testing.T) {
	t.Setenv("AI_MOCK_MODE", "auto")

	if !getMockMode("AI_MOCK_MODE", "") {
		t.Fatalf("expected auto mode without API key to use mock mode")
	}

	if getMockMode("AI_MOCK_MODE", "sk-test") {
		t.Fatalf("expected auto mode with API key to use real AI")
	}
}

func TestGetMockModeExplicitTrueOverridesAPIKey(t *testing.T) {
	t.Setenv("AI_MOCK_MODE", "true")

	if !getMockMode("AI_MOCK_MODE", "sk-test") {
		t.Fatalf("expected explicit true to force mock mode")
	}
}

func TestStreamMockResultWritesChunks(t *testing.T) {
	var got strings.Builder

	streamMockResult(context.Background(), AIRequest{Action: "ask"}, func(chunk string) error {
		got.WriteString(chunk)
		return nil
	})

	if !strings.Contains(got.String(), "Mock answer") {
		t.Fatalf("expected streamed mock answer, got %q", got.String())
	}
}

func TestCallOpenAICompatibleStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	cfg := Config{
		BaseURL: server.URL,
		APIKey:  "sk-test",
		Model:   "test-model",
	}

	var got strings.Builder
	err := callOpenAICompatibleStream(context.Background(), cfg, "system", "user", func(chunk string) error {
		got.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected stream success, got %v", err)
	}
	if got.String() != "Hello world" {
		t.Fatalf("expected streamed text, got %q", got.String())
	}
}
