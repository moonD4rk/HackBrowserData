# RFC-005: Output

## Frozen

Everything a user sees in the output — file names, formats, field names and their order, the browser and profile values — is part of the compatibility contract ([RFC-000](000-goals.md)). Field names on the wire are the snake_case of the record fields, enforced by lint, so the record fields are frozen with the output.

## Deterministic

The same snapshot always produces byte-identical files. Order is defined once per category, in the exporter rather than in each extractor, with a stable sort, so ties keep the browser's own order instead of one nobody chose.

## Safe writing

- **Private.** Output files are readable by the user only.
- **Streamed.** Records go to the files as each extraction finishes, so memory holds one extraction, not the whole run.
- **Zip last.** Packing into a zip replaces the loose files only once the zip is complete, so a failure loses nothing.
- **Only its own files.** A run packs and removes only the files it wrote.

## No redaction

What reaches export was asked for, and the default exports every category.

## Formats

Adding a format is a new encoder; nothing else in the pipeline changes.
