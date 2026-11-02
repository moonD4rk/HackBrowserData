# RFC-000: Goals & Constraints

HackBrowserData is an MIT-licensed, cross-platform security-research tool. Its structure serves the goals below, within the constraints that follow.

## Goals

1. **A reference-grade architecture.** A newcomer can read the pipeline top to bottom and understand the whole run.
2. **A good operator experience.** Usage errors fail before anything is touched, every requested browser and data type gets a reported outcome, secrets are fetched only when the output needs them, and output is deterministic.
3. **Compatibility.** No command line, script, or output parser has to change.
4. **Quality that lasts.** Adding a browser, a data type, or an output format is one small, tested change.

## Non-goals

- **No new capability.** No targets, bypasses, or techniques beyond those already published.
- **No interface changes** beyond additive, opt-in ones.
- **No network.** The tool stays fully offline: no telemetry, no update checks.
- **No GUI and no library API.** The CLI is the only way in.

## Hard constraints

- **Go 1.20,** for Windows 7: no newer language or standard-library features, and no dependency that needs them.
- **MIT,** on Windows, macOS, and Linux, fully offline.

## The compatibility contract

The external interface is a contract with existing users, and tests pin it:

- **Commands and flags** keep their names, shorthands, types, and defaults.
- **Output** keeps one file per non-empty category, its formats, its field names and order, and the browser and profile values.
- **`keys.json` and archives** stay readable across releases, by older and newer versions alike.
- **Exit status** is `0` when the run completes, however partial its results, and `1` when it cannot start or finish.

Log lines and the summary on stderr are not part of the contract. Output changes only to fix output that was wrong, and each such fix is announced in the release notes.

## Guiding principles

- One explicit pipeline, with data flowing one way ([RFC-001](001-architecture.md)).
- Snapshot-first: no parser reads a live profile ([RFC-004](004-snapshots.md)).
- Lazy secrets: nothing secret is touched until a requested output needs it ([RFC-003](003-secrets.md)).
- An outcome for every extraction, and no failure that aborts the rest ([RFC-001](001-architecture.md)).
- Everything below the command is internal: the CLI is the only compatibility obligation.
