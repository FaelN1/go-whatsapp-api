#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: clean-eventlogs.sh [--path DIR] [--days N] [--keep-latest N] [--pattern GLOB] [--dry-run] [--no-recurse]

Options:
  --path DIR        Base directory containing event logs (default: ./event_logs)
  --days N          Remove files older than N days (default: 30)
  --keep-latest N   Keep the most recent N files per directory regardless of age (default: 0)
  --pattern GLOB    Limit to files matching the glob (default: *.json)
  --dry-run         Show files that would be deleted without removing them
  --no-recurse      Only inspect the top directory (no subdirectories)
  -h, --help        Show this message
EOF
}

path="./event_logs"
days=30
keep_latest=0
pattern="*.json"
recurse=1
dry_run=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --path)
      path=${2:-}
      shift 2
      ;;
    --days)
      days=${2:-}
      shift 2
      ;;
    --keep-latest)
      keep_latest=${2:-}
      shift 2
      ;;
    --pattern)
      pattern=${2:-}
      shift 2
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    --no-recurse)
      recurse=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown option: %s\n' "$1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ -z "$path" ]]; then
  printf 'Error: --path requires a value.\n' >&2
  exit 2
fi

if [[ ! -d "$path" ]]; then
  printf "Directory not found: %s\n" "$path" >&2
  exit 2
fi

if ! [[ "$days" =~ ^[0-9]+$ ]]; then
  printf 'Error: --days must be a non-negative integer.\n' >&2
  exit 2
fi

if ! [[ "$keep_latest" =~ ^[0-9]+$ ]]; then
  printf 'Error: --keep-latest must be a non-negative integer.\n' >&2
  exit 2
fi

printf 'Cleaning event logs in: %s\n' "$path"
printf 'Age threshold: %s days\n' "$days"
if (( keep_latest > 0 )); then
  printf 'Keep latest %s files per directory\n' "$keep_latest"
fi
if (( dry_run )); then
  printf 'Mode: DRY RUN (no files will be deleted)\n'
fi

find_opts=(-type f -name "$pattern")
if (( recurse == 0 )); then
  find_opts+=( -maxdepth 1 )
fi

# shellcheck disable=SC2034
declare -A dir_map
while IFS= read -r -d '' file; do
  dir=$(dirname "$file")
  mtime=$(stat -c '%Y' "$file")
  dir_map[$dir]="${dir_map[$dir]}$mtime::$file\n"
done < <(find "$path" "${find_opts[@]}" -print0 2>/dev/null)

if (( ${#dir_map[@]} == 0 )); then
  printf 'No files found matching pattern.\n'
  exit 0
fi

now=$(date +%s)
threshold=$(( now - days * 86400 ))

files_to_delete=()
for dir in "${!dir_map[@]}"; do
  list=${dir_map[$dir]}
  mapfile -t sorted < <(printf '%s' "$list" | sed '/^$/d' | sort -t: -k1,1nr)
  idx=0
  for entry in "${sorted[@]}"; do
    mtime=${entry%%::*}
    file=${entry#*::}
    if (( keep_latest > 0 )) && (( idx < keep_latest )); then
      ((idx++))
      continue
    fi
    ((idx++))
    if (( mtime < threshold )); then
      files_to_delete+=("$file")
    fi
  done
done

if (( ${#files_to_delete[@]} == 0 )); then
  printf 'No files to delete after applying filters.\n'
  exit 0
fi

total_bytes=0
for file in "${files_to_delete[@]}"; do
  size=$(stat -c '%s' "$file")
  total_bytes=$(( total_bytes + size ))
done

printf 'Files selected for deletion: %d (%.2f MB)\n' "${#files_to_delete[@]}" "$(awk -v b=$total_bytes 'BEGIN { printf "%.2f", b/1024/1024 }')"

if (( dry_run )); then
  printf -- '--- DRY RUN LIST ---\n'
  printf '%s\n' "${files_to_delete[@]}"
  exit 0
fi

printf 'Proceed to delete %d files? [y/N] ' "${#files_to_delete[@]}"
read -r answer
if [[ ! $answer =~ ^[Yy]$ ]]; then
  printf 'Aborted by user.\n'
  exit 0
fi

deleted=0
errors=0
for file in "${files_to_delete[@]}"; do
  if rm -f -- "$file"; then
    ((deleted++))
  else
    printf 'Failed to delete %s\n' "$file" >&2
    ((errors++))
  fi
done

printf 'Deleted: %d files. Errors: %d\n' "$deleted" "$errors"
