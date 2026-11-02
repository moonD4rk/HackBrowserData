# HackBrowserData RFCs

These RFCs are HackBrowserData's design record: the decisions that shape the program, why they were made, and what was rejected. They are short on purpose. The code, its doc comments, and the git history carry everything else.

## How an RFC is written

- It states a decision, the reason for it, and the alternatives rejected — in the words of the domain and the external contract.
- It names no code: no types, functions, fields, files, or tests. It gives no step-by-step procedures, and it keeps no list that grows with the code, such as browsers, versions, or fixes.
- The test for every sentence: would a refactor, a rename, a bug fix, or a new browser force an edit? If so, the sentence belongs in the code or its history, not here.
- An RFC changes only when a decision changes, and the pull request that edits one says which decision changed. The code never cites an RFC.

## Index

| RFC                                 | Decides                                                               |
| ----------------------------------- | --------------------------------------------------------------------- |
| [000](000-goals-and-constraints.md) | Why the redesign, its limits, and the frozen external contract        |
| [001](001-architecture.md)          | The one-way pipeline, outcomes, layering, and naming                  |
| [002](002-browser-support.md)       | Browsers as data, bases, and forks as rows                            |
| [003](003-secrets.md)               | Key scopes, lazy retrieval, and where secret handling lives           |
| [004](004-acquisition.md)           | Snapshot-first capture, and archive and restore as the same pipeline  |
| [005](005-output.md)                | The frozen output, deterministic order, and safe writing              |
| [006](006-cli.md)                   | The frozen command surface, validation first, and password input      |
| [007](007-quality.md)               | How tests, lint, Go 1.20, and dependency rules keep the design honest |
