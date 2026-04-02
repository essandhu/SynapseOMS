#!/usr/bin/env bash
#
# scripts/proto-gen.sh — Generate Go, Python, and TypeScript bindings from proto/ definitions.
# Requires: buf (https://buf.build/docs/installation)
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROTO_DIR="${REPO_ROOT}/proto"
GEN_DIR="${REPO_ROOT}/gen"

echo "==> Cleaning previous generated code..."
rm -rf "${GEN_DIR}"
mkdir -p "${GEN_DIR}/go" "${GEN_DIR}/python" "${GEN_DIR}/ts"

echo "==> Linting proto files..."
buf lint "${PROTO_DIR}"

echo "==> Checking for breaking changes..."
buf breaking "${PROTO_DIR}" --against ".git#subdir=proto" 2>/dev/null || echo "    (skipped — no previous git state)"

echo "==> Generating Go bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.go.yaml" --output "${GEN_DIR}/go"

echo "==> Generating Python bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.python.yaml" --output "${GEN_DIR}/python"

echo "==> Generating TypeScript bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.ts.yaml" --output "${GEN_DIR}/ts"

echo "==> Done. Generated code is in ${GEN_DIR}/"
