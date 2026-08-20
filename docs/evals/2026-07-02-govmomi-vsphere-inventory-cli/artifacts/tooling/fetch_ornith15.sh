#!/usr/bin/env bash
# Queue the Ornith 1.5 manifest into /Users/ldh/models.
# Manifest + provenance: ../ornith-1.5-download-manifest.md
#
# First-party ornith-ai only. Smallest-first so the loader is validated before
# the 66 GiB file commits. Resumable: hf download skips complete files, so
# re-running after an interrupt costs nothing.
#
# Usage: fetch_ornith15.sh [--dry-run]

set -uo pipefail

DEST_ROOT="/Users/ldh/models"
DRY=0
[ "${1:-}" = "--dry-run" ] && DRY=1

# repo | file | expected bytes | dest subdir
MANIFEST=(
  "ornith-ai/Ornith-1.5-9B-GGUF|Ornith-1.5-9B-Q8_0.gguf|9527501248|Ornith-1.5-9B-GGUF"
  "ornith-ai/Ornith-1.5-9B-GGUF|Ornith-1.5-9B-BF16.gguf|17920696768|Ornith-1.5-9B-GGUF"
  "ornith-ai/Ornith-1.5-9B-GGUF|mmproj-Ornith-1.5-9B-BF16.gguf|921704416|Ornith-1.5-9B-GGUF"
  "ornith-ai/Ornith-1.5-35B-A3B-GGUF|Ornith-1.5-35B-Q8_0.gguf|37802149120|Ornith-1.5-35B-A3B-GGUF"
  "ornith-ai/Ornith-1.5-35B-A3B-GGUF|Ornith-1.5-35B-BF16.gguf|71066994240|Ornith-1.5-35B-A3B-GGUF"
  "ornith-ai/Ornith-1.5-35B-A3B-GGUF|mmproj-Ornith-1.5-35B-BF16.gguf|902822016|Ornith-1.5-35B-A3B-GGUF"
)

need=0
for row in "${MANIFEST[@]}"; do
  IFS='|' read -r _ _ bytes _ <<<"$row"
  need=$((need + bytes))
done
avail=$(df -k "$DEST_ROOT" | awk 'NR==2{print $4*1024}')
printf 'manifest: %d files, %.2f GiB\n' "${#MANIFEST[@]}" "$(echo "$need/1073741824" | bc -l)"
printf 'available: %.2f GiB\n\n' "$(echo "$avail/1073741824" | bc -l)"
if [ "$need" -gt "$avail" ]; then
  echo "ABORT: manifest exceeds free space." >&2
  exit 1
fi

fail=0
i=0
for row in "${MANIFEST[@]}"; do
  IFS='|' read -r repo file bytes sub <<<"$row"
  i=$((i + 1))
  dest="$DEST_ROOT/$sub"
  target="$dest/$file"
  printf '[%d/%d] %s\n' "$i" "${#MANIFEST[@]}" "$file"

  if [ -f "$target" ] && [ "$(stat -f%z "$target")" = "$bytes" ]; then
    echo "      already complete, skipping"
    continue
  fi
  if [ "$DRY" = 1 ]; then
    echo "      DRY-RUN: hf download $repo $file --local-dir $dest"
    continue
  fi

  mkdir -p "$dest"
  if ! hf download "$repo" "$file" --local-dir "$dest"; then
    echo "      DOWNLOAD FAILED" >&2
    fail=$((fail + 1))
    continue
  fi

  # Size is the integrity gate: a truncated or HTML-error body fails here.
  got=$(stat -f%z "$target" 2>/dev/null || echo 0)
  if [ "$got" != "$bytes" ]; then
    printf '      SIZE MISMATCH: got %s, want %s\n' "$got" "$bytes" >&2
    fail=$((fail + 1))
  else
    echo "      OK ($got bytes)"
  fi
done

echo
if [ "$fail" -ne 0 ]; then
  echo "DONE with $fail failure(s) — re-run to resume." >&2
  exit 1
fi
echo "DONE — all files present and size-verified."
