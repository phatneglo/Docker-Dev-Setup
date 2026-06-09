# ITBS PNP Mail - All-in-one Mail + Roundcube Stack

This package gives you a ready-to-run **custom branded mail stack** for `itbsstudio.com`.

It includes:

- Docker Mailserver for SMTP + IMAP
- Dockerfile
- Docker Compose
- Go AI API microservice
- Go + React mail admin UI
- Roundcube AI Assistant plugin
- MariaDB container for Roundcube metadata
- Custom PNP-style placeholder logo
- Custom Roundcube Elastic theme overlay
- `.env.example`, `local.env.example`, and `remote.env.example`
- repo-local `data/` folders for config/logs/app state plus Docker volumes for mail storage
- DigitalOcean deployment notes
- AWS + Cloudflare DNS setup guide
- Theme customization guide
- Validation script
- AI compose, reply, summarize, translate, ask, and phishing-risk endpoints

The stack now runs its own IMAP/SMTP service, keeps the custom Roundcube layer, and stores persistent state in local folders plus Docker volumes.

## Quick start

Local development:

```bash
cp local.env.example local.env
docker compose --env-file local.env up -d --build
```

Open:

```text
http://localhost:8080
```

Mail admin UI:

```text
http://localhost:8092
```

AWS EC2 production template:

```bash
cp remote.env.example remote.env
nano remote.env
docker compose --env-file remote.env up -d --build
```

Keep `local.env`, `remote.env`, and `.env` out of git. Only the `.example` templates should be committed.

The stack bootstraps the first mailbox automatically from your selected env file:

```env
MAIL_ADMIN_ADDRESS=admin@itbsstudio.com
MAIL_ADMIN_PASSWORD=ChangeMeNow123!
```

Then log in to Roundcube with that mailbox and password. Use the helper script only for additional mailboxes.

The mail admin UI uses its own admin login from your selected env file:

```env
MAIL_ADMIN_UI_EMAIL=admin@itbsstudio.com
MAIL_ADMIN_UI_PASSWORD=ChangeMeNow123!
```

Use it to add mailboxes, reset passwords, delete mailboxes, and set or remove mailbox quotas.

## Services and ports

```text
Roundcube UI:        http://localhost:8080
AI API host bind:    http://localhost:8091
Mail admin UI:       http://localhost:8092
SMTP:                localhost:25
Submission:          localhost:587
IMAP:                localhost:143
SMTPS/IMAPS:         enable after real TLS is configured
```

## Persistent data

Repo-local config, logs, and app data are stored under:

```text
data/
  mailserver/
    config/
    mail-logs/
  mariadb/
  roundcube/
    db/
    temp/
```

Mail messages and mailserver runtime state are stored in Docker named volumes:

```text
itbs-pnp-mail_mailserver-mail-data
itbs-pnp-mail_mailserver-mail-state
```

Backups should include those volumes, the `data/` folders, and your active env file.

## Domain plan

Recommended public hostnames:

```text
mail.itbsstudio.com       -> this Roundcube webmail app
api.mail.itbsstudio.com   -> optional external AI/API hostname
```

## Files you usually customize

```text
.env
local.env.example
remote.env.example
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

For local/dev, the repo bootstraps one mailbox automatically and Roundcube connects to the internal mailserver over plain IMAP on the Docker network. Enable real TLS before using IMAPS/SMTPS in production.

For production, you still need proper DNS and a real TLS setup for mail delivery.

Also change:

```text
MYSQL_ROOT_PASSWORD
ROUNDCUBEMAIL_DB_PASSWORD
ROUNDCUBE_DES_KEY
ITBS_AI_SHARED_SECRET
MAIL_ADMIN_UI_PASSWORD
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
POST /v1/ai/chat
```

The Roundcube plugin adds a FAB-launched **PNP Mail AI** chat inside the mailbox UI.
It can compose, draft replies, summarize open or selected email content, translate, ask questions about the current email or recent mailbox context, and check phishing risk. Responses stream into the panel as the AI generates them.

By default, AI runs in mock mode so you can test the UI without paying for API calls:

```env
AI_MOCK_MODE=auto
OPENAI_API_KEY=
```

To enable real AI:

```env
AI_MOCK_MODE=auto
OPENAI_API_KEY=your_api_key_here
AI_MODEL=gpt-4o-mini
```

Set `AI_MOCK_MODE=true` only when you want to force mock responses even with an API key configured.

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
docs/AWS_CLOUDFLARE_DNS_SETUP.md
docs/AI_INTEGRATION_PLAN.md
docs/GO_AI_API.md
docs/MAIL_BACKEND_NOTES.md
```
