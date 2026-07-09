#!/usr/bin/env bash
# Minimal smoke test for mdx-validate. Run from this directory after `make build`.
# Verifies the binary handles:
#   1. valid frontmatter passes (exit 0)
#   2. missing title fails (exit 1)
#   3. --version prints
#   4. snippets/ paths are skipped during dir walk

set -euo pipefail

# Run from this script's directory so BIN resolves regardless of CWD.
cd "$(dirname "$0")"
BIN="$(pwd)/mdx-validate"

# Each case uses its own scratch directory to avoid cross-contamination.
mktemp_subdir() { mktemp -d; }

# --- Case 1: valid file passes ---
TMP1=$(mktemp_subdir)
trap 'rm -rf "$TMP1"' EXIT
cat > "$TMP1/good.mdx" <<'EOF'
---
title: Smoke Page
---

body
EOF
"$BIN" "$TMP1/good.mdx" >/dev/null
echo "  ok: valid file passes"
rm -rf "$TMP1"
trap - EXIT

# --- Case 2: missing title fails ---
TMP2=$(mktemp_subdir)
trap 'rm -rf "$TMP2"' EXIT
cat > "$TMP2/bad.mdx" <<'EOF'
---
description: no title
---

body
EOF
if "$BIN" "$TMP2/bad.mdx" >/dev/null 2>&1; then
  echo "FAIL: missing-title file should have failed"
  exit 1
fi
echo "  ok: missing title fails as expected"
rm -rf "$TMP2"
trap - EXIT

# --- Case 3: --version prints ---
"$BIN" --version | grep -q "mdx-validate"
echo "  ok: --version prints"

# --- Case 4: snippet skip during dir walk ---
TMP4=$(mktemp_subdir)
trap 'rm -rf "$TMP4"' EXIT
mkdir -p "$TMP4/snippets"
echo "no frontmatter, but it's a snippet" > "$TMP4/snippets/partial.mdx"
cat > "$TMP4/page.mdx" <<'EOF'
---
title: Page
---

body
EOF
"$BIN" "$TMP4" >/dev/null
echo "  ok: snippets directory is skipped during dir walk"
rm -rf "$TMP4"
trap - EXIT

echo "smoke OK"
