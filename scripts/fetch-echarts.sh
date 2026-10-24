#!/bin/sh
# Vendors Apache ECharts into internal/chart/echarts, for the chart page gloss
# embeds. The npm package is fetched at a pinned version and checked against
# its published hash. Bump the two lines together:
#   npm view echarts@VERSION dist.integrity   (base64 of the sha512 below)
set -eu
ECHARTS_VERSION=6.1.0
ECHARTS_INTEGRITY='q0yaFPggC9FUdsWH4blavRWFmxdrIodbkoKNAjJudAI6CA9gNPxHtV2RcZNEepZVlk4yvBYkOkbk6HIVpIyHZA=='
want=$(printf '%s' "$ECHARTS_INTEGRITY" | base64 -d | od -An -v -tx1 | tr -d ' \n')
dir=internal/chart/echarts
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL -o "$tmp/package.tgz" "https://registry.npmjs.org/echarts/-/echarts-$ECHARTS_VERSION.tgz"
if command -v sha512sum >/dev/null 2>&1; then got=$(sha512sum "$tmp/package.tgz" | cut -d' ' -f1); else got=$(shasum -a 512 "$tmp/package.tgz" | cut -d' ' -f1); fi
if [ "$got" != "$want" ]; then echo "fetch-echarts: echarts $ECHARTS_VERSION does not match its pinned hash ($got)" >&2; exit 1; fi
tar -xzf "$tmp/package.tgz" -C "$tmp" --strip-components=1 package/dist/echarts.min.js package/LICENSE
mkdir -p "$dir"
cp "$tmp/dist/echarts.min.js" "$dir/echarts.min.js"
cp "$tmp/LICENSE" "$dir/LICENSE"
printf 'Apache ECharts %s: dist/echarts.min.js and LICENSE from the npm package, fetched and checked by scripts/fetch-echarts.sh. Do not edit; rerun the script to update.\n' "$ECHARTS_VERSION" > "$dir/README.md"
echo "fetch-echarts: echarts $ECHARTS_VERSION in $dir"
