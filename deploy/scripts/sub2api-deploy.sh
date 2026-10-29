#!/bin/sh
set -eu

PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PATH

usage() {
  echo "Usage: $0 deploy <staging|production> <ghcr-image@sha256:digest>" >&2
  echo "       $0 verify-staging <ghcr-image@sha256:digest>" >&2
  echo "       $0 status <staging|production>" >&2
  echo "       $0 sync-image <staging|production> (persist the running image without restarting)" >&2
  exit 2
}

valid_digest() {
  case "$1" in
    ghcr.io/qxsazz/model-gate-sub2api@sha256:????????????????????????????????????????????????????????????????)
      digest="${1##*@sha256:}"
      case "$digest" in *[!0-9a-f]*) return 1 ;; *) return 0 ;; esac ;;
    *) return 1 ;;
  esac
}

configure_environment() {
  environment="$1"
  case "$environment" in
    staging)
      root="${STAGING_DEPLOY_PATH:-/opt/sub2api-staging}"
      project="sub2api-staging"
      container="sub2api-staging"
      port="8081"
      compose_args="-f $root/docker-compose.base.yml -f $root/docker-compose.staging.yml"
      ;;
    production)
      root="${PRODUCTION_DEPLOY_PATH:-/opt/sub2api}"
      project="sub2api"
      container="sub2api"
      port="8080"
      compose_args="-f $root/docker-compose.yml -f $root/docker-compose.cicd.yml"
      ;;
    *) echo "unknown environment: $environment" >&2; exit 2 ;;
  esac

  if [ "$environment" = "staging" ] && [ "$root" != "/opt/sub2api-staging" ]; then
    echo "refusing unexpected staging path: $root" >&2
    exit 2
  fi
  if [ "$environment" = "production" ] && [ "$root" != "/opt/sub2api" ]; then
    echo "refusing unexpected production path: $root" >&2
    exit 2
  fi
  env_file="$root/.env"
  release_dir="$root/releases"
  backup_dir="$root/backups"
}

compose() {
  # shellcheck disable=SC2086
  docker compose --project-name "$project" --env-file "$env_file" $compose_args "$@"
}

# Replace only the image assignment, preserving credentials and file permissions.
# A same-directory rename ensures readers never see a partially written .env.
persist_image() (
  pinned_image="$1"
  valid_digest "$pinned_image" || exit 1
  [ -f "$env_file" ] && [ ! -L "$env_file" ] || exit 1
  umask 077
  image_tmp="$(mktemp "$root/.env.image.XXXXXX")" || exit 1
  trap 'rm -f "$image_tmp"' EXIT
  trap 'exit 1' HUP INT TERM
  cp -p "$env_file" "$image_tmp" || exit 1
  awk -v image="$pinned_image" '
    /^[[:space:]]*(export[[:space:]]+)?SUB2API_IMAGE[[:space:]]*=/ {
      if (!written++) print "SUB2API_IMAGE=" image
      next
    }
    { print }
    END { if (!written) print "SUB2API_IMAGE=" image }
  ' "$env_file" > "$image_tmp" || exit 1
  mv "$image_tmp" "$env_file" || exit 1
)

verify_image_configuration() (
  expected_image="$1"
  [ "$(current_image)" = "$expected_image" ] || exit 1
  # Shell variables override --env-file; explicitly exclude that false positive.
  unset SUB2API_IMAGE
  configured_images="$(compose config --images sub2api)" || exit 1
  printf '%s\n' "$configured_images" | grep -Fx "$expected_image" >/dev/null
)

sync_image() {
  configure_environment "$1"
  running_image="$(current_image)"
  valid_digest "$running_image" || { echo "running image is not a pinned deployment digest" >&2; return 1; }
  healthcheck || return 1
  persist_image "$running_image" || return 1
  verify_image_configuration "$running_image" || return 1
  printf 'image_config=ok\n'
}

current_image() {
  docker inspect "$container" --format '{{.Config.Image}}' 2>/dev/null || true
}

current_digest() {
  docker inspect "$container" --format '{{range .RepoDigests}}{{println .}}{{end}}' 2>/dev/null | head -n 1 || true
}

healthcheck() {
  attempt=1
  while [ "$attempt" -le 30 ]; do
    health="$(docker inspect "$container" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}running{{end}}' 2>/dev/null || true)"
    if [ "$health" = "healthy" ] || [ "$health" = "running" ]; then
      if curl --fail --silent --show-error --max-time 10 "http://127.0.0.1:$port/health" >/dev/null && \
         curl --fail --silent --show-error --max-time 10 "http://127.0.0.1:$port/" >/dev/null; then
        return 0
      fi
    fi
    sleep 2
    attempt=$((attempt + 1))
  done
  return 1
}

backup_postgres() {
  database_container="$1"
  backup_file=""
  if ! docker inspect "$database_container" >/dev/null 2>&1; then
    return 0
  fi
  mkdir -p "$backup_dir"
  backup_file="$backup_dir/postgres-$(date -u +%Y%m%dT%H%M%SZ).dump"
  docker exec "$database_container" sh -c 'PGPASSWORD="$POSTGRES_PASSWORD" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_file"
  chmod 600 "$backup_file"
}

verify_staging() {
  target="$1"
  valid_digest "$target" || { echo "invalid staging digest" >&2; exit 2; }
  configure_environment staging
  [ -f "$env_file" ] || { echo "staging .env is missing" >&2; exit 1; }
  actual="$(current_image)"
  [ "$actual" = "$target" ] || { echo "staging image mismatch: $actual" >&2; exit 1; }
  verify_image_configuration "$target" || { echo "staging persistent image configuration mismatch" >&2; exit 1; }
  healthcheck
}

status() {
  configure_environment "$1"
  printf 'environment=%s\n' "$1"
  printf 'path=%s\n' "$root"
  printf 'image=%s\n' "$(current_image)"
  healthcheck && printf 'health=ok\n' || printf 'health=failed\n'
}

deploy() {
  environment="$1"
  target="$2"
  valid_digest "$target" || { echo "invalid deployment digest" >&2; exit 2; }
  configure_environment "$environment"
  [ -f "$env_file" ] || { echo "environment .env is missing: $env_file" >&2; exit 1; }
  mkdir -p "$release_dir" "$backup_dir"

  previous="$(current_image)"
  previous_digest="$(current_digest)"
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  release_file="$release_dir/deploy-$timestamp.env"
  database_container="sub2api-postgres"
  [ "$environment" = "staging" ] && database_container="sub2api-staging-postgres"

  export SUB2API_IMAGE="$target"
  compose config --quiet
  backup_postgres "$database_container"
  docker pull "$target"

  cat > "$release_file" <<EOF
environment=$environment
previous_image=$previous
previous_digest=$previous_digest
target_image=$target
backup_file=$backup_file
started_at=$timestamp
EOF
  chmod 600 "$release_file"

  rollback() {
    if [ -n "$previous" ]; then
      export SUB2API_IMAGE="$previous"
      compose up -d --no-deps --force-recreate sub2api >/dev/null 2>&1 || return 1
      healthcheck || return 1
      persist_image "$previous" || return 1
      verify_image_configuration "$previous" || return 1
    fi
  }

  if ! compose up -d --no-deps --force-recreate sub2api; then
    rollback || echo "rollback failed; verify the running image and persistent configuration" >&2
    exit 1
  fi
  if ! healthcheck; then
    rollback || echo "rollback failed; verify the running image and persistent configuration" >&2
    exit 1
  fi

  if ! persist_image "$target" || ! verify_image_configuration "$target"; then
    echo "deployment image persistence failed" >&2
    rollback || echo "rollback failed; verify the running image and persistent configuration" >&2
    exit 1
  fi

  printf 'completed_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$release_file"
  ls -1t "$release_dir"/deploy-*.env 2>/dev/null | tail -n +4 | xargs -r rm -f
}

[ "$#" -ge 2 ] || usage
case "$1" in
  deploy) [ "$#" -eq 3 ] || usage; deploy "$2" "$3" ;;
  verify-staging) [ "$#" -eq 2 ] || usage; verify_staging "$2" ;;
  status) [ "$#" -eq 2 ] || usage; status "$2" ;;
  sync-image) [ "$#" -eq 2 ] || usage; sync_image "$2" ;;
  *) usage ;;
esac
