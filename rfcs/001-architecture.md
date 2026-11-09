# RFC-001: Architecture

## The pipeline

A run is a fixed, one-way sequence of stages, and no stage reaches back:

1. **Validate** — turn the flags into options, and reject bad input before anything is touched.
2. **Discover** — find each browser's data directories and their profiles.
3. **Plan** — cross one directory's profiles with the requested categories into extractions, record why each skipped one cannot run, and list the files to copy.
4. **Capture** — copy one directory's planned files into a temporary snapshot.
5. **Extract** — parse the snapshot into records, fetching a secret only when a record needs it.
6. **Export** — append each extraction's records to the output files.
7. **Report** — summarize every outcome.

Plan, Capture, Extract, and Export run once per data directory: a directory is planned right before its files are copied, and at most one directory's copies are on disk at a time. Every command is a cut of this pipeline: `list` stops after discovery, `list --detail` counts records instead of exporting them, `archive` captures and zips, `dumpkeys` retrieves keys only, and `restore` extracts from a snapshot it is given.

**Why.** When each browser runs the whole flow itself, every concern is solved several times, and bugs hide where nothing else checks them. Named stages implement each concern once; a browser contributes only where its files are and how to read them.

## Outcomes

Every extraction — one category from one profile of one data directory — ends in exactly one outcome: `ok`, `partial` (records came out, with record-level problems), `absent` (nothing on disk), `unsupported` (the browser cannot produce it), or `error`. A failure that concerns a whole directory is reported once, on its own. No failure aborts the rest of the run, and outcomes never change the exit status. `partial` is first-class, because a few unreadable rows are normal in the field and must not look like total failure.

## Layers

- The command layer wires flags into the pipeline and holds what lives as long as the process: signals, the console window, the data a restore reads.
- The pipeline orchestrates, and it is the only place that knows every browser base.
- Each browser base knows its own discovery and file formats.
- Shared services sit below them: secret retrieval, pure crypto primitives, snapshot capture, and export.
- Platform code sits at the bottom, behind build tags.

Dependencies point down only. Interfaces live with their consumer, nothing registers itself at init time, and a new dependency edge is a reviewed decision. All code below the command is internal: there is no library API to keep compatible, so the code can be reshaped freely.

## Naming

- A name is judged at its call site, qualified by its package, and never repeats the package's name.
- A name says what a thing is, not its role or its shape, and one concept has one word.
- Names are spelled out; only Go's universal idioms stay short, and lint enforces it.
- The domain's words: a **browser** is a row of the browser table; its **base** is what it is built on (Chromium, Firefox, or Safari); a **user data dir** is a browser's data directory, where an installation would be the program; and a **profile**, an **extraction**, a **snapshot**, a **record**, and a **category** mean what they say.

**Rejected:** "engine" for the base, which means the renderer; "origin" or "owner" for where a record came from, which are web and access-control terms; and "job" for an extraction, which suggests a worker pool that does not exist.
