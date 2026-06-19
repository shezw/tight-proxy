#!/usr/bin/env bash
set -euo pipefail

part="${1:-patch}"
version="$(tr -d '[:space:]' < VERSION)"
if [[ ! "${version}" =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
  echo "VERSION must use MAJOR.MINOR.PATCH, got ${version}" >&2
  exit 1
fi

major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "${part}" in
  major)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  minor)
    minor=$((minor + 1))
    patch=0
    ;;
  patch)
    patch=$((patch + 1))
    ;;
  *)
    echo "Usage: $0 [major|minor|patch]" >&2
    exit 1
    ;;
esac

next="${major}.${minor}.${patch}"
printf '%s\n' "${next}" > VERSION
echo "${next}"
