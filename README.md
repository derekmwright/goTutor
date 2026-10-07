# goTutor

An interactive Go tutoring workspace for [Claude Code](https://claude.com/claude-code).
Claude is the tutor; you are the student. Each lesson is a short conversation,
a scaffolded exercise with `// TODO(you)` markers, tests that pin down the
behaviour (goroutine leaks and deadlocks included), and a review. Short
pure-recall drills warm up each session.

What's in the box:

- **A tutor brief** (`CLAUDE.md`) that tells Claude how to teach: talk first,
  Socratic hints, never write the student's code, review like a mentor.
- **A Claude Code mod** (`.claude/skills/gotutor/`) that adds `/learn`,
  `/hint`, `/review`, `/check`, `/tutor`, a lesson band above the prompt with
  live test status, a watcher that re-runs `go test -race` on save, and tools
  Claude uses to start lessons and drive your editor.
- **An editor split**: Neovim (or Vim) opens beside Claude in tmux or Windows
  Terminal; Claude can point at a line while explaining.
- **Lessons** in `lessons/`, **drills** in `drills/`, and `PROGRESS.md`, which
  the tutor keeps current.

## Requirements

- Go 1.25+ (`testing/synctest`, `sync.WaitGroup.Go`, range-over-func)
- Claude Code 2.1.290+ with mods enabled
- [Task](https://taskfile.dev) (`task`), `tmux` (recommended), Neovim 0.11+ or Vim
- `gopls` for the editor's LSP (optional but nice)

## Get started

```sh
git clone https://github.com/derekmwright/goTutor
cd goTutor
task nvim:install   # optional: Neovim to ~/.local, no sudo
task start          # Claude Code inside a tmux session named gotutor
```

In Claude Code, accept the trust prompt for the directory, then:

```
/learn
```

Two lessons (channel foundations, pipelines and bounded parallelism) and six
drills are already scaffolded, so Claude will suggest starting there. For a
new topic, `/learn <topic>`: Claude talks through what you know, scaffolds
`lessons/NN-slug/`, opens the editor beside you, and runs the baseline tests. Edit the `TODO(you)` parts,
save, and watch the band above the prompt. `/hint` for the next nudge,
`/review` when you think you're done, `/check` to run the tests by hand.

Start each session with a warm-up: `task drill:list` shows what's due, Claude
picks one and runs `task drill -- <name>`; you check with
`task drill:check -- <name>`.

## Layout

```
CLAUDE.md               the tutor's instructions
Taskfile.yml            tasks (task --list)
.claude/skills/gotutor/ the Claude Code mod (hooks/register.tsx)
editor/                 tutor-editor.sh launcher/driver, init.lua, vimrc
lessons/NN-slug/        README, code with TODO(you) stubs, tests
drills/                 warm-up katas; log.tsv (your times) is git-ignored
PROGRESS.md             what you've finished and what to revisit
```

## Hacking on the mod

```sh
task mod:check      # claude plugin validate + claude plugin test
/reload-plugins     # in the session, after editing the mod
```

For hot reload while developing, run `claude --plugin-dir .claude/skills/gotutor`
from the repo root. If you keep a working copy elsewhere,
`MOD_SRC=/path/to/copy task mod:sync` copies it back into the repo.

## Make it yours

Lessons and drills here are a starting point. Fork it, clear `PROGRESS.md`,
and let Claude pick the next topic from the curriculum ideas in `CLAUDE.md`
or follow your own interests.
