<?php
/**
 * ITBS PNP Mail custom Roundcube config.
 * This file is automatically included by the official Roundcube Docker image
 * from /var/roundcube/config/*.php.
 */

$brandName = getenv('ROUNDCUBEMAIL_PRODUCT_NAME') ?: getenv('ITBS_BRAND_NAME') ?: 'PNP Mail';
$supportEmail = getenv('ITBS_SUPPORT_EMAIL') ?: 'support@itbsstudio.com';

$config['product_name'] = $brandName;
$config['skin'] = 'elastic';

// Custom logo copied by roundcube/post-setup/99-itbs-pnp-theme.sh.
$config['skin_logo'] = [
    '*' => 'skins/elastic/images/itbs-pnp/logo.svg',
    'small' => 'skins/elastic/images/itbs-pnp/logo-small.svg',
];

$config['support_url'] = 'mailto:' . $supportEmail;
$config['login_autocomplete'] = 2;
$config['login_lc'] = 2;
$config['display_next'] = true;
$config['mail_pagesize'] = 50;
$config['addressbook_pagesize'] = 50;
$config['draft_autosave'] = 60;
$config['session_lifetime'] = 30;
$config['logout_purge'] = true;
$config['message_show_email'] = true;
$config['prefer_html'] = true;
$config['htmleditor'] = 1;

// Safer defaults for production-style deployments.
$config['force_https'] = false; // Put Cloudflare/Nginx/Traefik HTTPS in front and set this true only when ready.
$config['x_frame_options'] = 'sameorigin';
$config['des_key'] = getenv('ROUNDCUBE_DES_KEY') ?: 'change_this_24_char_key!!';

// ITBS AI Assistant plugin settings. The Roundcube plugin proxies browser requests
// to this internal Go API so the AI provider key is never exposed to users.
$config['itbs_ai_title'] = getenv('ITBS_AI_TITLE') ?: 'PNP Mail AI';
$config['itbs_ai_api_url'] = getenv('ITBS_AI_API_URL') ?: 'http://ai-api:8090';
$config['itbs_ai_shared_secret'] = getenv('ITBS_AI_SHARED_SECRET') ?: '';
