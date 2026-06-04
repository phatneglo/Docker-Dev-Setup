# Theme Customization Guide

## Main theme files

```text
roundcube/assets/custom.css
roundcube/assets/logo.svg
roundcube/assets/logo-small.svg
roundcube/assets/login-bg.svg
```

## Change brand name

Edit `.env`:

```env
ROUNDCUBEMAIL_PRODUCT_NAME=PNP Mail
ITBS_BRAND_NAME=PNP Mail
```

## Change colors

Edit `roundcube/assets/custom.css`:

```css
:root {
  --itbs-primary: #0B2E59;
  --itbs-primary-2: #0F4C81;
  --itbs-gold: #F2C94C;
  --itbs-red: #C62828;
}
```

## Replace placeholder logo

Put your logo here:

```text
roundcube/assets/logo.svg
roundcube/assets/logo-small.svg
```

Then rebuild:

```bash
docker compose up -d --build
```

## Preview without Docker

Open this file in your browser:

```text
preview/theme-preview.html
```

This preview is only for checking the visual style. The actual app runs through Docker/Roundcube.
