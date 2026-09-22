#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$repo_root"

fail() {
  printf 'appcatalogd release test failed: %s\n' "$1" >&2
  exit 1
}

assert_file() {
  test -f "$1" || fail "missing file: $1"
}

assert_contains() {
  file=$1
  needle=$2
  grep -Fq "$needle" "$file" || fail "$file is missing: $needle"
}

assert_count() {
  file=$1
  needle=$2
  expected=$3
  actual=$(grep -Fxc "$needle" "$file" || true)
  [ "$actual" -eq "$expected" ] || fail "$file has $actual occurrences of '$needle', expected $expected"
}

assert_absent() {
  file=$1
  needle=$2
  if grep -Fq "$needle" "$file"; then
    fail "$file still contains: $needle"
  fi
}

assert_file backend/cmd/appcatalogd/main.go
assert_file deploy/sub2api-appcatalogd.service
assert_file deploy/APPCATALOGD_CN.md
assert_file deploy/appcatalogd-entrypoint.sh
assert_file Dockerfile.appcatalogd
assert_file .github/workflows/release-appcatalogd.yml

assert_contains .goreleaser.yaml 'id: appcatalogd'
assert_contains .goreleaser.yaml 'main: ./cmd/appcatalogd'
assert_contains .github/workflows/release-appcatalogd.yml 'appcatalogd-v*'
assert_contains .github/workflows/release-appcatalogd.yml 'Dockerfile.appcatalogd'
assert_contains .github/workflows/release-appcatalogd.yml 'sub2api-appcatalogd:'
assert_contains Dockerfile.appcatalogd 'COPY --from=backend-builder --chown=sub2api:sub2api /out/appcatalogd /app/appcatalogd'
assert_count .goreleaser.yaml '      - appcatalogd' 0
assert_absent .goreleaser.simple.yaml 'id: appcatalogd'
assert_absent Dockerfile.goreleaser 'COPY appcatalogd /app/appcatalogd'
assert_absent Dockerfile 'COPY --from=backend-builder --chown=sub2api:sub2api /app/appcatalogd /app/appcatalogd'
assert_absent deploy/Dockerfile 'COPY --from=backend-builder /app/appcatalogd /app/appcatalogd'
assert_contains deploy/install.sh 'INSTALL_DIR/appcatalogd'
assert_contains deploy/install.sh 'install_appcatalogd_service'
assert_contains deploy/install.sh 'sub2api-appcatalogd.service'
assert_contains deploy/sub2api-appcatalogd.service 'ExecStart=/opt/sub2api/appcatalogd'
assert_contains deploy/sub2api-appcatalogd.service 'pricing-path /var/lib/sub2api/appcatalog/provider_pricing.json'
assert_contains deploy/install.sh 'provider_pricing.json'
assert_contains deploy/docker-compose.yml '${APP_CATALOG_PRICING_FILE:-./provider-pricing.json}:/app/config/provider_pricing.json:ro'
assert_contains deploy/sub2api.service 'sub2api-appcatalogd.service'

for compose_file in \
  deploy/docker-compose.yml \
  deploy/docker-compose.local.yml \
  deploy/docker-compose.standalone.yml
do
  assert_contains "$compose_file" '  appcatalogd:'
  assert_contains "$compose_file" 'image: ${APP_CATALOG_IMAGE:-weishaw/sub2api-appcatalogd:latest}'
  assert_contains "$compose_file" 'APP_CATALOG_BASE_URL=${APP_CATALOG_BASE_URL:-http://appcatalogd:18099}'
  assert_absent "$compose_file" '/app/appcatalogd'
done

assert_contains deploy/docker-compose.dev.yml 'dockerfile: Dockerfile.appcatalogd'
assert_contains deploy/docker-compose.dev.yml 'APP_CATALOG_BASE_URL=${APP_CATALOG_BASE_URL:-http://appcatalogd:18099}'
assert_absent deploy/docker-compose.dev.yml '/app/appcatalogd'

printf 'appcatalogd release test passed\n'
