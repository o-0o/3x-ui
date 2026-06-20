#!/usr/bin/env bash
set -euo pipefail

XRAY_TAG="${XRAY_TAG:-v26.6.1}"
REPO_URL="${XRAY_REPO_URL:-https://github.com/XTLS/Xray-core.git}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WORK_DIR="${WORK_DIR:-/tmp/xray-core-tokenbucket}"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/dist}"
PATCH_FILE="$ROOT_DIR/patches/xray-core-v26.6.1-tokenbucket-speedlimit.patch"
XRAY_BINARY="$OUT_DIR/xray"

rm -rf "$WORK_DIR"
mkdir -p "$OUT_DIR"

git clone --depth 1 --branch "$XRAY_TAG" "$REPO_URL" "$WORK_DIR"
cd "$WORK_DIR"
git apply "$PATCH_FILE"
if command -v protoc >/dev/null 2>&1; then
  GOBIN="${GOBIN:-/tmp/xray-protoc-tools}"
  mkdir -p "$GOBIN"
  GOBIN="$GOBIN" go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11
  PATH="$GOBIN:$PATH" protoc --go_out=. --go_opt=paths=source_relative common/protocol/user.proto
else
  echo "protoc is required because this patch changes common/protocol/user.proto" >&2
  exit 1
fi

GOCACHE="${GOCACHE:-/tmp/go-build-xray}" \
GOMODCACHE="${GOMODCACHE:-/tmp/go-mod-xray}" \
GOTOOLCHAIN="${GOTOOLCHAIN:-go1.26.4}" \
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
go build -trimpath -ldflags "-s -w -buildid=" -o "$XRAY_BINARY" ./main

zip -q -j "$OUT_DIR/Xray-linux-64.zip" "$XRAY_BINARY"
echo "$OUT_DIR/Xray-linux-64.zip"
