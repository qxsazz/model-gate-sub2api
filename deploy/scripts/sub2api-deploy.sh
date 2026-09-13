#!/bin/sh
set -eu

PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
export PATH

usage() {
  echo "Usage: $0 deploy <staging|production> <ghcr-image@sha256:digest>" >&2
  echo "       $0 verify-staging <ghcr-image@sha256:digest>" >&2
  echo "       $0 status <staging|production>" >&2
  exit 2
}

valid_digest() {
  case "$1" in
    ghcr.io/qxsazz/model-gate-sub2api@sha256:????????????????????????????????????????????????????????????????) return 0 ;;
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
started_at=$timestamp
EOF
  chmod 600 "$release_file"

  rollback() {
    if [ -n "$previous" ]; then
      export SUB2API_IMAGE="$previous"
      compose up -d --no-deps --force-recreate sub2api >/dev/null 2>&1 || true
    fi
  }

  if ! compose up -d --no-deps --force-recreate sub2api; then
    rollback
    exit 1
  fi
  if ! healthcheck; then
    rollback
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
  *) usage ;;
esac
