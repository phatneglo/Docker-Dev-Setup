# AWS + Cloudflare DNS Setup for `itbsstudio.com`

This guide uses these hostnames:

```text
mail.itbsstudio.com     canonical mail server hostname, MX target, SMTP, IMAP
webmail.itbsstudio.com  browser URL for Roundcube
admin.mail.itbsstudio.com optional private mail-admin UI
api.mail.itbsstudio.com optional AI/API hostname
```

Best practice is to keep `mail.itbsstudio.com` as the real mail host and use `webmail.itbsstudio.com` for the webmail app. Do not put the MX target behind the Cloudflare orange-cloud proxy.

## Recommended production shape

```text
Cloudflare DNS
  |
  |-- webmail.itbsstudio.com -> AWS EC2 / load balancer -> reverse proxy -> Roundcube :8080
  |
  |-- mail.itbsstudio.com    -> AWS EC2 Elastic IP -> docker-mailserver ports 25, 465, 587, 993
```

For a small deployment, one EC2 instance can run the full Docker Compose stack. For a cleaner production deployment, split mail delivery from the web app or use Amazon SES for outbound relay.

## Important AWS mail limitation

AWS restricts outbound SMTP port `25` from EC2 by default. Inbound port `25` can receive mail, but sending directly to other mail servers on port `25` usually requires an AWS request to remove the restriction.

Production options:

```text
Option A, direct mailserver:
  - Request AWS removal of outbound port 25 restriction.
  - Use an Elastic IP.
  - Set reverse DNS/PTR for the Elastic IP to mail.itbsstudio.com.
  - Make sure mail.itbsstudio.com A record points back to that same Elastic IP.

Option B, recommended for deliverability:
  - Receive mail on this docker-mailserver.
  - Send outbound mail through Amazon SES SMTP on 587.
  - Keep SPF/DKIM/DMARC aligned with SES and this server.
```

Relevant AWS references:

```text
https://repost.aws/knowledge-center/ec2-port-25-throttle
https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Elastic_Addressing_Reverse_DNS.html
https://docs.aws.amazon.com/ses/latest/DeveloperGuide/verify-domain-procedure.html
https://docs.aws.amazon.com/ses/latest/dg/send-email-authentication-dmarc.html
```

## AWS checklist

1. Create an EC2 instance.
   - Ubuntu 22.04/24.04 LTS is fine.
   - Use a fixed-size instance with enough memory for mail scanning if you later enable SpamAssassin/ClamAV.

2. Attach an Elastic IP.
   - Do not use a changing public IP for mail.
   - Use this Elastic IP in Cloudflare DNS.

3. Configure AWS Security Group inbound rules.

```text
TCP 22    your office/VPN IP only
TCP 80    0.0.0.0/0 and ::/0, for HTTP challenge or redirect
TCP 443   0.0.0.0/0 and ::/0, for webmail HTTPS
TCP 25    0.0.0.0/0 and ::/0, inbound SMTP delivery
TCP 465   0.0.0.0/0 and ::/0, SMTPS clients if enabled
TCP 587   0.0.0.0/0 and ::/0, authenticated submission
TCP 993   0.0.0.0/0 and ::/0, IMAPS clients
```

Avoid exposing plain IMAP `143` publicly in production. Keep it internal unless you have a specific compatibility reason.

4. Configure reverse DNS/PTR.
   - PTR for the Elastic IP should be `mail.itbsstudio.com`.
   - Forward DNS must match: `mail.itbsstudio.com A <Elastic-IP>`.

5. Request outbound port `25` removal if sending directly.
   - Explain that this is a legitimate business mail server.
   - Include the AWS Region, Elastic IP, domain, abuse handling process, and anti-spam controls.
   - If AWS denies it, use SES or another SMTP relay for outbound mail.

## EC2 first-run flow

On the EC2 instance:

```bash
sudo apt update
sudo apt install -y docker.io docker-compose-plugin certbot nginx
sudo systemctl enable --now docker
```

Copy the project to the server, then create the production env file:

```bash
cd roundcube_docker
cp remote.env.example remote.env
nano remote.env
```

Before starting with `MAIL_SSL_TYPE=letsencrypt`, create certificates for the public names:

```bash
sudo certbot certonly --standalone \
  -d mail.itbsstudio.com \
  -d webmail.itbsstudio.com

mkdir -p data/letsencrypt
sudo rsync -a /etc/letsencrypt/ data/letsencrypt/
sudo chown -R "$USER:$USER" data/letsencrypt
```

Then start Docker:

```bash
docker compose --env-file remote.env up -d --build
```

If you need to test Docker before certificates exist, temporarily set this in `remote.env`:

```env
MAIL_SSL_TYPE=
ROUNDCUBE_FORCE_HTTPS=false
```

Switch both values back after Nginx and certificates are ready.

## Cloudflare DNS records

In Cloudflare, keep mail records as **DNS only**. Cloudflare can proxy HTTP/HTTPS, but it does not proxy standard SMTP/IMAP mail traffic on normal DNS records.

Replace `<AWS_ELASTIC_IP>` with the EC2 Elastic IP.

```dns
Type  Name                  Value                         Proxy
A     mail                  <AWS_ELASTIC_IP>              DNS only
A     webmail               <AWS_ELASTIC_IP>              Proxied or DNS only
A     admin.mail            <AWS_ELASTIC_IP>              DNS only, or restrict by VPN/IP
A     api.mail              <AWS_ELASTIC_IP>              Proxied or DNS only
MX    @                     mail.itbsstudio.com  priority 10
TXT   @                     v=spf1 mx ip4:<AWS_ELASTIC_IP> -all
TXT   _dmarc                v=DMARC1; p=none; rua=mailto:dmarc@itbsstudio.com; fo=1
CAA   @                     0 issue "letsencrypt.org"
CAA   @                     0 issue "amazon.com"
```

If you use Amazon SES for outbound sending, use an SPF record like this instead:

```dns
TXT @ v=spf1 mx ip4:<AWS_ELASTIC_IP> include:amazonses.com -all
```

Use only one SPF TXT record for the root domain. Multiple SPF records at the same name can break SPF validation.

Relevant Cloudflare references:

```text
https://developers.cloudflare.com/dns/manage-dns-records/reference/proxied-dns-records/
https://www.cloudflare.com/learning/dns/dns-records/dns-mx-record/
https://developers.cloudflare.com/dmarc-management/security-records/
```

## DKIM setup

DKIM is required for reliable delivery.

For docker-mailserver, generate DKIM keys on the server after the stack is installed:

```bash
docker compose exec mailserver setup config dkim
```

Then find the generated DNS TXT value under:

```text
data/mailserver/config/opendkim/keys/itbsstudio.com/
```

Add the generated selector record in Cloudflare. It usually looks like:

```dns
Type  Name                                  Value
TXT   mail._domainkey.itbsstudio.com        v=DKIM1; k=rsa; p=<public-key>
```

If the selector is not `mail`, use the selector generated by docker-mailserver.

If using Amazon SES for outbound mail, add the SES-provided DKIM CNAME records too. SES gives these records during domain identity verification.

## DMARC rollout

Start with monitoring:

```dns
TXT _dmarc v=DMARC1; p=none; rua=mailto:dmarc@itbsstudio.com; fo=1
```

After SPF and DKIM pass consistently, tighten the policy:

```dns
TXT _dmarc v=DMARC1; p=quarantine; rua=mailto:dmarc@itbsstudio.com; fo=1; pct=50
```

Then move to reject:

```dns
TXT _dmarc v=DMARC1; p=reject; rua=mailto:dmarc@itbsstudio.com; fo=1
```

Create the reporting mailbox first:

```text
dmarc@itbsstudio.com
```

Also create these standard operational mailboxes:

```text
postmaster@itbsstudio.com
abuse@itbsstudio.com
admin@itbsstudio.com
```

## Optional but recommended records

Autoconfig records help mail clients discover settings:

```dns
CNAME autoconfig             mail.itbsstudio.com
CNAME autodiscover           mail.itbsstudio.com
```

TLS reporting:

```dns
TXT _smtp._tls               v=TLSRPTv1; rua=mailto:tlsrpt@itbsstudio.com
```

MTA-STS requires an HTTPS policy file at:

```text
https://mta-sts.itbsstudio.com/.well-known/mta-sts.txt
```

DNS:

```dns
A   mta-sts                  <AWS_ELASTIC_IP>
TXT _mta-sts                 v=STSv1; id=2026060901
```

Policy file:

```text
version: STSv1
mode: testing
mx: mail.itbsstudio.com
max_age: 86400
```

After testing, change `mode: testing` to `mode: enforce` and increase `max_age`.

## Application `.env` values

For this repository, use the committed templates and keep the real files private:

```bash
# Local machine
cp local.env.example local.env
docker compose --env-file local.env up -d --build

# EC2 server
cp remote.env.example remote.env
nano remote.env
docker compose --env-file remote.env up -d --build
```

The real files are ignored by git:

```text
local.env
remote.env
.env
```

Production values in `remote.env` should include:

```env
MAIL_HOSTNAME=mail
MAIL_DOMAIN=itbsstudio.com
MAIL_FQDN=mail.itbsstudio.com

ROUNDCUBEMAIL_USERNAME_DOMAIN=itbsstudio.com
ROUNDCUBEMAIL_DEFAULT_HOST=mailserver
ROUNDCUBEMAIL_DEFAULT_PORT=143
ROUNDCUBEMAIL_SMTP_SERVER=mailserver
ROUNDCUBEMAIL_SMTP_PORT=587

MAIL_SSL_TYPE=letsencrypt
MAIL_ADMIN_COOKIE_SECURE=true
ROUNDCUBE_FORCE_HTTPS=true
```

Roundcube can still connect to `mailserver:143` internally over the Docker network. Public users should connect over HTTPS to `webmail.itbsstudio.com`, and mail clients should use `mail.itbsstudio.com` with TLS ports.

The `remote.env.example` template binds web/admin/API ports to localhost:

```env
ROUNDCUBE_HTTP_PORT=127.0.0.1:8080
MAIL_ADMIN_UI_PORT=127.0.0.1:8092
AI_API_HTTP_PORT=127.0.0.1:8091
MAIL_IMAP_PORT=127.0.0.1:143
```

That means only SMTP/submission/SMTPS/IMAPS are directly public. `webmail.itbsstudio.com` should go through Nginx/Caddy/Traefik on `443` to `127.0.0.1:8080`.

Change every default secret before production:

```text
MYSQL_ROOT_PASSWORD
ROUNDCUBEMAIL_DB_PASSWORD
ROUNDCUBE_DES_KEY
ITBS_AI_SHARED_SECRET
MAIL_ADMIN_UI_PASSWORD
MAIL_ADMIN_PASSWORD
```

## Reverse proxy example

Terminate HTTPS on Nginx or Caddy and proxy to Roundcube.

Example Nginx config:

```nginx
server {
    listen 80;
    server_name webmail.itbsstudio.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name webmail.itbsstudio.com;

    ssl_certificate /etc/letsencrypt/live/webmail.itbsstudio.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/webmail.itbsstudio.com/privkey.pem;

    client_max_body_size 25m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

Do not proxy SMTP, submission, or IMAP through this HTTP reverse proxy.

## Cloudflare SSL mode

Use one of these:

```text
Full strict:
  - Recommended.
  - Requires a valid certificate on the AWS origin.

DNS only for webmail:
  - Also valid.
  - Lets users connect directly to the AWS origin certificate.
```

Do not use Flexible SSL for webmail. It can cause redirect loops and leaves the origin leg unencrypted.

## Legal and compliance checklist

This is not legal advice, but these are the minimum operational controls to run mail responsibly:

```text
- Send only legitimate business mail for domains you own or control.
- Do not send purchased-list, scraped-list, or unsolicited bulk email.
- Keep consent/opt-in records for any marketing mail.
- Include a real business identity and unsubscribe process for marketing mail.
- Monitor abuse@ and postmaster@ mailboxes.
- Use strong mailbox passwords and require TLS for client access.
- Remove compromised accounts immediately.
- Keep logs long enough to investigate abuse.
- Follow AWS Acceptable Use Policy and SES sending policies if using SES.
- Follow applicable laws for your recipients, such as CAN-SPAM, GDPR, and local privacy rules.
```

For newsletters or marketing campaigns, use a dedicated email marketing provider instead of this mailbox server.

## Verification commands

Run these after DNS is published:

```bash
dig +short A mail.itbsstudio.com
dig +short A webmail.itbsstudio.com
dig +short MX itbsstudio.com
dig +short TXT itbsstudio.com
dig +short TXT _dmarc.itbsstudio.com
dig +short TXT mail._domainkey.itbsstudio.com
```

Check open ports:

```bash
nc -vz mail.itbsstudio.com 25
nc -vz mail.itbsstudio.com 587
nc -vz mail.itbsstudio.com 993
nc -vz webmail.itbsstudio.com 443
```

Check TLS:

```bash
openssl s_client -connect mail.itbsstudio.com:993 -servername mail.itbsstudio.com
openssl s_client -starttls smtp -connect mail.itbsstudio.com:587 -servername mail.itbsstudio.com
```

Check mail authentication with an external tester:

```text
https://www.mail-tester.com/
https://mxtoolbox.com/
https://dmarcian.com/dmarc-inspector/
```

## Final DNS target example

For the requested domain, the preferred final public setup is:

```text
Roundcube browser URL:
  https://webmail.itbsstudio.com

Mail client incoming server:
  mail.itbsstudio.com
  IMAPS port 993

Mail client outgoing server:
  mail.itbsstudio.com
  Submission port 587 with STARTTLS

Domain MX:
  itbsstudio.com MX 10 mail.itbsstudio.com
```
