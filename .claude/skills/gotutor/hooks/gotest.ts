import type { TestCase, TestRun } from '../types'

const CASE = /^\s*--- (PASS|FAIL|SKIP): (\S+)/

/** Reads `go test -v` output into a run summary. */
export function parseGoTest(
  exitCode: number,
  output: string,
): Pick<TestRun, 'status' | 'tests' | 'tail'> {
  const lines = output.split('\n')
  const seen = new Map<string, TestCase['result']>()
  for (const line of lines) {
    const m = CASE.exec(line)
    if (m?.[1] && m[2]) seen.set(m[2], m[1].toLowerCase() as TestCase['result'])
  }
  const tests = [...seen].map(([name, result]) => ({ name, result }))

  let status: TestRun['status'] = 'fail'
  if (exitCode === 0) status = 'pass'
  else if (/panic: test timed out/.test(output)) status = 'timeout'
  else if (/\[(build|setup) failed\]/.test(output) || (tests.length === 0 && !/^(FAIL|---)/m.test(output))) status = 'build'

  return { status, tests, tail: tailOf(output, 40) }
}

export function tailOf(text: string, n: number): string {
  return text.trimEnd().split('\n').slice(-n).join('\n')
}

export function countTodos(source: string): number {
  return source.split('TODO(you)').length - 1
}

/** One line for the status bar and the band. */
export function summary(run: TestRun | null): string {
  if (!run) return 'not run yet'
  if (run.status === 'running') return 'running…'
  if (run.status === 'build') return 'does not compile'
  if (run.status === 'timeout') return 'timed out (deadlock?)'
  const passed = run.tests.filter(t => t.result === 'pass').length
  const mark = run.status === 'pass' ? '✓' : '✗'
  return `${mark} ${passed}/${run.tests.length} tests`
}
