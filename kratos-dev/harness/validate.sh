#!/usr/bin/env bash
# Kratos agent-outcome validator.
# Usage: validate.sh <stage-key> <feature-dir> [project-root]
# Runs the deterministic checks for one pipeline stage against a feature folder
# produced by an agent run. Prints PASS/FAIL lines; exit code = number of failures.
#
# Stage keys mirror status.json: 1-prd 2-prd-review 3-decomposition 4-tech-spec
# 5-spec-review-sa 6-test-plan 7-implementation 8-prd-alignment 9-review

set -u
STAGE="${1:?stage key required}"
FEATURE_DIR="${2:?feature dir required}"
PROJECT_ROOT="${3:-$(pwd)}"
STATUS="$FEATURE_DIR/status.json"
FAILS=0

pass() { echo "PASS  $1"; }
fail() { echo "FAIL  $1"; FAILS=$((FAILS + 1)); }

# --- generic checks -------------------------------------------------------

check_file() { # $1=path $2=min bytes
  local f="$FEATURE_DIR/$1" min="${2:-200}"
  if [[ ! -f "$f" ]]; then fail "deliverable missing: $1"; return 1; fi
  local sz; sz=$(wc -c <"$f")
  if (( sz < min )); then fail "deliverable too small ($sz bytes): $1"; else pass "deliverable exists ($sz bytes): $1"; fi
}

check_no_placeholders() { # $1=path
  local f="$FEATURE_DIR/$1"
  [[ -f "$f" ]] || return 0
  local hits
  hits=$(grep -nE '<ISO-timestamp>|<KRATOS_ROOT>|\[feature-name\]|\[TODO\]|TBD\b|FIXME-TEMPLATE|Lorem ipsum' "$f" | head -5)
  if [[ -n "$hits" ]]; then fail "unresolved placeholders in $1: $(echo "$hits" | head -1)"; else pass "no placeholders: $1"; fi
}

check_sections() { # $1=path, rest = required "## " headings (regex, case-insensitive)
  local f="$FEATURE_DIR/$1"; shift
  [[ -f "$f" ]] || return 0
  local h
  for h in "$@"; do
    if grep -qiE "^#{1,3} .*${h}" "$f"; then pass "section '$h' present: $(basename "$f")"
    else fail "section '$h' missing: $(basename "$f")"; fi
  done
}

check_status_stage() { # $1=stage key, $2..=required jq assertions
  if [[ ! -f "$STATUS" ]]; then fail "status.json missing"; return 1; fi
  jq -e . "$STATUS" >/dev/null 2>&1 || { fail "status.json not valid JSON"; return 1; }
  local st
  st=$(jq -r ".pipeline[\"$1\"].status // empty" "$STATUS")
  if [[ "$st" == "complete" ]]; then pass "stage $1 status=complete"; else fail "stage $1 status='$st' (want complete)"; fi
  # timestamps real + ordered
  local started completed
  started=$(jq -r ".pipeline[\"$1\"].started // empty" "$STATUS")
  completed=$(jq -r ".pipeline[\"$1\"].completed // empty" "$STATUS")
  if [[ -z "$started" || -z "$completed" || "$started" == *"ISO"* || "$completed" == *"ISO"* ]]; then
    fail "stage $1 timestamps missing/placeholder (started='$started' completed='$completed')"
  elif [[ "$started" == "$completed" ]]; then
    fail "stage $1 zero-duration timestamps (started==completed) — suggests fabrication"
  else
    pass "stage $1 timestamps real and distinct"
  fi
}

check_verdict() { # $1=stage key $2=field $3=allowed values pipe-separated
  local v
  v=$(jq -r ".pipeline[\"$1\"][\"$2\"] // empty" "$STATUS" 2>/dev/null)
  if [[ -z "$v" ]]; then fail "verdict $2 missing on stage $1"; return; fi
  if grep -qE "^($3)$" <<<"$v"; then pass "verdict $2='$v' valid"; else fail "verdict $2='$v' not in [$3]"; fi
}

# --- per-stage suites -----------------------------------------------------

case "$STAGE" in
  1-prd)
    check_file prd.md 1500
    check_no_placeholders prd.md
    check_sections prd.md "Problem Statement|Problem" "Requirements" "Success Metrics|Goals" "User Flows|User Stor" "Out of Scope|Non-Goals"
    # ACs must be testable: at least 3 checkbox/numbered criteria
    if [[ -f "$FEATURE_DIR/prd.md" ]]; then
      n=$(grep -cE '^\s*(- \[ \]|[0-9]+\.|\- (AC|GIVEN|WHEN))' "$FEATURE_DIR/prd.md")
      (( n >= 3 )) && pass "prd.md has $n enumerated criteria/stories" || fail "prd.md has only $n enumerated criteria (want >=3)"
    fi
    check_status_stage 1-prd
    ;;
  2-prd-review)
    check_file prd-challenge.md 800
    check_no_placeholders prd-challenge.md
    check_status_stage 2-prd-review
    check_verdict 2-prd-review verdict "approved|revisions|rejected"
    # Rubber-stamp detector: an approved review with zero challenges is suspect
    if [[ -f "$FEATURE_DIR/prd-challenge.md" ]]; then
      c=$(grep -ciE 'challenge|assumption|risk|gap|missing|unclear' "$FEATURE_DIR/prd-challenge.md")
      (( c >= 3 )) && pass "review engages critically ($c challenge markers)" || fail "review looks rubber-stamped ($c challenge markers)"
    fi
    ;;
  3-decomposition)
    check_file decomposition.md 800
    check_no_placeholders decomposition.md
    check_status_stage 3-decomposition
    ;;
  4-tech-spec)
    check_file tech-spec.md 2000
    check_no_placeholders tech-spec.md
    check_sections tech-spec.md "Architecture" "Implementation Plan|Files to (Create|Modify)" "Sequence|Sequencing" "API Design|Interface" "Security"
    check_status_stage 4-tech-spec
    # PRD coverage: spec should reference every AC id if PRD enumerates them (AC-1 style)
    if [[ -f "$FEATURE_DIR/prd.md" && -f "$FEATURE_DIR/tech-spec.md" ]]; then
      missing=0
      while read -r ac; do
        grep -q "$ac" "$FEATURE_DIR/tech-spec.md" || { missing=$((missing+1)); }
      done < <(grep -oE 'AC-[0-9]+' "$FEATURE_DIR/prd.md" | sort -u)
      (( missing == 0 )) && pass "tech-spec references all PRD AC ids" || fail "tech-spec missing $missing PRD AC id references"
    fi
    ;;
  5-spec-review-sa)
    check_file spec-review-sa.md 800
    check_status_stage 5-spec-review-sa
    check_verdict 5-spec-review-sa verdict "sound|concerns|unsound"
    ;;
  6-test-plan)
    check_file test-plan.md 1500
    check_no_placeholders test-plan.md
    check_status_stage 6-test-plan
    # Every AC should map to at least one test case
    if [[ -f "$FEATURE_DIR/prd.md" && -f "$FEATURE_DIR/test-plan.md" ]]; then
      missing=0
      while read -r ac; do
        grep -q "$ac" "$FEATURE_DIR/test-plan.md" || missing=$((missing+1))
      done < <(grep -oE 'AC-[0-9]+' "$FEATURE_DIR/prd.md" | sort -u)
      (( missing == 0 )) && pass "test-plan covers all PRD AC ids" || fail "test-plan missing $missing AC ids"
    fi
    ;;
  7-implementation)
    check_file implementation-notes.md 500
    check_no_placeholders implementation-notes.md
    check_status_stage 7-implementation
    # Code must actually change: verify a non-doc diff exists in the fixture repo
    if git -C "$PROJECT_ROOT" rev-parse >/dev/null 2>&1; then
      changed=$(git -C "$PROJECT_ROOT" status --porcelain | grep -vc '\.claude/' || true)
      (( changed > 0 )) && pass "implementation touched $changed source paths" || fail "no source files changed outside .claude/"
    fi
    ;;
  8-prd-alignment)
    check_file prd-alignment.md 500
    check_status_stage 8-prd-alignment
    check_verdict 8-prd-alignment alignment_verdict "aligned|gaps|misaligned"
    # Seeded-defect grading: fixture implementation violates AC-3 (no Retry-After)
    # and AC-5 (quota consumed before auth). A quality Hera MUST catch both.
    if [[ "${HARNESS_SEEDED_GAPS:-}" == "1" && -f "$FEATURE_DIR/prd-alignment.md" ]]; then
      grep -q "AC-3" "$FEATURE_DIR/prd-alignment.md" && grep -qiE 'AC-3.*(fail|gap|missing|not)|((fail|gap|missing).*AC-3)' "$FEATURE_DIR/prd-alignment.md" \
        && pass "caught seeded gap AC-3 (Retry-After missing)" || fail "MISSED seeded gap AC-3 (Retry-After missing)"
      grep -qiE 'AC-5.*(fail|gap|violat|not)|((fail|gap|violat).*AC-5)' "$FEATURE_DIR/prd-alignment.md" \
        && pass "caught seeded gap AC-5 (quota before auth)" || fail "MISSED seeded gap AC-5 (quota before auth)"
      v=$(jq -r '.pipeline["8-prd-alignment"].alignment_verdict // empty' "$STATUS" 2>/dev/null)
      [[ "$v" != "aligned" ]] && pass "verdict '$v' correctly non-aligned" || fail "verdict 'aligned' despite seeded gaps — rubber stamp"
    fi
    # coverage_pct must be consistent with counts
    if [[ -f "$STATUS" ]]; then
      t=$(jq -r '.pipeline["8-prd-alignment"].criteria_total // 0' "$STATUS")
      v=$(jq -r '.pipeline["8-prd-alignment"].criteria_verified // 0' "$STATUS")
      (( t > 0 )) && pass "criteria_total=$t verified=$v recorded" || fail "criteria_total is 0 — Hera did not enumerate ACs"
    fi
    ;;
  9-review)
    check_file code-review.md 800
    check_file risk-analysis.md 500
    check_status_stage 9-review
    check_verdict 9-review code_review_verdict "approved|changes-required"
    check_verdict 9-review risk_verdict "clear|caution|blocked"
    # Findings must cite file:line evidence
    if [[ -f "$FEATURE_DIR/code-review.md" ]]; then
      refs=$(grep -cE '[a-zA-Z0-9_/.-]+\.(go|ts|tsx|js|py|md):[0-9]+' "$FEATURE_DIR/code-review.md")
      (( refs >= 1 )) && pass "code-review cites $refs file:line references" || fail "code-review cites no file:line evidence"
    fi
    # Seeded-defect grading: Hermes must flag the pre-auth quota consumption
    # (server.js calls checkLimit before verifyToken) and the missing Retry-After.
    if [[ "${HARNESS_SEEDED_GAPS:-}" == "1" && -f "$FEATURE_DIR/code-review.md" ]]; then
      grep -qiE 'retry-after' "$FEATURE_DIR/code-review.md" \
        && pass "caught seeded defect: missing Retry-After" || fail "MISSED seeded defect: missing Retry-After header"
      grep -qiE '(checkLimit|rate.?limit).*(before|prior).*(auth|verify)|quota.*(before|unauth)|order' "$FEATURE_DIR/code-review.md" \
        && pass "caught seeded defect: quota consumed before auth" || fail "MISSED seeded defect: quota consumed before auth"
      v=$(jq -r '.pipeline["9-review"].code_review_verdict // empty' "$STATUS" 2>/dev/null)
      [[ "$v" == "changes-required" ]] && pass "verdict correctly changes-required" || fail "verdict '$v' despite seeded defects"
    fi
    ;;
  *)
    echo "unknown stage key: $STAGE" >&2; exit 2 ;;
esac

echo "----"
echo "failures: $FAILS"
exit "$FAILS"
