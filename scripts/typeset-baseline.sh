#!/usr/bin/env bash
# Export typeset PNGs across fonts and themes and record or check their hashes.
#   typeset-baseline.sh record   export to dist/typeset-baseline/ and write MANIFEST.sha256
#   typeset-baseline.sh check    export to dist/typeset-check/ and compare against the baseline
set -euo pipefail

mode=${1:-}
case "$mode" in
  record|check) ;;
  *) echo "usage: typeset-baseline.sh record|check" >&2; exit 2 ;;
esac

cd "$(dirname "$0")/.."

if command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$@"; }
elif command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$@"; }
else
  echo "typeset-baseline: need shasum or sha256sum" >&2
  exit 1
fi

if ! log=$(GOWORK=off task build 2>&1); then
  echo "$log" >&2
  exit 1
fi

fonts="readable editorial technical compact"
themes="light dark"
baseline=dist/typeset-baseline
if [ "$mode" = record ]; then out=$baseline; else out=dist/typeset-check; fi

rm -rf "$out"
mkdir -p "$out"

# export_set INPUT SUFFIX [FLAG...]: export every font and theme into
# $out/<file>-<font>-<theme>SUFFIX/. gloss is quiet unless it fails.
export_set() {
  local input=$1 suffix=$2 f t log
  shift 2
  for f in $fonts; do
    for t in $themes; do
      if ! log=$(./gloss --typeset --page all --dpi 150 --font "$f" --theme "$t" "$@" \
        --output-dir "$out/$(basename "$input")-$f-$t$suffix/" "$input" 2>&1); then
        echo "$log" >&2
        echo "typeset-baseline: export failed: $input $f $t $*" >&2
        exit 1
      fi
    done
  done
}

for input in examples/typeset.md examples/notes.txt; do
  export_set "$input" ""
  export_set "$input" -N --line-numbers
  export_set "$input" -S --chop-long-lines
done
export_set examples/batch.jsonl ""

# Hash every PNG, as paths relative to $out, sorted by path.
manifest() {
  (cd "$1" && find . -type f -name '*.png' | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r p; do sha256 "$p"; done)
}

if [ "$mode" = record ]; then
  manifest "$out" > "$baseline/MANIFEST.sha256"
  echo "recorded $(wc -l < "$baseline/MANIFEST.sha256" | tr -d ' ') hashes in $baseline/MANIFEST.sha256"
  exit 0
fi

if [ ! -f "$baseline/MANIFEST.sha256" ]; then
  echo "typeset-baseline: no baseline; run 'typeset-baseline.sh record' first" >&2
  exit 1
fi
manifest "$out" > "$out/MANIFEST.sha256"
if cmp -s "$baseline/MANIFEST.sha256" "$out/MANIFEST.sha256"; then
  echo "baseline unchanged"
else
  echo "typeset-baseline: differing, missing (-) or extra (+) paths:" >&2
  diff "$baseline/MANIFEST.sha256" "$out/MANIFEST.sha256" | sed -n 's/^[<>] [0-9a-f]*  //p' | sort -u >&2
  exit 1
fi
