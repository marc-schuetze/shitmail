#!/usr/bin/env bash
# Push a sanitised snapshot of HEAD to the public GitHub mirror.
#
# The private build repo carries .forgejo/ (registry, cache prefix, dispatch
# hooks). The mirror gets one squashed commit per publish, parented on the
# upstream MailTub release, with .forgejo/ removed. History with infra details
# never leaves the private forge.
set -euo pipefail
UPSTREAM=4a550bb5   # MailTub v1.0.0, the public ancestor
REMOTE=${1:-github}
cd "$(git rev-parse --show-toplevel)"
if git ls-files | grep -v '^\.forgejo/' | xargs grep -l -i -E 'schuetze\.io|zkm\.de|harbor\.|100\.100\.' -- 2>/dev/null | grep -q .; then
  echo "private references found outside .forgejo/, refusing:" >&2
  git ls-files | grep -v '^\.forgejo/' | xargs grep -n -i -E 'schuetze\.io|zkm\.de|harbor\.|100\.100\.' -- >&2
  exit 1
fi
export GIT_INDEX_FILE=$(mktemp)
git read-tree HEAD
git rm -rq --cached .forgejo
TREE=$(git write-tree)
rm -f "$GIT_INDEX_FILE"; unset GIT_INDEX_FILE
C=$(git commit-tree "$TREE" -p "$UPSTREAM" -m "shitmail snapshot $(date +%Y-%m-%d) ($(git rev-parse --short HEAD))")
git push --force "$REMOTE" "$C:refs/heads/main"
echo "published $C to $REMOTE/main"
