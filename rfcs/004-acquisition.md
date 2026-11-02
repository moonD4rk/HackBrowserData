# RFC-004: Acquisition

## Snapshot-first

No parser reads a live profile. Capture copies the files a plan needs into a temporary snapshot, and every parser reads only the copy. A running browser changes and locks its files; a copy gives stable bytes, keeps locked-file handling in one place, and lets every parser be tested with a fixture file instead of a browser.

## Lifetime

A dump captures one data directory at a time and removes its copies before the next, so copied secrets never outlive the extractions that read them, and disk use stays at one directory's worth. Temporary copies are private to the user and always removed — on success, on error, and on Ctrl-C. Capture only ever reads the browser's files.

## Failures stay local

A file that cannot be copied fails only the extractions that need it, and a file that vanished since planning makes them `absent`. The rest of the run goes on.

## Archive and restore

`archive` is "capture, then zip", and `restore` is "open or unzip, then extract": the same pipeline, not special modes.

- **The layout is the format.** An archive holds one directory per data directory, with the browser's own relative paths inside. v1.1.0 reads it, and restore rebuilds the directories and profiles from it, with no manifest.
- **A zip is only transport.** It is written to a temp file and renamed into place, so a failed archive never leaves a partial file; unzipping refuses any entry that would land outside its directory.
- **Only Chromium is archived.** Firefox and Safari keep no portable key.
