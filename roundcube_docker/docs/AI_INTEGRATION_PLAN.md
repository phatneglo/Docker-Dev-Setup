# AI Integration Plan

This project includes a working AI foundation:

1. `ai-api` - Go microservice for AI features.
2. `roundcube/plugins/itbs_ai_assistant` - Roundcube plugin that calls the Go service.
3. `.env` - AI settings and shared secret.
4. `docker-compose.yml` - runs Roundcube, MariaDB, and the AI API together.

## Current AI features

```text
Compose email
Draft reply
Summarize email/thread
Translate email
Ask about selected email content
Check phishing/scam risk
```

## How it works

```text
User opens Roundcube
  ↓
PNP Mail AI panel appears
  ↓
User selects text/email content and clicks AI action
  ↓
Roundcube plugin sends request to PHP proxy action
  ↓
PHP plugin calls internal Go API at http://ai-api:8090
  ↓
Go API calls OpenAI-compatible /chat/completions endpoint
  ↓
Result returns to the Roundcube UI
```

## Why Go API is used

The Go API keeps AI logic outside Roundcube. This makes it easier to add:

```text
Rate limiting
Audit logs
Prompt templates
Tenant/account permissions
AI usage billing
RAG/vector search later
Multiple AI providers
```

## Real AI setup

Edit `.env`:

```env
AI_MOCK_MODE=auto
OPENAI_API_KEY=your_api_key_here
AI_MODEL=gpt-4o-mini
```

Then run:

```bash
docker compose up -d --build
```

## Security

Roundcube calls the Go API using `ITBS_AI_SHARED_SECRET` / `AI_SHARED_SECRET`.
Change this before production.

Never put the AI provider API key in frontend JavaScript.
