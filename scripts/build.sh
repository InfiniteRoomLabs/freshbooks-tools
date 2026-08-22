#!/usr/bin/env -S usage bash
#USAGE arg "[modules]" var=#true help="Modules to build binaries for (default: mcp cli)"

# Cross-compiles freshbooks-mcp and/or the freshbooks CLI for
# {linux,darwin,windows} x {amd64,arm64} into dist/ (gitignored).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$repo_root/dist"
mkdir -p "$dist"

if [ -z "${usage_modules:-}" ]; then
  want=(mcp cli)
else
  read -ra want <<<"$usage_modules"
fi

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

version=$(git -C "$repo_root" describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")

built_any=false
for binary in "${binaries[@]}"; do
  read -r module name pkg <<<"$binary"

  wanted=false
  for w in "${want[@]}"; do
    [ "$w" = "$module" ] && wanted=true
  done
  if [ "$wanted" = false ]; then
    continue
  fi
  built_any=true

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

if [ "$built_any" = false ]; then
  echo "build: no buildable modules in the requested set (${want[*]}) -- nothing to do"
  exit 0
fi

echo "build: done, artifacts in $dist"
