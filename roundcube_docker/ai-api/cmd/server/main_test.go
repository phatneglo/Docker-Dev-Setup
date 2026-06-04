package main

import (
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
