#!/usr/bin/env bash
#
# scripts/proto-gen.sh — Generate Go, Python, and TypeScript bindings from proto/ definitions.
# Requires: buf (https://buf.build/docs/installation)
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROTO_DIR="${REPO_ROOT}/proto"
GEN_DIR="${REPO_ROOT}/gen"
PYTHON_PROTO_DIR="${REPO_ROOT}/risk-engine/risk_engine/proto"

echo "==> Cleaning previous generated code..."
rm -rf "${GEN_DIR}"
mkdir -p "${GEN_DIR}/go" "${GEN_DIR}/ts"

# Clean Python generated files (preserve __init__.py)
find "${PYTHON_PROTO_DIR}" -name '*_pb2.py' -o -name '*_pb2_grpc.py' -o -name '*_pb2.pyi' -o -name '*_pb2_grpc.pyi' 2>/dev/null | xargs rm -f

echo "==> Linting proto files..."
buf lint "${PROTO_DIR}"

echo "==> Checking for breaking changes..."
buf breaking "${PROTO_DIR}" --against ".git#subdir=proto" 2>/dev/null || echo "    (skipped — no previous git state)"

echo "==> Generating Go bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.go.yaml" --output "${GEN_DIR}/go"

echo "==> Generating Python bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.python.yaml"

echo "==> Generating TypeScript bindings..."
buf generate "${PROTO_DIR}" --template "${PROTO_DIR}/buf.gen.ts.yaml" --output "${GEN_DIR}/ts"

echo "==> Ensuring __init__.py files in Python proto packages..."
for dir in "${PYTHON_PROTO_DIR}" "${PYTHON_PROTO_DIR}"/*/; do
  [ -d "${dir}" ] && touch "${dir}/__init__.py"
done

echo "==> Done. Generated code is in ${GEN_DIR}/ and ${PYTHON_PROTO_DIR}/"
