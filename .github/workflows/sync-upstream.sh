#!/usr/bin/env bash
# Syncs this fork with upstream cel-go.
# Run it locally, or from Actions -> "Sync with upstream" -> Run workflow.
#
#   sync-upstream.sh                     # full sync
#   sync-upstream.sh --continue          # after resolving conflicts by hand
#   sync-upstream.sh rename [--reverse]  # just the rename
#
# This fork differs from upstream in exactly one way: every
# `github.com/google/cel-go` import is rewritten to `github.com/authzed/cel-go`.
# Merging upstream directly means both sides edited the same import lines, so it
# conflicts on nearly every file upstream touched -- ~77 conflicts, none real.
#
# So the rename is applied last, and never merged:
#
#   1. un-rename, making our tree match the upstream commit we last synced from
#   2. merge upstream, which now sees no changes on our side
#   3. re-rename
#   4. build and test
#
# Commits, never pushes. The workflow hands the commits to
# peter-evans/create-pull-request.
#
# Merge the resulting PR with a merge commit, NOT a squash. Step 2's merge
# commit is what moves `git merge-base master upstream/master` forward, and that
# is what keeps the next sync clean.
#
# Env:
#   PREFER_UPSTREAM=0  stop on conflicts instead of taking upstream's version
#   NO_TEST=1          skip `go test ./...`
#   UPSTREAM_REMOTE    default: upstream
#   UPSTREAM_URL       default: https://github.com/cel-expr/cel-go.git
#   UPSTREAM_BRANCH    default: master
#   BRANCH             branch to commit on; defaults to a new
#                      sync-upstream-<sha> locally, current branch on CI
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

UPSTREAM_PATH="github.com/google/cel-go"
FORK_PATH="github.com/authzed/cel-go"

UPSTREAM_REMOTE="${UPSTREAM_REMOTE:-upstream}"
UPSTREAM_URL="${UPSTREAM_URL:-https://github.com/cel-expr/cel-go.git}"
UPSTREAM_BRANCH="${UPSTREAM_BRANCH:-master}"
PREFER_UPSTREAM="${PREFER_UPSTREAM:-1}"

# Skipped when verifying: these do not build against unmodified upstream
# either, so a failure there says nothing about the sync.
SKIP_MODULES="repl codelab tools"

log()  { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[33m%s\033[0m\n' "$*"; }
die()  { printf '\n\033[31merror: %s\033[0m\n' "$*" >&2; exit 1; }

# The entire fork delta, as one substitution.
rename() {
  local from="$UPSTREAM_PATH" to="$FORK_PATH" files count
  if [[ "${1:-}" == "--reverse" ]]; then
    from="$FORK_PATH"
    to="$UPSTREAM_PATH"
  fi

  # -I skips binary files, and git grep only looks at tracked ones.
  files="$(git grep -I --name-only --fixed-strings -e "$from" -- . || true)"
  if [[ -z "$files" ]]; then
    echo "rename: no occurrences of ${from}; nothing to do"
    return 0
  fi

  count="$(printf '%s\n' "$files" | wc -l | tr -d ' ')"
  printf '%s\n' "$files" | tr '\n' '\0' \
    | xargs -0 perl -pi -e "s{\\Q${from}\\E}{${to}}g"
  echo "rename: ${from} -> ${to} in ${count} file(s)"
}

# Take upstream's version, and honor upstream's deletions. Correct for a fork
# whose only intentional change is the rename, which goes back on afterwards.
auto_resolve() {
  local unmerged path
  unmerged="$(git diff --name-only --diff-filter=U)"
  [[ -z "$unmerged" ]] && return 0

  warn "Auto-resolving in upstream's favor (PREFER_UPSTREAM=1):"
  while IFS= read -r path; do
    [[ -z "$path" ]] && continue
    if git rev-parse -q --verify ":3:$path" >/dev/null 2>&1; then
      git checkout --theirs -- "$path"
      git add -- "$path"
      echo "  took upstream: $path"
    else
      git rm -q --force -- "$path" >/dev/null
      echo "  upstream deleted: $path"
    fi
  done <<< "$unmerged"
}

# Builds every Go module in the repo. The set is discovered, not listed,
# because it changes as upstream adds and drops modules.
verify() {
  local m
  for m in . $(git ls-files '*/go.mod' | sed 's|/go.mod$||' | sort -u); do
    case " $SKIP_MODULES " in
      *" $m "*) echo "  skipped       $m" ; continue ;;
    esac
    ( cd "$m" && go build ./... ) || die "go build failed in module '$m'"
    echo "  go build OK   $m"
  done
  if [[ "${NO_TEST:-0}" == "1" ]]; then
    echo "  go test skipped (NO_TEST=1)"
  else
    go test ./... >/dev/null || die "go test failed in the root module"
    echo "  go test OK    ."
  fi
}

finish() {
  log "Step 3/4: re-applying the rename"
  rename
  if git diff --quiet && git diff --cached --quiet; then
    echo "Nothing to re-rename."
  else
    git add -A
    git commit -qm "Re-apply authzed rename"
  fi

  log "Step 4/4: verifying"
  verify

  # The fork should be upstream plus the rename and nothing else.
  local extra
  extra="$(git diff "$UPSTREAM_REMOTE/$UPSTREAM_BRANCH" -- . ':(exclude).github' \
    | grep -E '^[+-]' | grep -vE '^(\+\+\+|---)' | grep -cv 'cel-go' || true)"
  echo "  non-rename lines vs upstream (excluding .github/): ${extra}"

  log "Sync complete on '$(git rev-parse --abbrev-ref HEAD)'. Nothing was pushed."
}

case "${1:-}" in
rename)
  shift
  rename "$@"
  exit 0
  ;;
--continue)
  if git rev-parse -q --verify MERGE_HEAD >/dev/null; then
    if git diff --name-only --diff-filter=U | grep -q .; then
      die "still unresolved: $(git diff --name-only --diff-filter=U | tr '\n' ' ')"
    fi
    git commit -qm "Merge ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH} into fork"
  fi
  finish
  exit 0
  ;;
"") ;;
*) die "unknown argument '${1}'" ;;
esac

if git rev-parse -q --verify MERGE_HEAD >/dev/null; then
  die "a merge is already in progress; run 'git merge --abort' or use --continue"
fi
if ! git diff --quiet || ! git diff --cached --quiet; then
  die "working tree is dirty; commit or stash first"
fi

log "Step 1/4: fetching ${UPSTREAM_REMOTE}/${UPSTREAM_BRANCH}"
git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1 \
  || git remote add "$UPSTREAM_REMOTE" "$UPSTREAM_URL"
git fetch --quiet "$UPSTREAM_REMOTE" "$UPSTREAM_BRANCH"
UPSTREAM_REF="$UPSTREAM_REMOTE/$UPSTREAM_BRANCH"
UPSTREAM_SHA="$(git rev-parse --short "$UPSTREAM_REF")"
echo "upstream is at ${UPSTREAM_SHA}"

if git merge-base --is-ancestor "$UPSTREAM_REF" HEAD; then
  log "Already up to date with ${UPSTREAM_REF}. Nothing to do."
  exit 0
fi

# On CI, commit onto the branch that is already checked out.
# create-pull-request only pushes commits as-is when they sit on the base
# branch; otherwise it cherry-picks them, and cherry-pick cannot replay the
# merge commit that step 2 creates.
if [[ -n "${BRANCH:-}" ]]; then
  git checkout -q -B "$BRANCH"
elif [[ -z "${GITHUB_ACTIONS:-}" ]]; then
  git checkout -q -B "sync-upstream-${UPSTREAM_SHA}"
fi
echo "working on '$(git rev-parse --abbrev-ref HEAD)'"

BASE="$(git merge-base HEAD "$UPSTREAM_REF")"

log "Step 2/4: reverting the rename, then merging"
rename --reverse
git add -A
if git diff --cached --quiet; then
  echo "Nothing to un-rename."
else
  git commit -qm "Revert authzed rename for upstream sync"
fi

# If un-renaming did not make our tree identical to the merge base, the fork
# carries changes of its own, and those files are the only ones that can
# conflict below.
DRIFT="$(git diff --name-only "$BASE" HEAD)"
if [[ -n "$DRIFT" ]]; then
  warn "Fork differs from upstream beyond the rename in $(printf '%s\n' "$DRIFT" | wc -l | tr -d ' ') file(s):"
  printf '%s\n' "$DRIFT" | sed 's/^/  /'
  echo "  -> only these files can conflict."
else
  echo "drift check: clean. Fork is pure upstream + rename; the merge cannot conflict."
fi

if ! git merge --no-edit "$UPSTREAM_REF"; then
  if [[ "$PREFER_UPSTREAM" == "1" ]]; then
    auto_resolve
    git commit -qm "Merge ${UPSTREAM_REF} into fork"
  else
    warn "Conflicts (real fork divergence, not rename noise):"
    git diff --name-only --diff-filter=U | sed 's/^/  /'
    cat <<'EOF'

Resolve them, `git add` the files, then run:
  .github/workflows/sync-upstream.sh --continue
EOF
    exit 1
  fi
fi

finish
