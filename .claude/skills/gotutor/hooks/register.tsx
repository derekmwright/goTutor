import { atom, read, update } from 'claude-code'
import type { EngineInterface, Register } from 'claude-code'

import type { Lesson, TestRun } from '../types'
import { countTodos, parseGoTest, summary, tailOf } from './gotest'

const PANE = 'gotutor'
const EDITOR = 'editor/tutor-editor.sh'
const lesson = atom({ plugin: 'gotutor', key: 'lesson' } as const, null)
const run = atom({ plugin: 'gotutor', key: 'run' } as const, null)

const HINT =
  'Give me a hint for the current lesson: only the next rung of the hint ladder, no solution code.'
const REVIEW =
  'Review my current code for this lesson (read the files fresh and look at the latest test run). ' +
  'Tell me what is right, what is wrong or racy, and ask me a question instead of fixing it.'

const EDITOR_ACTIONS = ['open', 'goto', 'reload', 'resize', 'zoom', 'focus', 'close', 'status'] as const

// Module state: lost on reload, which only costs one extra test run.
let isRunning = false
let lastSignature = ''

async function sourceFiles($: EngineInterface, dir: string) {
  const entries = await $.fs.list(dir).catch(() => [])
  return entries.filter(f => f.kind === 'file' && f.name.endsWith('.go'))
}

async function runTests($: EngineInterface): Promise<{ run: TestRun; output: string } | null> {
  const current = await read($, lesson)
  if (!current || isRunning) return null
  isRunning = true
  try {
    const at = await $.clock.now()
    await update($, run, (prev): TestRun => ({ ...(prev ?? { tests: [], tail: '', todos: 0 }), status: 'running', at }))
    $.ui.status(`go: ${current.title} · running…`)

    const res = await $.process.run(
      ['go', 'test', '-race', '-count=1', '-timeout', '20s', '-v', `./${current.dir}/...`],
      { timeoutMs: 120_000 },
    )
    const output = `${res.stdout}${res.stderr}`

    let todos = 0
    for (const f of await sourceFiles($, current.dir)) {
      if (f.name.endsWith('_test.go')) continue
      todos += countTodos(await $.fs.read(`${current.dir}/${f.name}`))
    }

    const result: TestRun = { ...parseGoTest(res.exitCode, output), todos, at: await $.clock.now() }
    await update($, run, () => result)
    $.ui.status(`go: ${current.title} · ${summary(result)}`)
    if (result.status === 'pass') $.ui.toast(`goTutor: all tests pass for ${current.title}`)

    return { run: result, output }
  } finally {
    isRunning = false
  }
}

async function editor($: EngineInterface, args: string[]) {
  const exists = await $.fs.exists(EDITOR)
  if (!exists) return { ok: false, text: `${EDITOR} not found: start Claude Code from the goTutor repo root.` }
  const res = await $.process.run([EDITOR, ...args], { timeoutMs: 15_000 })
  return { ok: res.exitCode === 0, text: `${res.stdout}${res.stderr}`.trim() }
}

async function openEditor($: EngineInterface) {
  const current = await read($, lesson)
  if (!current) return { ok: false, text: 'No lesson is active.' }
  return editor($, ['open', current.dir, ...current.files])
}

// Submitting from inside command.run would wait on the turn that hook holds,
// so the prompt goes out from a timer a moment later.
function ask($: EngineInterface, text: string) {
  $.clock.after(1, () => {
    $.prompt.submit({ text, asUser: true }).catch(err => $.ui.toast(`goTutor: could not send prompt: ${String(err)}`))
  })
}

// Re-runs the tests whenever a .go file in the lesson changes on disk.
async function watch($: EngineInterface) {
  const current = await read($, lesson)
  if (!current || isRunning) return
  const files = await sourceFiles($, current.dir)
  const signature = files.map(f => `${f.name}:${f.mtimeMs}`).join('|')
  const isChanged = lastSignature !== '' && signature !== lastSignature
  lastSignature = signature
  if (isChanged) await runTests($)
}

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    const saved = (await $.store.get('lesson')) as Lesson | undefined
    if (saved) {
      await update($, lesson, () => saved)
      $.ui.status(`go: ${saved.title} · ${summary(await read($, run))}`)
    }

    const commands = [
      { name: 'learn', description: 'goTutor: start a lesson on a Go topic', argumentHint: '[topic]' },
      { name: 'hint', description: 'goTutor: next hint for the current lesson' },
      { name: 'review', description: 'goTutor: have the tutor review your code' },
      { name: 'check', description: 'goTutor: run the lesson tests now' },
      { name: 'tutor', description: 'goTutor: show the lesson pane' },
      { name: 'editor', description: 'goTutor: open the editor on the lesson', argumentHint: '[zoom|focus|close|status]' },
    ]
    for (const c of commands) await $.command.register(c).catch(() => undefined)

    await $.tool.register({
      name: 'start_lesson',
      description:
        'Begin a goTutor lesson after scaffolding its folder. Records the lesson, shows it in the ' +
        "student's lesson band, runs the baseline tests, and opens the student's editor (a tmux or " +
        'Windows Terminal split beside Claude) on the files they will edit.',
      inputSchema: {
        type: 'object',
        properties: {
          dir: { type: 'string', description: 'Lesson folder relative to the repo root, e.g. lessons/03-fan-in' },
          title: { type: 'string', description: 'Short title, e.g. "Fan-out / fan-in"' },
          objectives: { type: 'array', items: { type: 'string' }, description: '2-5 things the student will be able to do' },
          files: { type: 'array', items: { type: 'string' }, description: 'Files the student edits, relative to dir' },
          openEditor: { type: 'boolean', description: 'Open the editor (default true)' },
        },
        required: ['dir', 'title', 'objectives', 'files'],
      },
    })
    await $.tool.register({
      name: 'run_tests',
      description:
        "Run the current lesson's tests (go test -race -v, 20s timeout) and return the output. " +
        "Also refreshes the student's lesson band. Tests re-run by themselves whenever the student saves.",
      inputSchema: { type: 'object', properties: {} },
    })
    await $.tool.register({
      name: 'editor',
      description:
        "Drive the student's editor. open: (re)open on the lesson files. goto: show file at line " +
        '(use to point at code while explaining). reload: reload buffers after you edit files. ' +
        'resize: editor width percent. zoom: toggle full-window. focus: give the student the keyboard. ' +
        'close, status.',
      inputSchema: {
        type: 'object',
        properties: {
          action: { type: 'string', enum: EDITOR_ACTIONS },
          file: { type: 'string', description: 'goto: path relative to the repo root' },
          line: { type: 'number', description: 'goto: line number' },
          percent: { type: 'number', description: 'resize: width percent' },
        },
        required: ['action'],
      },
    })
    await $.tool.register({
      name: 'finish_lesson',
      description: 'Close the current lesson once the student has it working and you have debriefed. Clears the band.',
      inputSchema: {
        type: 'object',
        properties: { notes: { type: 'string', description: 'One line: what clicked, what to revisit' } },
        required: ['notes'],
      },
    })

    $.clock.every(1500, () => void watch($))

    return next(e)
  })

  on('tool.call', { tool: 'mcp__gotutor__start_lesson' }, async ($, e) => {
    const input = e.input as Partial<Lesson> & { openEditor?: boolean }
    if (!input.dir || !(await $.fs.exists(input.dir))) {
      return { result: `Lesson folder ${input.dir} does not exist yet: scaffold it first.` }
    }
    const next: Lesson = {
      dir: input.dir.replace(/^\.\//, '').replace(/\/$/, ''),
      title: input.title ?? input.dir,
      objectives: input.objectives ?? [],
      files: input.files ?? [],
      startedAt: await $.clock.now(),
    }
    await update($, lesson, () => next)
    await update($, run, () => null)
    await $.store.set('lesson', next)
    lastSignature = ''

    const baseline = await runTests($)
    const opened = input.openEditor === false ? null : await openEditor($)

    return {
      result: [
        `Lesson "${next.title}" started in ${next.dir}.`,
        opened ? `Editor: ${opened.text}` : 'Editor not opened.',
        `Baseline: ${summary(baseline?.run ?? null)}`,
        baseline ? tailOf(baseline.output, 30) : '',
      ].join('\n'),
    }
  })

  on('tool.call', { tool: 'mcp__gotutor__run_tests' }, async $ => {
    const done = await runTests($)
    if (!done) return { result: (await read($, lesson)) ? 'Tests are already running; try again shortly.' : 'No lesson is active.' }
    return {
      result: `${summary(done.run)} · TODO(you) left: ${done.run.todos}\n\n${tailOf(done.output, 150)}`,
    }
  })

  on('tool.call', { tool: 'mcp__gotutor__editor' }, async ($, e) => {
    const input = e.input as { action: string; file?: string; line?: number; percent?: number }
    if (input.action === 'open') return { result: (await openEditor($)).text }
    const args = [input.action]
    if (input.action === 'goto') args.push(input.file ?? '', String(input.line ?? 1))
    if (input.action === 'resize') args.push(String(input.percent ?? 55))
    return { result: (await editor($, args)).text }
  })

  on('tool.call', { tool: 'mcp__gotutor__finish_lesson' }, async ($, e) => {
    const current = await read($, lesson)
    if (!current) return { result: 'No lesson is active.' }
    const notes = String((e.input as { notes?: string }).notes ?? '')
    const history = ((await $.store.get('history')) as unknown[] | undefined) ?? []
    await $.store.set('history', [...history, { ...current, notes, finishedAt: await $.clock.now() }])
    await $.store.delete('lesson')
    await update($, lesson, () => null)
    await update($, run, () => null)
    $.ui.status(undefined)
    $.ui.toast(`goTutor: finished ${current.title}`)
    return { result: `Finished "${current.title}". Remember to update PROGRESS.md.` }
  })

  on('command.run', { command: 'learn' }, async ($, e) => {
    const topic = e.args.trim()
    ask(
      $,
      topic
        ? `Teach me: ${topic}. Start with a short conversation to see what I already know, then scaffold a lesson.`
        : 'Suggest what I should learn next, based on PROGRESS.md. Offer 3 options.',
    )
    return { text: topic ? `Starting a lesson on ${topic}…` : 'Picking a next topic…' }
  })

  on('command.run', { command: 'hint' }, async $ => {
    ask($, HINT)
    return { text: 'Asking for a hint…' }
  })

  on('command.run', { command: 'review' }, async $ => {
    ask($, REVIEW)
    return { text: 'Asking for a review…' }
  })

  on('command.run', { command: 'check' }, async $ => {
    const done = await runTests($)
    if (!done) return { text: 'No lesson is active (or tests are already running).' }
    return { text: `${summary(done.run)} · TODO(you) left: ${done.run.todos}\n${tailOf(done.output, 25)}` }
  })

  on('command.run', { command: 'tutor' }, async $ => {
    await $.ui.open({ id: PANE, title: 'goTutor', closeOnEscape: true })
    return { text: 'Lesson pane opened.' }
  })

  on('command.run', { command: 'editor' }, async ($, e) => {
    const action = e.args.trim()
    if (action && action !== 'open') return { text: (await editor($, [action])).text }
    return { text: (await openEditor($)).text }
  })

  // The student's own prompts carry the lesson and the latest run, so the tutor
  // never has to ask "did the tests pass?".
  on('prompt.submit', async ($, e, next) => {
    const current = await read($, lesson)
    if (!current || e.origin.kind !== 'composer') return next(e)
    const last = await read($, run)
    const note = `goTutor: active lesson "${current.title}" in ${current.dir}; latest test run: ${summary(last)}; TODO(you) left: ${last?.todos ?? '?'}.`
    return next({ ...e, context: [...(e.context ?? []), note] })
  })

  // One line above the prompt while a lesson is active.
  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const current = await read($, lesson)
    if (!current || e.props.hasSurvey) return next(e)
    const last = await read($, run)
    const { Box, Button, Text } = $.ui.resolve(e)
    const color = !last || last.status === 'running' ? 'yellow' : last.status === 'pass' ? 'green' : 'red'

    return (
      <Box flexDirection="row" gap={1} flexWrap="wrap">
        <Text bold>goTutor</Text>
        <Text>{current.title}</Text>
        <Text color={color}>{summary(last)}</Text>
        {last && last.todos > 0 && <Text dimColor>TODO×{last.todos}</Text>}
        <Button key="check" label="test" hotkey="t" plain onPress={() => void runTests($)} />
        <Button key="hint" label="hint" hotkey="h" plain onPress={() => ask($, HINT)} />
        <Button key="review" label="review" hotkey="r" plain onPress={() => ask($, REVIEW)} />
        <Button key="editor" label="editor" hotkey="e" plain onPress={() => void openEditor($)} />
        <Button key="more" label="more" hotkey="m" plain onPress={() => void $.ui.open({ id: PANE, title: 'goTutor', closeOnEscape: true })} />
      </Box>
    )
  })

  on('ui.render', { component: 'Pane', requestId: PANE }, async ($, e) => {
    const { Box, Text, Code } = $.ui.resolve(e)
    const current = await read($, lesson)
    if (!current) {
      return (
        <Box flexDirection="column">
          <Text bold>goTutor</Text>
          <Text dimColor>No lesson yet. Try /learn concurrency fan-out fan-in</Text>
        </Box>
      )
    }
    const last = await read($, run)

    return (
      <Box flexDirection="column" gap={1}>
        <Box flexDirection="column">
          <Text bold>{current.title}</Text>
          <Text dimColor>{current.dir}</Text>
        </Box>
        <Box flexDirection="column">
          <Text bold>Objectives</Text>
          {current.objectives.map(o => (
            <Text>• {o}</Text>
          ))}
        </Box>
        <Box flexDirection="column">
          <Text bold>Tests: {summary(last)}</Text>
          {(last?.tests ?? []).map(t => (
            <Text color={t.result === 'pass' ? 'green' : t.result === 'fail' ? 'red' : undefined} dimColor={t.result === 'skip'}>
              {t.result === 'pass' ? '✓' : t.result === 'fail' ? '✗' : '–'} {t.name}
            </Text>
          ))}
          {last && <Text dimColor>TODO(you) markers left: {last.todos}</Text>}
        </Box>
        {last && last.status !== 'pass' && last.tail && (
          <Code source={tailOf(last.tail, Math.max(5, (e.viewport?.rows ?? 30) - 18))} wrap="wrap" />
        )}
      </Box>
    )
  })
}
