#!/usr/bin/env python3
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[1]

required_files = [
    'Dockerfile',
    'docker-compose.yml',
    '.env.example',
    '.env',
    'README.md',
    'roundcube/config/itbs-pnp.php',
    'roundcube/assets/custom.css',
    'roundcube/assets/logo.svg',
    'roundcube/assets/logo-small.svg',
    'roundcube/assets/login-bg.svg',
    'roundcube/post-setup/99-itbs-pnp-theme.sh',
    'roundcube/php/zzz_itbs_custom.ini',
    'roundcube/plugins/itbs_ai_assistant/itbs_ai_assistant.php',
    'roundcube/plugins/itbs_ai_assistant/itbs_ai_assistant.js',
    'roundcube/plugins/itbs_ai_assistant/itbs_ai_assistant.css',
    'ai-api/Dockerfile',
    'ai-api/go.mod',
    'ai-api/cmd/server/main.go',
    'preview/theme-preview.html',
    'scripts/add-mail-user.ps1',
]

errors = []
for file in required_files:
    if not (ROOT / file).exists():
        errors.append(f'Missing required file: {file}')

compose = (ROOT / 'docker-compose.yml').read_text(encoding='utf-8')
dockerfile = (ROOT / 'Dockerfile').read_text(encoding='utf-8')
for token in ['roundcube/roundcubemail', 'ghcr.io/docker-mailserver/docker-mailserver:latest', 'mariadb:11', 'ai-api', 'ITBS_AI_API_URL', 'ROUNDCUBEMAIL_DEFAULT_HOST', 'ROUNDCUBEMAIL_SMTP_SERVER', 'mail-bootstrap', 'MAIL_ADMIN_ADDRESS', './data/mailserver/mail-data:/var/mail']:
    if token not in compose and token not in dockerfile:
        errors.append(f'Missing expected Docker token: {token}')

css = (ROOT / 'roundcube/assets/custom.css').read_text(encoding='utf-8')
for token in ['--itbs-primary', '--itbs-gold', '.task-login', 'itbs-pnp']:
    if token not in css:
        errors.append(f'Missing expected CSS token: {token}')

php = (ROOT / 'roundcube/config/itbs-pnp.php').read_text(encoding='utf-8')
for token in ['$config', 'product_name', 'skin_logo', 'support_url', 'itbs_ai_api_url']:
    if token not in php:
        errors.append(f'Missing expected PHP config token: {token}')

ai_php = (ROOT / 'roundcube/plugins/itbs_ai_assistant/itbs_ai_assistant.php').read_text(encoding='utf-8')
for token in ['class itbs_ai_assistant', 'plugin.itbs_ai', 'curl_init', 'X-ITBS-AI-SECRET']:
    if token not in ai_php:
        errors.append(f'Missing expected AI plugin token: {token}')

ai_js = (ROOT / 'roundcube/plugins/itbs_ai_assistant/itbs_ai_assistant.js').read_text(encoding='utf-8')
for token in ['PNP Mail AI', 'compose', 'summarize', 'phishing', 'rcmail.url']:
    if token not in ai_js:
        errors.append(f'Missing expected AI JS token: {token}')

ai_go = (ROOT / 'ai-api/cmd/server/main.go').read_text(encoding='utf-8')
for token in ['/v1/ai/', 'OPENAI_API_KEY', 'AI_MOCK_MODE', 'chat/completions']:
    if token not in ai_go:
        errors.append(f'Missing expected Go AI token: {token}')

for svg_name in ['logo.svg', 'logo-small.svg', 'login-bg.svg']:
    svg_bytes = (ROOT / 'roundcube/assets' / svg_name).read_bytes()
    if not svg_bytes:
        errors.append(f'{svg_name} is empty')

script = ROOT / 'roundcube/post-setup/99-itbs-pnp-theme.sh'
if sys.platform != 'win32' and not script.stat().st_mode & 0o111:
    errors.append('Post setup script is not executable')

if errors:
    print('Validation failed:')
    for err in errors:
        print(f' - {err}')
    sys.exit(1)

print('Validation passed. Project files look ready for Docker build/run with mailserver + Roundcube theme + Go AI API.')
