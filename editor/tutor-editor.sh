#!/usr/bin/env bash
# Opens and drives the student's editor beside Claude Code.
#
#   tutor-editor.sh open   <lesson-dir> [file...]   open (or reuse) the editor on a lesson
#   tutor-editor.sh goto   <file> [line]            show a file at a line
#   tutor-editor.sh reload                          reload buffers changed on disk
#   tutor-editor.sh resize <percent>                editor pane width (tmux only)
#   tutor-editor.sh zoom                            toggle the editor pane full-window (tmux only)
#   tutor-editor.sh focus                           move the keyboard to the editor (tmux only)
#   tutor-editor.sh close                           close the editor
#   tutor-editor.sh status                          print how the editor is attached
#
# Prefers Neovim (driven over its --listen socket) and falls back to Vim
# (driven with tmux send-keys). Inside tmux the editor is a split pane;
# under Windows Terminal (WSL) it is a split of the current window.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SOCK="${XDG_RUNTIME_DIR:-/tmp}"
SOCK="${SOCK%/}/gotutor-nvim.sock"
WIDTH="${GOTUTOR_EDITOR_WIDTH:-55}"

die() { echo "tutor-editor: $*" >&2; exit 1; }
abspath() { (cd "$(dirname "$1")" && echo "$(pwd)/$(basename "$1")"); }

has_nvim() { command -v nvim >/dev/null 2>&1; }
nvim_alive() { [ -S "$SOCK" ] && nvim --server "$SOCK" --remote-expr 1 >/dev/null 2>&1; }
nvim_send() { nvim --server "$SOCK" --remote-send "$1"; }
# Escapes a path for use on a Vim command line.
vim_escape() { printf '%s' "$1" | sed 's/[ \\|%#"]/\\&/g'; }

in_tmux() { [ -n "${TMUX:-}" ]; }
tmux_pane() {
  in_tmux || return 1
  local id
  id=$(tmux show-options -wqv @gotutor_pane 2>/dev/null || true)
  [ -n "$id" ] && tmux list-panes -a -F '#{pane_id}' | grep -qx -- "$id" && echo "$id"
}
vim_keys() { # sends an Ex command to a plain Vim in the tmux pane
  local pane
  pane=$(tmux_pane) || die "no editor pane to drive"
  tmux send-keys -t "$pane" Escape ":$1" Enter
}

find_wt() {
  command -v wt.exe 2>/dev/null && return
  local f
  for f in /mnt/c/Users/*/AppData/Local/Microsoft/WindowsApps/wt.exe; do
    [ -x "$f" ] && echo "$f" && return
  done
  return 1
}

cmd_open() {
  [ $# -ge 1 ] || die "usage: open <lesson-dir> [file...]"
  local dir
  dir=$(cd "$1" && pwd) || die "no such lesson dir: $1"
  shift
  local files=() f
  for f in "$@"; do
    case "$f" in /*) files+=("$f") ;; *) files+=("$dir/$f") ;; esac
  done

  # Reuse a running editor.
  if has_nvim && nvim_alive; then
    nvim_send "<C-\\><C-N>:cd $(vim_escape "$dir")<CR>"
    for f in "${files[@]}"; do nvim_send "<C-\\><C-N>:tab drop $(vim_escape "$f")<CR>"; done
    cmd_focus >/dev/null 2>&1 || true
    echo "reused nvim ($SOCK)"
    return
  fi
  if tmux_pane >/dev/null && ! has_nvim; then
    vim_keys "cd $(vim_escape "$dir")"
    for f in "${files[@]}"; do vim_keys "tab drop $(vim_escape "$f")"; done
    cmd_focus >/dev/null
    echo "reused vim pane $(tmux_pane)"
    return
  fi

  local cmd
  if has_nvim; then
    rm -f "$SOCK"
    cmd=(nvim -u "$ROOT/editor/init.lua" --listen "$SOCK" -p "${files[@]}")
  else
    cmd=(vim -u "$ROOT/editor/vimrc" -p "${files[@]}")
  fi

  if in_tmux; then
    local pane
    pane=$(tmux split-window -h -l "${WIDTH}%" -c "$dir" -P -F '#{pane_id}' \
      env GOTUTOR_ROOT="$ROOT" PATH="$PATH" "${cmd[@]}")
    tmux set-option -w @gotutor_pane "$pane"
    echo "opened ${cmd[0]} in tmux pane $pane"
    return
  fi

  local wt
  if [ -n "${WSL_DISTRO_NAME:-}" ] && wt=$(find_wt); then
    "$wt" -w 0 split-pane -V --size "0.$WIDTH" \
      wsl.exe -d "$WSL_DISTRO_NAME" --cd "$dir" -e \
      env GOTUTOR_ROOT="$ROOT" PATH="$PATH" "${cmd[@]}" >/dev/null 2>&1 &
    disown
    echo "opened ${cmd[0]} in a Windows Terminal split"
    return
  fi

  echo "No tmux or Windows Terminal found. Run this in another terminal:" >&2
  printf '  cd %q &&' "$dir" >&2; printf ' %q' "${cmd[@]}" >&2; echo >&2
  exit 2
}

cmd_goto() {
  [ $# -ge 1 ] || die "usage: goto <file> [line]"
  local f line="${2:-1}"
  f=$(abspath "$1")
  if has_nvim && nvim_alive; then
    nvim_send "<C-\\><C-N>:tab drop $(vim_escape "$f")<CR>:${line}<CR>zz"
  else
    vim_keys "tab drop $(vim_escape "$f")"; vim_keys "$line"
  fi
  cmd_focus >/dev/null 2>&1 || true
  echo "showing $f:$line"
}

cmd_reload() {
  if has_nvim && nvim_alive; then
    nvim_send "<C-\\><C-N>:checktime<CR>"
  else
    vim_keys "checktime"
  fi
  echo "reloaded"
}

cmd_resize() {
  local pane
  pane=$(tmux_pane) || die "resize needs the editor in a tmux pane"
  tmux resize-pane -t "$pane" -x "${1:-$WIDTH}%"
  echo "editor at ${1:-$WIDTH}%"
}

cmd_zoom() {
  local pane
  pane=$(tmux_pane) || die "zoom needs the editor in a tmux pane"
  tmux resize-pane -t "$pane" -Z
  echo "toggled zoom"
}

cmd_focus() {
  local pane
  pane=$(tmux_pane) || die "focus needs the editor in a tmux pane"
  tmux select-pane -t "$pane"
  echo "focused $pane"
}

cmd_close() {
  if has_nvim && nvim_alive; then
    nvim_send "<C-\\><C-N>:wall | qall<CR>" || true
  elif pane=$(tmux_pane); then
    tmux send-keys -t "$pane" Escape ":wall | qall" Enter
  fi
  echo "closed"
}

cmd_status() {
  local how="none"
  if has_nvim && nvim_alive; then how="nvim socket $SOCK"; fi
  if pane=$(tmux_pane); then how="$how; tmux pane $pane"; fi
  echo "editor: $how (nvim installed: $(has_nvim && echo yes || echo no), tmux: $(in_tmux && echo yes || echo no))"
}

action="${1:-}"
[ -n "$action" ] && shift
case "$action" in
  open) cmd_open "$@" ;;
  goto) cmd_goto "$@" ;;
  reload) cmd_reload ;;
  resize) cmd_resize "$@" ;;
  zoom) cmd_zoom ;;
  focus) cmd_focus ;;
  close) cmd_close ;;
  status) cmd_status ;;
  *) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
