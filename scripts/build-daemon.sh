#!/usr/bin/env bash
set -euo pipefail

# Cross-compile the dmaild binary for all desktop platforms.
# No CGO needed thanks to modernc.org/sqlite (pure Go).

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUTPUT_DIR="${PROJECT_ROOT}/frontend/resources/bin"

TARGETS=(
  "darwin arm64"
  "darwin amd64"
  "linux amd64"
  "windows amd64"
)

echo "Building dmaild for all platforms..."

for target in "${TARGETS[@]}"; do
  read -r goos goarch <<< "$target"
  ext=""
  if [ "$goos" = "windows" ]; then
    ext=".exe"
  fi

  # Map Go arch names to electron-builder platform names
  platform_arch="${goos}-${goarch}"
  if [ "$goarch" = "amd64" ]; then
    platform_arch="${goos}-x64"
  fi

  outdir="${OUTPUT_DIR}/${platform_arch}"
  mkdir -p "$outdir"
  outfile="${outdir}/dmaild${ext}"

  echo "  ${goos}/${goarch} -> ${outfile}"
  CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
    go build -o "$outfile" "${PROJECT_ROOT}/cmd/dmaild/"
done

echo "Done! Binaries in ${OUTPUT_DIR}"
