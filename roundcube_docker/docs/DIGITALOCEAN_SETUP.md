# DigitalOcean Setup Guide for `mail.itbsstudio.com`

## Recommended DigitalOcean services

Use DigitalOcean for the webmail container and app services:

```text
Droplet or App Platform: Roundcube webmail
Managed Database: optional, can replace the MariaDB container later
Spaces: future attachment/avatar/logo storage
Cloudflare: DNS, WAF, SSL, caching, rate limiting
```

## Do not use DigitalOcean as the actual mail server first

DigitalOcean Droplets commonly block outbound SMTP ports. For production email, use an external mail provider or a mail-friendly VPS for the actual IMAP/SMTP service.

Good backend choices:

```text
Google Workspace
Zoho Mail
Microsoft 365
Mailcow on a mail-friendly VPS
Mailu on a mail-friendly VPS
Modoboa on a mail-friendly VPS
```

## DNS example

```dns
mail.itbsstudio.com       A      <DigitalOcean Droplet IP>
api.mail.itbsstudio.com   A      <DigitalOcean Droplet IP>
admin.mail.itbsstudio.com A      <DigitalOcean Droplet IP>
```

For email delivery, use the MX, SPF, DKIM, and DMARC records provided by your mail backend.

## Nginx reverse proxy example

```nginx
server {
    listen 80;
    server_name mail.itbsstudio.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Then use Certbot or Cloudflare Full SSL.

## Deploy commands

```bash
sudo apt update
sudo apt install -y docker.io docker-compose-plugin unzip
sudo systemctl enable --now docker

unzip itbs-pnp-roundcube-docker.zip
cd itbs_pnp_roundcube_docker
cp .env.example .env
nano .env

docker compose up -d --build
```

Check logs:

```bash
docker compose logs -f roundcube
```
