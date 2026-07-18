#!/usr/bin/env bash
# gen-changelog.sh — walk `git tag --sort=-creatordate v*` and print a markdown
# changelog to stdout. Groups commits by conventional-commit prefix. Idempotent:
# running it twice on the same repo produces byte-identical output.
#
# Usage:  bash scripts/gen-changelog.sh > CHANGELOG.md
set -euo pipefail

emit_group() {
    local title="$1" pattern="$2" range="$3" printed_header="$4"
    local commits
    commits=$(git log --no-merges --pretty=format:"%h%x09%s" "$range" 2>/dev/null \
        | grep -Ei "^[0-9a-f]+	$pattern" || true)
    if [ -z "$commits" ]; then
        return
    fi
    if [ "$printed_header" = "no" ]; then
        printf "\n"
    fi
    printf "### %s\n" "$title"
    while IFS=$'\t' read -r sha subj; do
        # strip conventional-commit prefix for brevity: "feat: X" -> "X"
        clean=$(printf "%s" "$subj" | sed -E 's/^[a-zA-Z]+(\([^)]+\))?:[[:space:]]*//')
        printf -- "- %s (%s)\n" "$clean" "$sha"
    done <<<"$commits"
}

main() {
    printf "# Changelog\n"

    mapfile -t tags < <(git tag -l 'v*' --sort=-creatordate)
    if [ "${#tags[@]}" -eq 0 ]; then
        printf "\n_no tags yet_\n"
        return
    fi

    for i in "${!tags[@]}"; do
        tag="${tags[$i]}"
        date=$(git log -1 --pretty=format:"%ad" --date=short "$tag")
        if [ "$i" -lt "$((${#tags[@]} - 1))" ]; then
            prev="${tags[$((i + 1))]}"
            range="${prev}..${tag}"
        else
            range="$tag"
        fi

        printf "\n## [%s] - %s\n" "$tag" "$date"
        emit_group "Features"     'feat(\(.+\))?:'  "$range" "yes"
        emit_group "Bug fixes"    'fix(\(.+\))?:'   "$range" "no"
        emit_group "Performance"  'perf(\(.+\))?:'  "$range" "no"
        emit_group "Docs"         'docs(\(.+\))?:'  "$range" "no"
        emit_group "Chore"        'chore(\(.+\))?:' "$range" "no"
    done
}

main "$@"
