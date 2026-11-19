#!/usr/bin/env bash
# Generates every template for every target it supports and checks the result
# with the real tools: go vet/test, gofmt, terraform validate, docker compose
# config, pytest and mkdocs. Slow (downloads providers and modules); the fast
# render-only check is `go test ./...`.
#
# Usage: scripts/verify.sh [template ...]     (default: all templates)
set -uo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
bin="$work/kickhub"
fail=0
export TF_PLUGIN_CACHE_DIR="${TF_PLUGIN_CACHE_DIR:-$work/tf-cache}"
mkdir -p "$TF_PLUGIN_CACHE_DIR"

(cd "$root" && go build -o "$bin" .) || exit 1

check() { # check <label> <command...>
  local label="$1"; shift
  if out="$("$@" 2>&1)"; then
    echo "  ok    $label"
  else
    echo "  FAIL  $label"; echo "$out" | tail -15 | sed 's/^/        /'; fail=1
  fi
}

have() { command -v "$1" >/dev/null 2>&1; }

templates=("$@")
if [ ${#templates[@]} -eq 0 ]; then
  mapfile -t templates < <("$bin" list | awk '{print $1}')
fi

for t in "${templates[@]}"; do
  targets="$("$bin" list | awk -v t="$t" '$1==t' | sed 's/.*\[\(.*\)\]/\1/; s/,//g')"
  for target in $targets; do
    dir="$work/$t-$target"
    echo "$t / $target"
    if ! "$bin" new "$t" -name "demo-$target" -target "$target" -out "$dir" -set gcp_project=demo-project >/dev/null 2>&1 &&
       ! "$bin" new "$t" -name "demo-$target" -target "$target" -out "$dir" >/dev/null 2>&1; then
      echo "  FAIL  generate"; fail=1; continue
    fi
    cd "$dir" || exit 1

    check "status is clean" bash -c "'$bin' status . | grep -q ' 0 modified, 0 missing'"

    if [ -f go.mod ]; then
      check "go mod tidy" go mod tidy
      check "go vet" go vet ./...
      check "go test" go test ./...
      check "gofmt" bash -c 'test -z "$(gofmt -l .)"'
      if [ -d cmd/lambda ]; then
        check "lambda build" env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o /dev/null ./cmd/lambda
      fi
    fi
    if [ -f requirements-dev.txt ] && have python3; then
      check "pytest" bash -c 'python3 -m venv .venv && .venv/bin/pip install -q -r requirements-dev.txt && .venv/bin/pytest -q'
    fi
    if [ -f mkdocs.yml ] && have python3; then
      check "mkdocs build --strict" bash -c 'python3 -m venv .venv && .venv/bin/pip install -q -r requirements.txt && .venv/bin/mkdocs build -q'
    fi
    if [ -d infra ] && have terraform; then
      check "terraform fmt" terraform -chdir=infra fmt -check
      check "terraform validate" bash -c 'terraform -chdir=infra init -backend=false -input=false >/dev/null && terraform -chdir=infra validate'
    fi
    if [ -f docker-compose.yml ] && have docker; then
      check "compose config" docker compose config -q
    fi
  done
done

rm -rf "$work"
[ "$fail" -eq 0 ] && echo "ALL CHECKS PASSED" || { echo "SOME CHECKS FAILED"; exit 1; }
