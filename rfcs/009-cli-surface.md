# RFC-009: CLI Surface

The command surface is v1.1.0's, unchanged: the operator's view is the spec, and the architecture is derived backward from it. This RFC records that surface, the one additive input the rewrite introduces, and the behavior fixes behind unchanged flags.

## Commands (unchanged)

| Command    | Does                                                                                                                             | Flags                                                                                                     |
| ---------- | -------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| `dump`     | Discover → extract → export. The default command.                                                                                | `-b` `-c` `-f` `-d` `-p` `--keychain-pw` `--zip`                                                          |
| `dumpkeys` | Write Chromium master keys as `keys.json` (stdout by default; `-o` writes the file with mode `0600`).                            | `-b` `-o` `--keychain-pw`                                                                                 |
| `archive`  | Zip the decryption-relevant Chromium files for another host.                                                                     | `-b` `-c` `-o` (default `browser-data.zip`)                                                               |
| `restore`  | Decrypt an archive or copied data with a `keys.json`.                                                                            | `--keys` (required, `-` = stdin), `--data-dir` \| `--data-zip` (exactly one), `-b` `-c` `-f` `-d` `--zip` |
| `list`     | Show detected browsers and profiles (columns Browser, Profile, Path); `--detail` adds a count column per category.               | `--detail`                                                                                                |
| `version`  | Version, commit (first 8 characters), and build date, set by `-ldflags`; a build without them reads the module's VCS build info. | —                                                                                                         |

`-v, --verbose` is global. `hack-browser-data` with no subcommand runs `dump`, and `dump`'s flags are mirrored onto the root, so `hack-browser-data -b chrome` keeps working (this also covers the Windows double-click case).

Every command is a different cut of one pipeline ([RFC-001](001-architecture-and-pipeline.md)): `list` stops after Plan; `list --detail` runs Extract with no secrets and counts what the `Collector` receives instead of exporting it; `archive` captures every user data dir into one snapshot directory and zips it; `dumpkeys` captures each user data dir's `Local State` and runs only key retrieval; and `restore` extracts from an existing snapshot directory.

## Core flags of `dump` (unchanged)

| Flag                 | Default   | Meaning                                                                                                                                          |
| -------------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `-b, --browser`      | `all`     | `all` or one ID from `browser.Supported()`, this OS's rows                                                                                       |
| `-c, --category`     | `all`     | comma-separated categories: `password`, `cookie`, `bookmark`, `history`, `download`, `creditcard`, `extension`, `localstorage`, `sessionstorage` |
| `-f, --format`       | `json`    | `csv`, `json`, or `cookie-editor`                                                                                                                |
| `-d, --dir`          | `results` | output directory                                                                                                                                 |
| `-p, --profile-path` | —         | custom data location for one browser (the path `chrome://version` shows, or a user data dir)                                                     |
| `--keychain-pw`      | —         | macOS login password (kept for compatibility; visible in `ps` and shell history)                                                                 |
| `--zip`              | `false`   | pack the output files into a zip                                                                                                                 |

The flag names stay, and the Go names follow them: `-c/--category` selects `category.Category` values ([RFC-004](004-naming-decisions.md)). What is frozen is each flag's name, shorthand, type, and default; usage strings may be clarified — `--keychain-pw` gains a note that it is visible in `ps` — and the root command's description gains Safari, which today it omits. `-b`'s usage string lists the running OS's IDs, as today.

## Validation happens first — the ordering bug, designed out

Today `-c` and `-f` are checked only after discovery and after the keychain prompt (`cmd/hack-browser-data/dump.go`, `extract.go`), so a typo in a format wastes a prompt and a discovery pass. In the rewrite, **Validate is stage 1** ([RFC-001](001-architecture-and-pipeline.md)): an unknown format, category, or browser ID, a `-d` that cannot be created, `--data-dir` together with `--data-zip`, or an unreadable `--keys` all fail before any browser is touched and before any secret is read. These exit `1`, as today; only the timing changes, and a typo in `-c` or `-f` now fails even where no browser is found, where today the run stops first at "no browsers found" with `0`. An unknown `-b`, including a browser only another OS has, is the same case: today it finds nothing and exits `0`; like a missing `-p`, it now exits `1`, because such a command line never did anything. The rule: _nothing with a side effect runs until the request is valid._

## `-p, --profile-path` — same meaning, the long-standing bugs fixed

The flag keeps its meaning and its name: a custom data location for one browser. It has never worked well, and the rewrite fixes every known fault behind it without changing the flag.

- **Validated first.** Today a non-existent `-p` is read as an empty directory, so the run just says "no browsers found" and exits `0`. Validate now checks that the path exists and is a directory, and exits `1` with a clear message if not ([RFC-001](001-architecture-and-pipeline.md)).
- **With `-b all`.** Today `-p` is silently ignored when `-b` is `all` (`browser/browser.go`, `discoverFromConfigs`), because the override is guarded by `name != "all"`. Validate now logs a warning that `-p` needs a specific `-b`, then ignores it as before — the run itself is unchanged. Rejecting the combination would turn today's exit status `0` into `1`, which the contract rules out.
- **Pointing at a Chromium profile directory.** The help text asks for the path `chrome://version` shows, which is a profile directory (`…/User Data/Default`). Today that directory becomes the whole user data dir, so the `Local State` one level up is never found and, on Windows, the DPAPI and ABE keys cannot be derived — `-p` effectively cannot decrypt anything there. The Chromium `Discover` now looks at what it was given: a directory holding `Preferences` but no `Local State` is a profile, so its parent is the user data dir and only that profile is selected; a directory holding profile subdirectories, or a flat layout with its own `Local State`, is a user data dir, as before. Either way the keys resolve.
- **Firefox selects only the named profile.** Today the Firefox override takes the parent of the given directory as the Profiles root and then exports _every_ profile under it, not the one asked for. `Discover` now selects only the profile the path names. This narrows today's output for `-b firefox -p <profile>` and is called out as an expected diff in the parity suite ([RFC-011](011-testing-and-quality.md)).
- **Safari keeps its meaning.** `-b safari -p` names a Safari directory to read instead of `~/Library/Safari`, as it always has: its parent stands in for `~/Library`, so the named profiles in the container beside it are found too, and passwords still come from the login keychain.

Selecting profiles by name across browsers is DEFERRED; if added, it gets its own flag rather than changing `-p`.

## Keychain password input (DECIDED)

`--keychain-pw` stays, because scripts use it. One safer input is added beside it:

- `--keychain-pw-stdin` (on `dump` and `dumpkeys`): read the password from the first line of standard input, for piping from a secret manager.

Precedence, highest first:

1. An explicit flag — `--keychain-pw` or `--keychain-pw-stdin` (mutually exclusive).
2. An interactive no-echo prompt (`golang.org/x/term`), only when stdin is a terminal.
3. None: keychain-protected data is exported as metadata only, as today.

Explicit inputs win because they are the user's stated intent; the prompt is the fallback, never the first step. Validate turns the chosen input into the one `keychain.PasswordFunc` the run's login keychain is built with ([RFC-003](003-core-types.md)); no other code sees the password. The function runs when the keychain is first used, so `dump -c history` never asks ([RFC-006](006-key-retrieval-and-crypto.md)). Opening the keychain is the check: a missing or wrong password leaves it locked, keychain-protected data is exported as metadata only, as today, and the reason is reported once. The help text says what each input exposes: `--keychain-pw` is visible in `ps` and shell history; stdin from a secret manager leaves no trace.

There is no environment variable. Every child process the tool starts would inherit it — the macOS key retrieval runs the `security` command — and the inline form `VAR=… hack-browser-data` lands in shell history just like the flag. A second non-interactive input would add only a precedence level and its tests.

## Reporting & exit status

After a run, the reporter prints a per-browser, per-profile, per-category summary with each extraction's outcome and count on stderr, after today's "Exported to …" lines. The summary is additive and not part of the contract. `-v` adds record-level debug lines, as today. The exit status is today's: `0` when the run completes, whatever the per-extraction outcomes, and `1` when it cannot start or finish ([RFC-001](001-architecture-and-pipeline.md)).

## `restore` specifics (unchanged)

`restore`'s universe is its `keys.json`, not the local table: `-b` filters the dump's vaults, and a vault's `Def` is resolved with `browser.Lookup`, which searches the whole table whatever the host OS ([RFC-005](005-browser-definitions.md)), so a Windows-only fork restores on macOS. Exactly one of `--data-dir` and `--data-zip` is required. A zip is extracted into a temporary snapshot directory (`snapshot.Unzip`), rejecting entries that escape it, and removed when the run ends; a `--data-dir` is opened in place (`snapshot.Open`) and never removed ([RFC-007](007-snapshot-and-acquisition.md)). The data has two layouts, as today: `<browser-key>/` subdirectories (the archive layout), from which every selected vault restores, or one browser's hand-copied `User Data`, which is unambiguous only when `-b` selects a single vault.

## DEFERRED

- Profile selection by name.
- More than one `-f` value per run.
- Machine-readable exit codes for usage errors and partial runs (opt-in only, if ever).
- A `--quiet` switch for the summary.
