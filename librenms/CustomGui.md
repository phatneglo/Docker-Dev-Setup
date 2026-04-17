# LibreNMS Custom GUI Guide

This guide shows how to customize the default LibreNMS GUI for:

- logo and images
- CSS styling
- JavaScript behavior

It is written for this Docker setup in:

`D:\DOCKER_CONTAINER_FILES\librenms`

## Important Note

If you edit files directly inside the running container, your changes will be lost when the container is recreated or updated.

For quick testing, editing inside the container is fine.

For persistent changes, use one of these:

1. bind-mounted custom files
2. a custom Docker image
3. LibreNMS config values for supported branding options

## Current Useful Paths

Inside the LibreNMS container:

- logo component: `/opt/librenms/resources/views/components/logo.blade.php`
- login form: `/opt/librenms/resources/views/auth/login-form.blade.php`
- legacy CSS: `/opt/librenms/html/css/styles.css`
- logo images: `/opt/librenms/html/images/`
- built CSS bundle: `/opt/librenms/html/build/assets/`

## Best Supported Options

LibreNMS already supports:

- `title_image` for changing the logo
- `webui.custom_css` for adding custom CSS

These are the safest ways to brand the UI without patching core files.

## Change the Logo

Create a folder on the host:

```powershell
mkdir D:\DOCKER_CONTAINER_FILES\librenms\branding
```

Put your logo file there, for example:

- `D:\DOCKER_CONTAINER_FILES\librenms\branding\my-logo.svg`

Then mount it in [docker-compose.yml](/d:/DOCKER_CONTAINER_FILES/librenms/docker-compose.yml:1):

```yaml
services:
  librenms:
    volumes:
      - ./data/librenms:/data
      - ./branding:/opt/librenms/html/branding:ro
```

Recreate the stack:

```powershell
cd D:\DOCKER_CONTAINER_FILES\librenms
docker compose --env-file ..\librenms.env up -d
```

Then tell LibreNMS to use the mounted file:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set title_image branding/my-logo.svg
```

If you use PNG:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set title_image branding/my-logo.png
```

## Add Custom CSS

LibreNMS supports a CSS override directly from config:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set webui.custom_css ".logon-logo{max-height:90px!important}.navbar{background:#17324d!important}.btn-primary{background:#c2410c!important;border-color:#c2410c!important}body{background:#f6f1e8!important}"
```

Useful selectors:

- `.logon-logo`
- `.navbar`
- `.btn-primary`
- `.panel`
- `body`

To clear custom CSS:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set webui.custom_css
```

## Add Custom JavaScript

LibreNMS does not provide a simple built-in `custom_js` setting like it does for CSS.

If you want custom JavaScript, you usually need to:

1. override a Blade template
2. inject a `<script>` tag into a layout or login page
3. serve your own JS file from a mounted folder

The most likely files to override are:

- `/opt/librenms/resources/views/layouts/librenmsv1.blade.php`
- `/opt/librenms/resources/views/auth/login-form.blade.php`

Example concept:

```html
<script src="{{ asset('branding/custom.js') }}"></script>
```

Then mount a host folder containing `custom.js`:

```yaml
services:
  librenms:
    volumes:
      - ./branding:/opt/librenms/html/branding:ro
```

## Quick Test Inside the Container

For temporary testing only:

```powershell
docker exec -it librenms-librenms-1 sh
```

Examples:

```sh
vi /opt/librenms/resources/views/components/logo.blade.php
vi /opt/librenms/resources/views/auth/login-form.blade.php
vi /opt/librenms/html/css/styles.css
```

These changes are not persistent across container recreation.

## Persistent Customization Strategy

### Option 1: Supported Branding Only

Use:

- `title_image`
- `webui.custom_css`

This is the safest and easiest option.

### Option 2: Bind-Mounted Static Assets

Good for:

- custom logos
- custom images
- custom JS files

Mount a host folder into `/opt/librenms/html/branding`.

### Option 3: Custom Docker Image

Best if you want to modify:

- Blade templates
- layout files
- core CSS files
- shipped images

Basic example:

```dockerfile
FROM librenms/librenms:latest

COPY branding/my-logo.svg /opt/librenms/html/branding/my-logo.svg
COPY custom/login-form.blade.php /opt/librenms/resources/views/auth/login-form.blade.php
COPY custom/styles.css /opt/librenms/html/css/styles.css
```

Then build and use your own image in Compose.

## Suggested Folder Layout

You can organize files like this:

```text
librenms/
  branding/
    my-logo.svg
    custom.js
  custom/
    login-form.blade.php
    logo.blade.php
    styles.css
  docker-compose.yml
  README.md
  CustomGui.md
```

## Recommended Workflow

1. Start with `title_image` and `webui.custom_css`
2. If you need more, mount static files under `branding`
3. If you need template or JS injection changes, move to a custom image

## Example Branding Commands

Set logo:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set title_image branding/my-logo.svg
```

Set CSS:

```powershell
docker exec -u librenms librenms-librenms-1 php /opt/librenms/artisan config:set webui.custom_css ".logon-logo{max-height:100px!important}body{background:#faf7f0!important}.navbar{background:#1f3a5f!important}"
```

Reload container:

```powershell
cd D:\DOCKER_CONTAINER_FILES\librenms
docker compose --env-file ..\librenms.env up -d
```

## Notes

- Hard refresh your browser after CSS or image changes.
- If you recreate the container, direct in-container edits are lost.
- If you update LibreNMS, core file overrides may need to be rechecked.

## If You Want the Next Step

The next practical step is:

1. add a `branding` volume to Compose
2. place your custom logo and JS file in that folder
3. use `title_image` and `webui.custom_css`

If you want, the next change I can make is updating your Compose file to add the persistent `branding` mount.
