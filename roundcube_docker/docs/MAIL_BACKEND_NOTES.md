# Mail Backend Notes

This package now includes its own mail server using `docker-mailserver`.

## What runs inside this stack

```text
mailserver  -> SMTP + IMAP
roundcube   -> webmail UI
db          -> Roundcube metadata database
ai-api      -> internal AI helper API
```

Roundcube now talks to the internal `mailserver` service by default:

```env
ROUNDCUBEMAIL_DEFAULT_HOST=ssl://mailserver
ROUNDCUBEMAIL_DEFAULT_PORT=993
ROUNDCUBEMAIL_SMTP_SERVER=tls://mailserver
ROUNDCUBEMAIL_SMTP_PORT=587
```

The stack also bootstraps one persistent mailbox on startup:

```env
MAIL_ADMIN_ADDRESS=admin@itbsstudio.com
MAIL_ADMIN_PASSWORD=ChangeMeNow123!
```

## Persistent data to back up

```text
data/mailserver/config
data/mailserver/mail-data
data/mailserver/mail-state
data/mailserver/mail-logs
data/mariadb
data/roundcube/db
data/roundcube/temp
```

## Creating mailboxes

The first mailbox is created automatically from `.env`. Add more mailboxes with:

```powershell
./scripts/add-mail-user.ps1 -Email admin@itbsstudio.com -Password "ChangeMeNow123!"
```

Equivalent raw command:

```powershell
docker compose exec mailserver setup email add admin@itbsstudio.com "ChangeMeNow123!"
```

## Local testing

For local/dev use, `.env` defaults to:

```env
MAIL_SSL_TYPE=
```

That allows the stack to bootstrap without external certificate files. It is acceptable for testing the stack. It is not enough for public mail delivery.

## Production requirements

You still need:

```text
A / AAAA for mail.itbsstudio.com
MX for itbsstudio.com -> mail.itbsstudio.com
SPF TXT
DKIM TXT
DMARC TXT
PTR / reverse DNS from your VPS provider
```

Many VPS providers also restrict outbound SMTP on port 25. If they do, inbound/outbound delivery will not work reliably until that is resolved.

## TLS for production

For production, replace the default self-signed mode with a proper certificate strategy supported by `docker-mailserver`, such as:

```env
MAIL_SSL_TYPE=manual
```

or

```env
MAIL_SSL_TYPE=letsencrypt
```

and provide the required certificate files / reverse proxy flow.

## DNS records needed for email

Your final deployment should publish:

```text
MX
SPF TXT
DKIM TXT
DMARC TXT
```

Start DMARC with monitoring:

```txt
v=DMARC1; p=none; rua=mailto:dmarc@itbsstudio.com; fo=1
```

After testing, move to quarantine/reject.
