# RFC-002: Browser Support

## Decision

The supported browsers are a data table, not code. A row says what a browser is called, what it is built on — its base: Chromium, Firefox, or Safari — and where it keeps its data on each OS; the base decides how that data is read. A plain fork is a new row and nothing else.

## Why

v1.1.0 spread a browser's facts across several lists joined by key, and checked fork kinds across many files. One table is auditable at a glance, and base behavior is written three times in total, not once per browser.

## Rules

- **The name is the identity.** The base is the only classification. A release channel that keeps its own data, such as a beta, is its own row.
- **Exceptions stay inside their base.** A fork that stores data its own way gets an override inside its base, keyed by name, never a new classification.
- **One table serves every OS.** The browsers supported here are the rows with data on this OS, while restore may look up any row, so data taken on one OS restores on another.
- **Discovery reports; it never guesses.** Every matching data directory counts. One that exists but cannot be read is an error, not absence, and a profile counts only when it holds data some category reads.
- **Tests keep facts in step.** Facts that must agree across languages, such as the Windows payload's own browser table, are checked by tests, not by review.

## Rejected

- **One type per browser:** the same logic implemented again and again.
- **An enum of products:** every fork would become a code change.
- **Self-registering browsers:** hidden coupling at init time.
