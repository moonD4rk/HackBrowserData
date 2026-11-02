# RFC-007: Quality

## Tests

- A test needs a fixture file, never a running browser or a mocked database: tests build real files in a temporary directory.
- Goldens pin the output, and the same fixture always produces the same bytes.
- Compatibility tests pin the frozen surface: every command's flags, the output, `keys.json`, and archives.
- Every fixed bug gets a regression test.
- Coverage is a signal, not a target.

## Lint and comments

Lint enforces what review would otherwise argue about: spelled-out names, the form of comments, wire names, and line length. A comment explains why, never what, and the code cites no RFC.

## Go 1.20

The module stays on Go 1.20, and CI pins that toolchain instead of following the module file, so a dependency that needs newer Go fails CI instead of passing silently. Go 1.20's own traps are covered by tests: loop variables are shared across iterations, and newer standard helpers are not there.

## Dependencies

A dependency earns its place only when writing it ourselves would cost more than auditing it. A release links permissively licensed modules only, and a module whose newer versions would raise the Go requirement is pinned below them.
