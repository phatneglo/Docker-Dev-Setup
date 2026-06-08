# Go AI API

The Go service is located here:

```text
ai-api/
```

## Run locally

```bash
cd ai-api
go test ./...
go run ./cmd/server
```

Open:

```text
http://localhost:8090/healthz
```

## Endpoints

```text
GET  /healthz
POST /v1/ai/compose
POST /v1/ai/reply
POST /v1/ai/summarize
POST /v1/ai/translate
POST /v1/ai/phishing
POST /v1/ai/ask
```

Each action also supports a streaming route:

```text
POST /v1/ai/{action}/stream
```

Streaming responses return `text/plain` chunks and include `X-ITBS-AI-Mock` to indicate whether mock mode handled the response.

## Example request

```bash
curl -X POST http://localhost:8090/v1/ai/summarize \
  -H "Content-Type: application/json" \
  -H "X-ITBS-AI-SECRET: change_me_ai_shared_secret" \
  -d '{"text":"Please attend the meeting tomorrow at 9 AM.","language":"English"}'
```

## Environment variables

```env
AI_API_HTTP_PORT=8090
AI_BASE_URL=https://api.openai.com/v1
AI_MODEL=gpt-4o-mini
AI_MOCK_MODE=auto
AI_MAX_INPUT_CHARS=12000
AI_TIMEOUT_SECONDS=60
AI_SHARED_SECRET=change_me_ai_shared_secret
OPENAI_API_KEY=
```

## Mock mode

Mock mode is enabled by default so you can test the UI even without an AI key.

```env
AI_MOCK_MODE=auto
```

For real AI:

```env
AI_MOCK_MODE=auto
OPENAI_API_KEY=your_api_key_here
```
