#!/usr/bin/env bash
# Build the site and publish it to the gh-pages branch through the GitHub API (no Actions, no local
# git state touched). Needs gh (logged in, repo scope), jq and go. Run from the repo root:
#   bash site/publish.sh
set -eu
repo=$(gh repo view --json nameWithOwner -q .nameWithOwner)
out=$(mktemp -d)
trap 'rm -rf "$out"' EXIT
go run ./site/gen -out "$out/site"
: > "$out/site/.nojekyll" # serve the files as they are

# one blob per file, then a tree, a commit on top of the current gh-pages (if any), and the ref
entries="$out/tree.jsonl"
: > "$entries"
(cd "$out/site" && find . -type f | sed 's|^\./||' | sort) | while read -r f; do
	jq -n --rawfile c <(base64 -w0 "$out/site/$f" 2>/dev/null || base64 <"$out/site/$f" | tr -d '\n') '{content:$c,encoding:"base64"}' >"$out/blob.json"
	sha=$(gh api "repos/$repo/git/blobs" --input "$out/blob.json" -q .sha)
	jq -n --arg p "$f" --arg s "$sha" '{path:$p,mode:"100644",type:"blob",sha:$s}' >>"$entries"
	echo "  $f"
done
tree=$(jq -s '{tree:.}' "$entries" | gh api "repos/$repo/git/trees" --input - -q .sha)
parent=$(gh api "repos/$repo/git/ref/heads/gh-pages" -q .object.sha 2>/dev/null | grep -E '^[0-9a-f]{40}$' || true) # a 404 prints its error body
src=$(git rev-parse --short HEAD)
commit=$(jq -n --arg m "site: publish from $src" --arg t "$tree" --arg p "$parent" \
	'{message:$m,tree:$t,parents:(if $p=="" then [] else [$p] end)}' | gh api "repos/$repo/git/commits" --input - -q .sha)
if [ -n "$parent" ]; then
	gh api -X PATCH "repos/$repo/git/refs/heads/gh-pages" -f sha="$commit" >/dev/null
else
	gh api "repos/$repo/git/refs" -f ref=refs/heads/gh-pages -f sha="$commit" >/dev/null
fi
echo "published $commit to gh-pages of $repo"
