package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port          string
	BaseURL       string
	APIKey        string
	Model         string
	MockMode      bool
	SharedSecret  string
	MaxInputChars int
	Timeout       time.Duration
}

type AIRequest struct {
	Action   string `json:"action"`
	Text     string `json:"text"`
	Prompt   string `json:"prompt"`
	Tone     string `json:"tone"`
	Language string `json:"language"`
	Subject  string `json:"subject"`
	Context  any    `json:"context,omitempty"`
}

type AIResponse struct {
	Action string `json:"action"`
	Result string `json:"result"`
	Mock   bool   `json:"mock"`
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

type openAIStreamResponse struct {
	Choices []struct {
		Delta openAIMessage `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

func main() {
	cfg := loadConfig()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":        true,
			"service":   "itbs-pnp-mail-ai-api",
			"mock_mode": cfg.MockMode,
		})
	})

	for _, action := range []string{"compose", "reply", "summarize", "translate", "phishing", "ask", "chat"} {
		a := action
		mux.HandleFunc("POST /v1/ai/"+a, func(w http.ResponseWriter, r *http.Request) {
			handleAI(w, r, cfg, a)
		})
		mux.HandleFunc("POST /v1/ai/"+a+"/stream", func(w http.ResponseWriter, r *http.Request) {
			handleAIStream(w, r, cfg, a)
		})
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      withCORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.Timeout + 5*time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("ITBS PNP Mail AI API listening on :%s mock_mode=%v model=%s", cfg.Port, cfg.MockMode, cfg.Model)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func loadConfig() Config {
	maxChars := getEnvInt("AI_MAX_INPUT_CHARS", 12000)
	timeoutSeconds := getEnvInt("AI_TIMEOUT_SECONDS", 60)
	apiKey := os.Getenv("OPENAI_API_KEY")
	mockMode := getMockMode("AI_MOCK_MODE", apiKey)

	return Config{
		Port:          getEnv("AI_API_PORT", "8090"),
		BaseURL:       strings.TrimRight(getEnv("AI_BASE_URL", "https://api.openai.com/v1"), "/"),
		APIKey:        apiKey,
		Model:         getEnv("AI_MODEL", "gpt-4o-mini"),
		MockMode:      mockMode,
		SharedSecret:  os.Getenv("AI_SHARED_SECRET"),
		MaxInputChars: maxChars,
		Timeout:       time.Duration(timeoutSeconds) * time.Second,
	}
}

func handleAI(w http.ResponseWriter, r *http.Request, cfg Config, action string) {
	req, ok := readAIRequest(w, r, cfg, action)
	if !ok {
		return
	}

	systemPrompt, userPrompt := buildPrompt(req)

	if cfg.MockMode {
		writeJSON(w, http.StatusOK, AIResponse{Action: action, Result: mockResult(req), Mock: true})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
	defer cancel()

	result, err := callOpenAICompatible(ctx, cfg, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("AI error action=%s: %v", action, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, AIResponse{Action: action, Result: result, Mock: false})
}

func handleAIStream(w http.ResponseWriter, r *http.Request, cfg Config, action string) {
	req, ok := readAIRequest(w, r, cfg, action)
	if !ok {
		return
	}

	systemPrompt, userPrompt := buildPrompt(req)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-ITBS-AI-Mock", strconv.FormatBool(cfg.MockMode))

	flusher, _ := w.(http.Flusher)
	writeChunk := func(chunk string) error {
		if _, err := io.WriteString(w, chunk); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	if cfg.MockMode {
		streamMockResult(r.Context(), req, writeChunk)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), cfg.Timeout)
	defer cancel()

	if err := callOpenAICompatibleStream(ctx, cfg, systemPrompt, userPrompt, writeChunk); err != nil {
		log.Printf("AI stream error action=%s: %v", action, err)
		_ = writeChunk("\n\n[AI stream error: " + err.Error() + "]")
	}
}

func readAIRequest(w http.ResponseWriter, r *http.Request, cfg Config, action string) (AIRequest, bool) {
	if cfg.SharedSecret != "" && r.Header.Get("X-ITBS-AI-SECRET") != cfg.SharedSecret {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return AIRequest{}, false
	}

	var req AIRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(cfg.MaxInputChars+4096))).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return AIRequest{}, false
	}

	req.Action = action
	req.Text = limitString(req.Text, cfg.MaxInputChars)
	req.Prompt = limitString(req.Prompt, 3000)
	req.Tone = limitString(req.Tone, 100)
	req.Language = limitString(req.Language, 100)
	req.Subject = limitString(req.Subject, 300)

	return req, true
}

func buildPrompt(req AIRequest) (string, string) {
	system := "You are ITBS PNP Mail AI Assistant. Help users write, understand, and protect email content. Be clear, professional, and concise. Never invent facts not present in the email."

	tone := req.Tone
	if tone == "" {
		tone = "professional"
	}
	lang := req.Language
	if lang == "" {
		lang = "English unless the user asks otherwise"
	}

	switch req.Action {
	case "compose":
		return system, fmt.Sprintf("Compose an email. Tone: %s. Language: %s. User request: %s", tone, lang, req.Prompt)
	case "reply":
		return system, fmt.Sprintf("Draft ONLY the reply body that can be inserted directly into the composer. Tone: %s. Language: %s. Extra instruction: %s\n\nRules:\n- Do not quote or repeat the original email.\n- Do not include explanations, labels, markdown fences, or meta-instructions.\n- Do not say to confirm/send in Roundcube.\n- Start with the actual reply text.\n\nEMAIL/TEXT:\n%s", tone, lang, req.Prompt, req.Text)
	case "summarize":
		return system, fmt.Sprintf("Summarize this email/thread. Include key points, important dates, decisions, and action items. Language: %s.\n\nEMAIL/TEXT:\n%s", lang, req.Text)
	case "translate":
		return system, fmt.Sprintf("Translate this email/text to %s. Preserve meaning and formatting where practical.\n\nEMAIL/TEXT:\n%s", lang, req.Text)
	case "phishing":
		return system, fmt.Sprintf("Analyze this email for phishing, scam, impersonation, malicious links, payment red flags, urgency manipulation, and suspicious attachments. Return: Risk Level, Reasons, Safe Next Steps.\n\nEMAIL/TEXT:\n%s", req.Text)
	case "ask":
		return system, fmt.Sprintf("Answer the user's question using only the provided email/thread. If the answer is not present, say you cannot find it in the email. Question: %s\n\nEMAIL/TEXT:\n%s", req.Prompt, req.Text)
	case "chat":
		contextJSON := "{}"
		if req.Context != nil {
			if encoded, err := json.Marshal(req.Context); err == nil {
				contextJSON = string(encoded)
			}
		}
		return system, fmt.Sprintf("You are inside Roundcube webmail. Use only the mailbox context and email text provided. Language: %s. Tone: %s.\n\nRules:\n- If the user asks to draft/reply/respond, return ONLY the ready-to-send draft body. Do not quote the original email. Do not include explanations or meta-instructions.\n- If the user asks to send, delete, move, or mark mail, do not perform it; state that they must confirm that action in Roundcube.\n- If the user asks to navigate, suggest the safe Roundcube action.\n\nUSER MESSAGE:\n%s\n\nEMAIL/TEXT:\n%s\n\nMAILBOX CONTEXT JSON:\n%s", lang, tone, req.Prompt, req.Text, contextJSON)
	default:
		return system, fmt.Sprintf("Help with this email. Instruction: %s\n\nEMAIL/TEXT:\n%s", req.Prompt, req.Text)
	}
}

func mockResult(req AIRequest) string {
	switch req.Action {
	case "compose":
		return "Mock AI compose result. Add OPENAI_API_KEY and leave AI_MOCK_MODE=auto to enable real AI.\n\nSubject: Sample professional email\n\nHello,\n\nThank you for your message. I will review the details and get back to you with the next steps.\n\nBest regards,"
	case "reply":
		return "Mock AI reply result. Real AI is disabled.\n\nHello,\n\nThank you for the update. Noted on the details. I will coordinate with the team and respond once we have confirmed the next action.\n\nBest regards,"
	case "summarize":
		return "Mock AI summary result.\n\nKey points:\n- This is a placeholder summary.\n- Add OPENAI_API_KEY and leave AI_MOCK_MODE=auto for real summaries.\n\nAction items:\n- Configure your AI provider in .env."
	case "translate":
		return "Mock AI translation result. Real translation will work after configuring your AI API key."
	case "phishing":
		return "Mock phishing analysis.\n\nRisk Level: Unknown / Test Mode\nReasons: Real AI is disabled.\nSafe Next Steps: Enable real AI before using this for security review."
	case "ask":
		return "Mock answer. Real email Q&A will work after configuring your AI provider."
	case "chat":
		return "Mock chat answer. Real mailbox-aware chat will work after configuring your AI provider."
	default:
		return "Mock AI result."
	}
}

func callOpenAICompatible(ctx context.Context, cfg Config, systemPrompt, userPrompt string) (string, error) {
	payload := openAIChatRequest{
		Model: cfg.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("AI provider returned non-JSON response with status %d", resp.StatusCode)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("AI provider error: %s", parsed.Error.Message)
		}
		return "", fmt.Errorf("AI provider HTTP status %d", resp.StatusCode)
	}

	if len(parsed.Choices) == 0 || strings.TrimSpace(parsed.Choices[0].Message.Content) == "" {
		return "", fmt.Errorf("AI provider returned empty response")
	}

	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func callOpenAICompatibleStream(ctx context.Context, cfg Config, systemPrompt, userPrompt string, writeChunk func(string) error) error {
	payload := openAIChatRequest{
		Model: cfg.Model,
		Messages: []openAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.2,
		Stream:      true,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		var parsed openAIChatResponse
		if err := json.Unmarshal(respBody, &parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
			return fmt.Errorf("AI provider error: %s", parsed.Error.Message)
		}
		return fmt.Errorf("AI provider HTTP status %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}

		var parsed openAIStreamResponse
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return fmt.Errorf("AI provider returned invalid stream chunk")
		}
		if parsed.Error != nil && parsed.Error.Message != "" {
			return fmt.Errorf("AI provider error: %s", parsed.Error.Message)
		}

		for _, choice := range parsed.Choices {
			if choice.Delta.Content != "" {
				if err := writeChunk(choice.Delta.Content); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func streamMockResult(ctx context.Context, req AIRequest, writeChunk func(string) error) {
	parts := strings.SplitAfter(mockResult(req), " ")
	for _, part := range parts {
		select {
		case <-ctx.Done():
			return
		default:
			if err := writeChunk(part); err != nil {
				return
			}
			time.Sleep(18 * time.Millisecond)
		}
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", getEnv("AI_CORS_ORIGIN", "*"))
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-ITBS-AI-SECRET")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return value
}

func getMockMode(key, apiKey string) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	hasAPIKey := strings.TrimSpace(apiKey) != ""

	if raw == "" || raw == "auto" {
		return !hasAPIKey
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return !hasAPIKey
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func limitString(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
