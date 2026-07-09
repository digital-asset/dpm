#!/usr/bin/env bash
# =============================================================================
# smoke-test.sh — End-to-end smoke test for rst-to-mdx.
#
# Self-contained: runs against testdata/fixtures/smoke.rst and compares the
# converted output byte-for-byte against testdata/fixtures/smoke.expected.mdx.
# No external corpus dependency — docs-website is being retired and is not
# guaranteed to exist on contributor machines or in CI.
#
# When you change a transform on purpose, regenerate the golden:
#     ./rst-to-mdx testdata/fixtures/smoke.rst \
#         testdata/fixtures/smoke.expected.mdx
# and commit both the code and the new golden in the same change.
#
# Usage:
#   ./smoke-test.sh           # run all cases
#   ./smoke-test.sh --keep    # keep the temp output for inspection
# =============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()    { echo -e "${YELLOW}[INFO]${NC}    $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}      $1"; }
log_error()   { echo -e "${RED}[FAIL]${NC}    $1" >&2; }

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

BIN="$SCRIPT_DIR/rst-to-mdx"
FIXTURE="testdata/fixtures/smoke.rst"
GOLDEN="testdata/fixtures/smoke.expected.mdx"

KEEP=false
[[ "${1:-}" == "--keep" ]] && KEEP=true

if [[ ! -x "$BIN" ]]; then
    log_info "binary not found at $BIN — building…"
    go build -o rst-to-mdx ./cmd/rst-to-mdx
fi

for required in "$FIXTURE" "$GOLDEN"; do
    if [[ ! -f "$required" ]]; then
        log_error "missing required file: $required"
        exit 1
    fi
done

TMPDIR="$(mktemp -d)"
trap '[[ "$KEEP" == true ]] && echo "kept: $TMPDIR" || rm -rf "$TMPDIR"' EXIT

# ---------------------------------------------------------------------------
# Case 1: --version prints without error
# ---------------------------------------------------------------------------
log_info "case 1: --version"
VERSION_OUT="$("$BIN" --version 2>&1)"
if [[ "$VERSION_OUT" != rst-to-mdx* ]]; then
    log_error "expected 'rst-to-mdx <version>', got: $VERSION_OUT"
    exit 1
fi
log_success "version: $VERSION_OUT"

# ---------------------------------------------------------------------------
# Case 2: structural invariants on a real conversion
# ---------------------------------------------------------------------------
log_info "case 2: structural invariants on $FIXTURE"
OUT="$TMPDIR/out.mdx"
"$BIN" "$FIXTURE" "$OUT"

[[ -s "$OUT" ]]                                || { log_error "output empty"; exit 1; }
head -n 1 "$OUT" | grep -q "^---$"             || { log_error "missing opening '---' frontmatter fence"; exit 1; }
grep -q "COPIED_START" "$OUT"                  || { log_error "missing COPIED_START marker"; exit 1; }
grep -q "COPIED_END"   "$OUT"                  || { log_error "missing COPIED_END marker"; exit 1; }
tail -c 1 "$OUT" | xxd | grep -q "0a"          || { log_error "missing trailing newline"; exit 1; }

log_success "structural invariants pass"

# ---------------------------------------------------------------------------
# Case 3: byte-for-byte diff against the frozen golden
#
# If this fails after an intentional transform change, regenerate the
# golden (see header comment) and commit both files together.
# ---------------------------------------------------------------------------
log_info "case 3: diff against $GOLDEN"
if ! diff -u "$GOLDEN" "$OUT"; then
    log_error "converter output drifted from golden"
    log_error "if the change is intentional, regenerate:"
    log_error "  ./rst-to-mdx $FIXTURE $GOLDEN"
    exit 1
fi
log_success "golden match"

# ---------------------------------------------------------------------------
# Case 4: --dry-run does not create a file
# ---------------------------------------------------------------------------
log_info "case 4: --dry-run leaves disk untouched"
DRY_OUT="$TMPDIR/dry-run.mdx"
"$BIN" "$FIXTURE" "$DRY_OUT" --dry-run > /dev/null
if [[ -e "$DRY_OUT" ]]; then
    log_error "--dry-run wrote a file"
    exit 1
fi
log_success "--dry-run passes"

# ---------------------------------------------------------------------------
# Case 5: empty input is rejected
# ---------------------------------------------------------------------------
log_info "case 5: empty input rejected"
EMPTY="$TMPDIR/empty.rst"
: > "$EMPTY"
if "$BIN" "$EMPTY" "$TMPDIR/empty.mdx" 2>/dev/null; then
    log_error "empty input should fail"
    exit 1
fi
log_success "empty input rejected as expected"

# Note: a case asserting `--strict` fails on the fixture's unresolved
# `:ref:` was considered, but `Options.Strict` is not currently wired
# through `convert/links.go` (tracked as a review-finding follow-up in
# the Notion ticket). Re-add once that path is hooked.

echo
log_success "smoke test passed"
