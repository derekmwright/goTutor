#!/usr/bin/env bash
# Pure-recall drills: reset a stub, time the attempt, log passes.
#
#   drill.sh start <name>   reset drills/<name>/drill.go to the blank stub,
#                           start the clock, open the editor
#   drill.sh check <name>   run the tests (-race); on pass, log the time
#   drill.sh list           every drill with attempts, last date, last and best time
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
D="$ROOT/drills"
STATE="$D/.state"
LOG="$D/log.tsv" # date<TAB>drill<TAB>seconds

die() { echo "drill: $*" >&2; exit 1; }
names() { for f in "$D"/*/drill.go.stub; do basename "$(dirname "$f")"; done; }
mmss() { printf '%d:%02d' $(($1 / 60)) $(($1 % 60)); }

need() {
  [ -n "${1:-}" ] || die "which drill? one of: $(names | tr '\n' ' ')"
  [ -f "$D/$1/drill.go.stub" ] || die "no drill '$1'; have: $(names | tr '\n' ' ')"
}

cmd_start() {
  need "${1:-}"
  cp "$D/$1/drill.go.stub" "$D/$1/drill.go"
  mkdir -p "$STATE"
  date +%s >"$STATE/$1"
  echo "drill $1: reset to blank, clock started. Check with: task drill:check -- $1"
  "$ROOT/editor/tutor-editor.sh" open "drills/$1" drill.go || true
}

cmd_check() {
  need "${1:-}"
  if ! (cd "$ROOT" && go test -race -count=1 -timeout 20s "./drills/$1"); then
    echo "drill $1: not yet, clock still running"
    exit 1
  fi
  if [ -f "$STATE/$1" ]; then
    secs=$(($(date +%s) - $(cat "$STATE/$1")))
    printf '%s\t%s\t%d\n' "$(date +%F)" "$1" "$secs" >>"$LOG"
    rm -f "$STATE/$1"
    echo "drill $1: PASS in $(mmss "$secs") (logged)"
  else
    echo "drill $1: PASS (no clock running; start one with: task drill -- $1)"
  fi
}

cmd_list() {
  printf '%-12s %8s  %-10s  %6s  %6s\n' DRILL ATTEMPTS LAST "TIME" BEST
  for n in $(names); do
    if [ -f "$LOG" ] && grep -q "	$n	" "$LOG"; then
      awk -F'\t' -v n="$n" '
        $2 == n { c++; last = $1; t = $3; if (best == "" || $3 < best) best = $3 }
        END { printf "%-12s %8d  %-10s  %3d:%02d  %3d:%02d\n", n, c, last, t/60, t%60, best/60, best%60 }
      ' "$LOG"
    else
      printf '%-12s %8d  %-10s  %6s  %6s\n' "$n" 0 never - -
    fi
    [ -f "$STATE/$n" ] && echo "             (clock running)"
  done
  return 0
}

case "${1:-}" in
  start) shift; cmd_start "$@" ;;
  check) shift; cmd_check "$@" ;;
  list) cmd_list ;;
  *) sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
