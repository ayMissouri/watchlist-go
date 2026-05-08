#!/usr/bin/env sh
set -eu

REPO_ROOT="$(git rev-parse --show-toplevel)"

git config core.hooksPath "$REPO_ROOT/githooks"
chmod +x "$REPO_ROOT/githooks/pre-commit"

echo "Git hooks installed from $REPO_ROOT/githooks"