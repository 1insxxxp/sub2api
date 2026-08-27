#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml \
  deploy/docker-compose.dev.yml
do
  for expected in \
    '      - DUJIAO_LOGIN_ENABLED=${DUJIAO_LOGIN_ENABLED:-false}' \
    '      - DUJIAO_LOGIN_SHARED_SECRET=${DUJIAO_LOGIN_SHARED_SECRET:-}'
  do
    count=$(grep -Fxc "$expected" "$compose_file" || true)
    if [ "$count" -ne 1 ]; then
      printf '%s must pass %s exactly once\n' "$compose_file" "$expected" >&2
      exit 1
    fi
  done
done

for key in DUJIAO_LOGIN_ENABLED DUJIAO_LOGIN_SHARED_SECRET; do
  count=$(grep -Ec "^${key}=" deploy/.env.example || true)
  if [ "$count" -ne 1 ]; then
    printf 'deploy/.env.example must declare %s exactly once\n' "$key" >&2
    exit 1
  fi
done

printf 'docker compose Dujiao login environment test passed\n'
