#!/bin/sh
set -eu

PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PATH

image="${1:-}"
root="${STAGING_DEPLOY_PATH:-/opt/sub2api-staging}"

case "$image" in
  ghcr.io/qxsazz/model-gate-sub2api@sha256:????????????????????????????????????????????????????????????????) ;;
  *) echo "staging image must be a GHCR sha256 digest" >&2; exit 2 ;;
esac

if [ "$root" != "/opt/sub2api-staging" ]; then
  echo "refusing unexpected staging path: $root" >&2
  exit 2
fi

umask 077
mkdir -p "$root/data" "$root/postgres_data" "$root/redis_data" "$root/backups" "$root/releases"

if [ -f "$root/.env" ]; then
  chmod 600 "$root/.env"
  exit 0
fi

random_hex() {
  openssl rand -hex 32
}

postgres_password="$(random_hex)"
redis_password="$(random_hex)"
admin_password="$(random_hex)"
jwt_secret="$(random_hex)"
totp_key="$(random_hex)"

tmp_env="$root/.env.tmp.$$"
tmp_credentials="$root/credentials.txt.tmp.$$"
trap 'rm -f "$tmp_env" "$tmp_credentials"' EXIT HUP INT TERM

cat > "$tmp_env" <<EOF
SUB2API_IMAGE=$image
SERVER_PORT=8081
STAGING_PORT=8081
BIND_HOST=127.0.0.1
SERVER_MODE=release
RUN_MODE=standard
POSTGRES_USER=sub2api_staging
POSTGRES_DB=sub2api_staging
POSTGRES_PASSWORD=$postgres_password
REDIS_PASSWORD=$redis_password
ADMIN_EMAIL=staging-admin@model-gate.cc
ADMIN_PASSWORD=$admin_password
JWT_SECRET=$jwt_secret
TOTP_ENCRYPTION_KEY=$totp_key
TZ=Asia/Shanghai
SECURITY_URL_ALLOWLIST_ENABLED=true
SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=false
SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=false
EOF

cat > "$tmp_credentials" <<EOF
Staging credentials. Keep this file private.
Admin email: staging-admin@model-gate.cc
Admin password: $admin_password
Created: $(date -u +%Y-%m-%dT%H:%M:%SZ)
EOF

mv "$tmp_env" "$root/.env"
mv "$tmp_credentials" "$root/credentials.txt"
chmod 600 "$root/.env" "$root/credentials.txt"
trap - EXIT HUP INT TERM
