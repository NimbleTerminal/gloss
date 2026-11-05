#!/bin/sh
# Go's -exec wrapper: install the trusted Linux stage beside a test executable.
set -eu
stage=$1
shift
cp "$stage" "$(dirname "$1")/ntconfine-stage"
exec "$@"
