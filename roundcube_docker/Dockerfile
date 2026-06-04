# ITBS PNP Mail - Custom Roundcube Docker Image
# Base: Official Roundcube Docker image.
# The official image supports extra config files in /var/roundcube/config
# and post setup scripts in /entrypoint-tasks/post-setup/.
FROM roundcube/roundcubemail:1.6.x-apache

USER root

COPY roundcube/config/itbs-pnp.php /var/roundcube/config/itbs-pnp.php
COPY roundcube/assets/ /opt/itbs-pnp/assets/
COPY roundcube/plugins/itbs_ai_assistant/ /var/www/html/plugins/itbs_ai_assistant/
COPY roundcube/post-setup/99-itbs-pnp-theme.sh /entrypoint-tasks/post-setup/99-itbs-pnp-theme.sh
COPY roundcube/php/zzz_itbs_custom.ini /usr/local/etc/php/conf.d/zzz_itbs_custom.ini

RUN chmod +x /entrypoint-tasks/post-setup/99-itbs-pnp-theme.sh \
    && mkdir -p /opt/itbs-pnp/assets
