import { describe, expect, mock, test } from 'claude-code/testing'

import { parseGoTest, summary } from './gotest'

const PASSING = `=== RUN   TestFanOut
--- PASS: TestFanOut (0.00s)
=== RUN   TestFanIn
--- FAIL: TestFanIn (0.01s)
    fanin_test.go:20: got 2 values, want 3
FAIL
FAIL\tgotutor/lessons/01-fan [0.02s]
`

const BAND_PROPS = {
  hasSurvey: false,
  isWorking: false,
  maxRows: 6,
  bodyColumns: 100,
  scroll: { offset: 0, bodyRows: 6 },
  view: {},
}

describe('go test parsing', () => {
  test('reads pass and fail cases', async () => {
    const run = parseGoTest(1, PASSING)
    expect(run.status).toBe('fail')
    expect(run.tests).toEqual([
      { name: 'TestFanOut', result: 'pass' },
      { name: 'TestFanIn', result: 'fail' },
    ])
    expect(summary({ ...run, todos: 0, at: 0 })).toBe('✗ 1/2 tests')
  })

  test('spots a build failure and a deadlock', async () => {
    expect(parseGoTest(1, '# gotutor/lessons/01\n./fan.go:9:2: undefined: x\nFAIL\tgotutor/lessons/01 [build failed]\n').status).toBe('build')
    expect(parseGoTest(1, 'panic: test timed out after 20s\n').status).toBe('timeout')
  })
})

test('start_lesson records the lesson, runs tests and draws the band', async ($, on) => {
  const argvs: string[][] = []
  mock.clock(on, { now: 1_000 })
  mock.store(on)
  on('fs.exists', async () => ({ value: true }))
  on('fs.list', async () => ({ value: [{ name: 'fan.go', kind: 'file' as const, size: 10, mtimeMs: 1, isLink: false }] }))
  on('fs.read', async () => ({ value: 'func FanIn() { // TODO(you): merge the channels\n}' }))
  on('process.run', async (_$, e) => {
    argvs.push([...e.argv])
    const isGo = e.argv[0] === 'go'
    const stdout = isGo ? PASSING : 'opened nvim in tmux pane %3'
    return { value: { exitCode: isGo ? 1 : 0, stdout, stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })

  const res = await $.tool.call({
    tool: 'mcp__gotutor__start_lesson',
    input: { dir: 'lessons/01-fan', title: 'Fan-in', objectives: ['merge channels'], files: ['fan.go'] },
  })
  expect(String(res.result)).toContain('Lesson "Fan-in" started')
  expect(String(res.result)).toContain('✗ 1/2 tests')
  expect(argvs.some(a => a[0] === 'go' && a.includes('./lessons/01-fan/...'))).toBe(true)
  expect(argvs.some(a => a[0] === 'editor/tutor-editor.sh' && a[1] === 'open' && a.includes('fan.go'))).toBe(true)

  for (const surface of ['terminal', 'desktop'] as const) {
    const ui = await $.ui.mount({ plugin: 'gotutor', surface, component: 'AbovePrompt', props: BAND_PROPS })
    expect(await ui.find({ type: 'Text', text: 'Fan-in' })).toBeDefined()
    expect(await ui.find({ type: 'Text', text: 'TODO×1' })).toBeDefined()
    expect(await ui.find({ key: 'hint' })).toBeDefined()
    await ui.unmount()
  }
})

const RUN = { origin: { kind: 'composer' as const }, presentation: { isFullscreen: false, columns: 120 } }

test('commands and the band send their prompts to the model', async ($, on) => {
  const clock = mock.clock(on, { now: 1 })
  mock.store(on)
  const sent: string[] = []
  on('prompt.submit', async (_$, e) => {
    sent.push(e.text)
    return { text: e.text }
  })
  on('fs.exists', async () => ({ value: true }))
  on('fs.list', async () => ({ value: [] }))
  on('process.run', async () => ({ value: { exitCode: 0, stdout: '', stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }))

  // Before the fix, submitting inside command.run was refused and nothing reached the model.
  await $.command.run({ command: 'learn', args: 'fan-out fan-in', ...RUN })
  await $.command.run({ command: 'hint', args: '', ...RUN })
  await clock.advance(10)
  expect(sent[0]).toContain('Teach me: fan-out fan-in')
  expect(sent[1]).toContain('hint')

  await $.tool.call({
    tool: 'mcp__gotutor__start_lesson',
    input: { dir: 'lessons/01-fan', title: 'Fan-in', objectives: [], files: [], openEditor: false },
  })
  const ui = await $.ui.mount({ plugin: 'gotutor', surface: 'terminal', component: 'AbovePrompt', props: BAND_PROPS })
  await ui.press({ key: 'review' })
  await clock.advance(10)
  expect(sent[2]).toContain('Review my current code')
  await ui.unmount()
})
