# ITBS PNP Mail - All-in-one Mail + Roundcube Stack

This package gives you a ready-to-run **custom branded mail stack** for `itbsstudio.com`.

It includes:

- Docker Mailserver for SMTP + IMAP
- Dockerfile
- Docker Compose
- Go AI API microservice
- Roundcube AI Assistant plugin
- MariaDB container for Roundcube metadata
- Custom PNP-style placeholder logo
- Custom Roundcube Elastic theme overlay
- `.env` and `.env.example`
- repo-local `data/` folders for persistent mail and app state
- DigitalOcean deployment notes
- Theme customization guide
- Validation script
- AI compose, reply, summarize, translate, ask, and phishing-risk endpoints

The stack now runs its own IMAP/SMTP service, keeps the custom Roundcube layer, and stores all persistent state in local folders that are easy to back up.

## Quick start

```bash
docker compose up -d --build
```

Open:

```text
http://localhost:8080
```

The stack bootstraps the first mailbox automatically from `.env`:

```env
MAIL_ADMIN_ADDRESS=admin@itbsstudio.com
MAIL_ADMIN_PASSWORD=ChangeMeNow123!
```

Then log in to Roundcube with that mailbox and password. Use the helper script only for additional mailboxes.

## Services and ports

```text
Roundcube UI:        http://localhost:8080
AI API host bind:    http://localhost:8091
SMTP:                localhost:25
Submission:          localhost:587
IMAP:                localhost:143
SMTPS/IMAPS:         enable after real TLS is configured
```

## Persistent data

Everything important is stored under:

```text
data/
  mailserver/
    config/
    mail-data/
    mail-logs/
    mail-state/
  mariadb/
  roundcube/
    db/
    temp/
```

Backups are just those folders plus your `.env`.

## Domain plan

Recommended public hostnames:

```text
mail.itbsstudio.com       -> this Roundcube webmail app
api.mail.itbsstudio.com   -> optional external AI/API hostname
```

## Files you usually customize

```text
.env
data/mailserver/config
roundcube/assets/custom.css
roundcube/assets/logo.svg
roundcube/assets/logo-small.svg
roundcube/config/itbs-pnp.php
```

## Change the logo

Replace:

```text
roundcube/assets/logo.svg
roundcube/assets/logo-small.svg
```

Then rebuild:

```bash
docker compose up -d --build
```

## Change colors

Edit:

```text
roundcube/assets/custom.css
```

Main variables:

```css
:root {
  --itbs-primary: #0B2E59;
  --itbs-primary-2: #0F4C81;
  --itbs-gold: #F2C94C;
  --itbs-red: #C62828;
}
```

## Production notes

For local/dev, the repo bootstraps one mailbox automatically and Roundcube connects to the internal mailserver over TLS on IMAPS and Submission.

For production, you still need proper DNS and a real TLS setup for mail delivery.

Also change:

```text
MYSQL_ROOT_PASSWORD
ROUNDCUBEMAIL_DB_PASSWORD
ROUNDCUBE_DES_KEY
ITBS_AI_SHARED_SECRET
```

## Validate the package locally

```bash
python3 -m venv .venv
. .venv/bin/activate
python scripts/validate_project.py
```

The script checks the important files, config, CSS, SVG files, helper script, and Docker Compose references.

## AI functionality included

This package now includes a Go AI API service and a Roundcube plugin.

Included AI endpoints:

```text
GET  /healthz
POST /v1/ai/compose
POST /v1/ai/reply
POST /v1/ai/summarize
POST /v1/ai/translate
POST /v1/ai/phishing
POST /v1/ai/ask
```

The Roundcube plugin adds a floating **PNP Mail AI** panel inside the mailbox UI.
It can compose, draft replies, summarize email content, translate, ask questions, and check phishing risk.

By default, AI runs in mock mode so you can test the UI without paying for API calls:

```env
AI_MOCK_MODE=true
OPENAI_API_KEY=
```

To enable real AI:

```env
AI_MOCK_MODE=false
OPENAI_API_KEY=your_api_key_here
AI_MODEL=gpt-4o-mini
```

You can also test only the Go API locally:

```bash
cd ai-api
go test ./...
go run ./cmd/server
```

Then test the host bind:

```bash
curl http://localhost:8091/healthz
```

See:

```text
docs/AI_INTEGRATION_PLAN.md
docs/GO_AI_API.md
docs/MAIL_BACKEND_NOTES.md
```
