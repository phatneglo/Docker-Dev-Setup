#!/usr/bin/env bash
set -euo pipefail

RC_ROOT="${ROUNDCUBE_INSTALL_PATH:-/var/www/html}"
ELASTIC_DIR="$RC_ROOT/skins/elastic"
BRAND_DIR="$ELASTIC_DIR/images/itbs-pnp"
STYLE_FILE="$ELASTIC_DIR/styles/styles.css"
CUSTOM_STYLE="$ELASTIC_DIR/styles/itbs-pnp-custom.css"

mkdir -p "$BRAND_DIR"
cp /opt/itbs-pnp/assets/logo.svg "$BRAND_DIR/logo.svg"
cp /opt/itbs-pnp/assets/logo-small.svg "$BRAND_DIR/logo-small.svg"
cp /opt/itbs-pnp/assets/login-bg.svg "$BRAND_DIR/login-bg.svg"
cp /opt/itbs-pnp/assets/custom.css "$CUSTOM_STYLE"

if [ -f "$STYLE_FILE" ] && ! grep -q "itbs-pnp-custom.css" "$STYLE_FILE"; then
  cat >> "$STYLE_FILE" <<'EOF'

/* ITBS PNP Mail custom theme overlay */
@import url("itbs-pnp-custom.css");
EOF
fi

echo "ITBS PNP Mail theme installed into Roundcube Elastic skin."
