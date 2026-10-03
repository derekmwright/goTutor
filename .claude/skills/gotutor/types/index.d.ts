export type Lesson = {
  /** The lesson folder, relative to the project root (`lessons/01-fan-out`). */
  dir: string
  title: string
  objectives: string[]
  /** Files the student edits, relative to `dir`; opened in the editor. */
  files: string[]
  startedAt: number
}

export type TestCase = { name: string; result: 'pass' | 'fail' | 'skip' }

export type TestRun = {
  status: 'running' | 'pass' | 'fail' | 'build' | 'timeout'
  tests: TestCase[]
  /** The last lines of `go test` output. */
  tail: string
  /** `TODO(you)` markers still in the student's files. */
  todos: number
  at: number
}

declare module 'claude-code' {
  interface PluginState {
    gotutor: {
      lesson: Lesson | null
      run: TestRun | null
    }
  }
}
