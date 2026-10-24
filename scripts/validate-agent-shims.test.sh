#!/usr/bin/env sh
# validate-agent-shims.test.sh — keep tool-specific instruction shims canonical.
#
# AGENTS.md is the durable, cross-tool source of truth. Include-capable agents
# receive it through shims that hold only a Markdown heading (which the
# repository's Markdown lint requires as the first line) and one @AGENTS.md
# include. Copied prose would create a second source that can silently drift.
# Run locally with:
#
#   sh scripts/validate-agent-shims.test.sh
#
# CI runs the same assertion in the core job of .github/workflows/ci.yaml.
set -eu

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
expected="$(mktemp)"
trap 'rm -f "$expected"' EXIT

printf '# Agent instructions\n\n@AGENTS.md\n' >"$expected"

failed=0

report_failure() {
	echo "FAIL: $*" >&2
	failed=1
}

for shim in CLAUDE.md GEMINI.md; do
	path="$repo_root/$shim"
	if [ ! -f "$path" ]; then
		report_failure "$shim is missing; it must include the canonical AGENTS.md"
		continue
	fi
	if ! cmp -s "$expected" "$path"; then
		report_failure "$shim must contain exactly the heading '# Agent instructions', one blank line and @AGENTS.md, ending in one newline"
	fi
done

if [ "$failed" -ne 0 ]; then
	exit 1
fi

echo "PASS: CLAUDE.md and GEMINI.md are canonical AGENTS.md shims"
