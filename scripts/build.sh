#!/usr/bin/env bash
# Cross-compiles freshbooks-mcp and the freshbooks CLI for
# {linux,darwin,windows} x {amd64,arm64} into dist/ (gitignored).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$repo_root/dist"
mkdir -p "$dist"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

binaries=(
  "mcp freshbooks-mcp ./cmd/freshbooks-mcp"
  "cli freshbooks ./cmd/freshbooks"
)

for binary in "${binaries[@]}"; do
  read -r module name pkg <<<"$binary"
  version=$(cd "$repo_root/$module" && git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

  for target in "${targets[@]}"; do
    read -r goos goarch <<<"$target"
    ext=""
    if [ "$goos" = "windows" ]; then
      ext=".exe"
    fi
    out="$dist/${name}_${goos}_${goarch}${ext}"
    echo "build: $module $pkg -> $out"
    (
      cd "$repo_root/$module"
      GOOS="$goos" GOARCH="$goarch" go build -ldflags "-s -w -X main.version=${version}" -o "$out" "$pkg"
    )
  done
done

echo "build: done, artifacts in $dist"
