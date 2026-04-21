<div align="center">
<img src="LOGO.png" alt="hack-browser-data logo" width="440px" />
</div> 

# HackBrowserData

[![Lint](https://github.com/moonD4rk/HackBrowserData/actions/workflows/lint.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/lint.yml) [![Build](https://github.com/moonD4rk/HackBrowserData/actions/workflows/build.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/build.yml) [![Release](https://github.com/moonD4rk/HackBrowserData/actions/workflows/release.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/release.yml) [![Tests](https://github.com/moonD4rk/HackBrowserData/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/test.yml) [![codecov](https://codecov.io/gh/moonD4rk/HackBrowserData/branch/main/graph/badge.svg?token=KWJCN38657)](https://codecov.io/gh/moonD4rk/HackBrowserData)

`HackBrowserData` is a command-line tool for decrypting and exporting browser data (passwords, history, cookies, bookmarks, credit cards, download history, localStorage, sessionStorage and extensions) from the browser. It supports the most popular Chromium-based browsers and Firefox on Windows, macOS and Linux, plus Safari on macOS.

> Disclaimer: This tool is only intended for security research. Users are responsible for all legal and related liabilities resulting from the use of this tool. The original author does not assume any legal responsibility.

## Supported Data Categories

| Category       | Chromium-based | Firefox | Safari |
|:---------------|:--------------:|:-------:|:------:|
| Password       |       ✅        |    ✅    |   ✅    |
| Cookie         |       ✅        |    ✅    |   ✅    |
| Bookmark       |       ✅        |    ✅    |   ✅    |
| History        |       ✅        |    ✅    |   ✅    |
| Download       |       ✅        |    ✅    |   ✅    |
| Credit Card    |       ✅        |    -    |   -    |
| Extension      |       ✅        |    ✅    |   ✅    |
| LocalStorage   |       ✅        |    ✅    |   ✅    |
| SessionStorage |       ✅        |    -    |   -    |

## Supported Browsers

> On macOS, some Chromium-based browsers **require a current user password** to decrypt.
>
> Password decryption may fail on macOS 26.4 or later.

| Browser        | Windows | macOS | Linux |
|:---------------|:-------:|:-----:|:-----:|
| Chrome         |   ✅²   |   ✅   |   ✅   |
| Chrome Beta    |   ✅²   |   ✅   |   ✅   |
| Chromium       |    ✅    |   ✅   |   ✅   |
| Edge           |   ✅²   |   ✅   |   ✅   |
| Brave          |   ✅²   |   ✅   |   ✅   |
| Opera          |    ✅    |   ✅   |   ✅   |
| OperaGX        |    ✅    |   ✅   |   -   |
| Vivaldi        |    ✅    |   ✅   |   ✅   |
| Yandex         |    ✅    |   ✅   |   -   |
| CocCoc         |   ✅²   |   ✅   |   -   |
| Arc            |    -    |   ✅   |   -   |
| DuckDuckGo     |    ✅    |   -   |   -   |
| QQ             |    ✅    |   -   |   -   |
| 360 ChromeX    |    ✅    |   -   |   -   |
| 360 Chrome     |    ✅    |   -   |   -   |
| DC Browser     |    ✅    |   -   |   -   |
| Sogou Explorer |    ✅    |   -   |   -   |
| Firefox        |    ✅    |   ✅   |   ✅   |
| Safari¹        |    -    |   ✅   |   -   |

> ¹ Safari requires Full Disk Access; enable it in System Settings → Privacy & Security → Full Disk Access if extraction returns empty results.
>
> ² On Windows, decrypting Chromium 127+ cookies (Chrome / Chrome Beta / Edge / Brave / CocCoc) requires the App-Bound Encryption payload built via `make build-windows` — see [Building from source](#building-from-source) below.

## Getting Started

### Install

Installation of `HackBrowserData` is dead-simple, just download [the release for your system](https://github.com/moonD4rk/HackBrowserData/releases) and run the binary.

> In some situations, this security tool will be treated as a virus by Windows Defender or other antivirus software and can not be executed. The code is all open source, you can modify and compile by yourself.

### Building from source

Requires `Go 1.20+`.

```bash
git clone https://github.com/moonD4rk/HackBrowserData
cd HackBrowserData
go build ./cmd/hack-browser-data/
```

#### Cross-platform build

```bash
# For Windows (standard build, no Chromium 127+ ABE cookie support)
GOOS=windows GOARCH=amd64 go build ./cmd/hack-browser-data/

# For Linux
GOOS=linux GOARCH=amd64 go build ./cmd/hack-browser-data/
```

#### Windows build with App-Bound Encryption (optional)

Chrome / Chrome Beta / Edge / Brave / CocCoc 127+ protect cookies with App-Bound Encryption. Decrypting those cookies requires a small C payload — [Zig](https://ziglang.org/) (0.13+) is the recommended C toolchain (the Makefile calls `zig cc`). MinGW-w64 `gcc` can also build the sources manually if you bypass `make payload`.

```bash
# 1. Install Zig
brew install zig                 # macOS
scoop install zig                # Windows (scoop)
# or download from https://ziglang.org/download/

# 2. Build the payload (outputs crypto/windows/payload/abe_extractor_amd64.bin)
make payload

# 3. Build hack-browser-data.exe with the ABE payload embedded
make build-windows
```

The resulting `hack-browser-data.exe` includes full ABE cookie decryption on Chromium 127+.

## Usage

```
$ hack-browser-data -h
hack-browser-data decrypts and exports browser data from Chromium-based
browsers and Firefox on Windows, macOS, and Linux.

GitHub: https://github.com/moonD4rk/HackBrowserData

Usage:
  hack-browser-data [flags]
  hack-browser-data [command]

Available Commands:
  dump        Extract and decrypt browser data (default command)
  help        Help about any command
  list        List detected browsers and profiles
  version     Print version information

Flags:
  -b, --browser string        target browser: all|chrome|firefox|edge|... (default "all")
  -c, --category string       data categories (comma-separated): all|password,cookie,... (default "all")
  -d, --dir string            output directory (default "results")
  -f, --format string         output format: csv|json|cookie-editor (default "csv")
  -h, --help                  help for hack-browser-data
      --keychain-pw string    macOS keychain password
  -p, --profile-path string   custom profile dir path, get with chrome://version
  -v, --verbose               enable debug logging
      --zip                   compress output to zip

Use "hack-browser-data [command] --help" for more information about a command.
```

### `dump` - Extract and decrypt browser data (default)

Running `hack-browser-data` without a subcommand defaults to `dump`.

| Flag             | Short | Default   | Description                                                                                                                                |
|------------------|-------|-----------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `--browser`      | `-b`  | `all`     | Target browser (all\|chrome\|firefox\|edge\|...)                                                                                           |
| `--category`     | `-c`  | `all`     | Data categories, comma-separated (all\|password\|cookie\|bookmark\|history\|download\|creditcard\|extension\|localstorage\|sessionstorage) |
| `--format`       | `-f`  | `csv`     | Output format (csv\|json\|cookie-editor)                                                                                                   |
| `--dir`          | `-d`  | `results` | Output directory                                                                                                                           |
| `--profile-path` | `-p`  |           | Custom profile dir path, get with chrome://version                                                                                         |
| `--keychain-pw`  |       |           | macOS keychain password                                                                                                                    |
| `--zip`          |       | `false`   | Compress output to zip                                                                                                                     |

### `list` - List detected browsers and profiles

| Flag       | Default | Description                    |
|------------|---------|--------------------------------|
| `--detail` | `false` | Show per-category entry counts |

### `version` - Print version information

```bash
hack-browser-data version
```

### Global flags

| Flag        | Short | Description          |
|-------------|-------|----------------------|
| `--verbose` | `-v`  | Enable debug logging |

### Examples

```bash
# Extract all data from all browsers (default)
hack-browser-data

# Extract specific browser and categories
hack-browser-data dump -b chrome -c password,cookie

# Export in JSON format to a custom directory
hack-browser-data dump -b chrome -f json -d output

# Export cookies in CookieEditor format
hack-browser-data dump -f cookie-editor

# Compress output to zip
hack-browser-data dump --zip

# List detected browsers and profiles
hack-browser-data list

# List with per-category entry counts
hack-browser-data list --detail

# Use custom profile path
hack-browser-data dump -b chrome -p "/path/to/User Data/Default"
```

## Contributing

We welcome and appreciate any contributions made by the community (GitHub issues/pull requests, email feedback, etc.).

Please see the [Contribution Guide](CONTRIBUTING.md) before contributing.


## Contributors

<!-- readme: collaborators,contributors -start -->
<table>
	<tbody>
		<tr>
            <td align="center">
                <a href="https://github.com/moonD4rk">
                    <img src="https://avatars.githubusercontent.com/u/24284231?v=4" width="100;" alt="moonD4rk"/>
                    <br />
                    <sub><b>Roger</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/Aquilao">
                    <img src="https://avatars.githubusercontent.com/u/25531497?v=4" width="100;" alt="Aquilao"/>
                    <br />
                    <sub><b>Aquilao Official</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/uinfziuna8n">
                    <img src="https://avatars.githubusercontent.com/u/43719451?v=4" width="100;" alt="uinfziuna8n"/>
                    <br />
                    <sub><b>uinfziuna8n</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/VMpc">
                    <img src="https://avatars.githubusercontent.com/u/50967051?v=4" width="100;" alt="VMpc"/>
                    <br />
                    <sub><b>Cyrus</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/stevenlele">
                    <img src="https://avatars.githubusercontent.com/u/15964380?v=4" width="100;" alt="stevenlele"/>
                    <br />
                    <sub><b>stevenlele</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/camandel">
                    <img src="https://avatars.githubusercontent.com/u/5462153?v=4" width="100;" alt="camandel"/>
                    <br />
                    <sub><b>Carlo Mandelli</b></sub>
                </a>
            </td>
		</tr>
		<tr>
            <td align="center">
                <a href="https://github.com/slimwang">
                    <img src="https://avatars.githubusercontent.com/u/14370794?v=4" width="100;" alt="slimwang"/>
                    <br />
                    <sub><b>slimwang</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/Amir-78">
                    <img src="https://avatars.githubusercontent.com/u/68391526?v=4" width="100;" alt="Amir-78"/>
                    <br />
                    <sub><b>Amir.</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/a-urth">
                    <img src="https://avatars.githubusercontent.com/u/3456803?v=4" width="100;" alt="a-urth"/>
                    <br />
                    <sub><b>a-urth</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/dexhek">
                    <img src="https://avatars.githubusercontent.com/u/39654918?v=4" width="100;" alt="dexhek"/>
                    <br />
                    <sub><b>Ciprian Conache</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/SantiiRepair">
                    <img src="https://avatars.githubusercontent.com/u/94815926?v=4" width="100;" alt="SantiiRepair"/>
                    <br />
                    <sub><b>Santiago Ramirez</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/BeichenDream">
                    <img src="https://avatars.githubusercontent.com/u/43266206?v=4" width="100;" alt="BeichenDream"/>
                    <br />
                    <sub><b>beichen</b></sub>
                </a>
            </td>
		</tr>
		<tr>
            <td align="center">
                <a href="https://github.com/testwill">
                    <img src="https://avatars.githubusercontent.com/u/8717479?v=4" width="100;" alt="testwill"/>
                    <br />
                    <sub><b>guoguangwu</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/zhe6652">
                    <img src="https://avatars.githubusercontent.com/u/24725680?v=4" width="100;" alt="zhe6652"/>
                    <br />
                    <sub><b>zhe6652</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/lc6464">
                    <img src="https://avatars.githubusercontent.com/u/64722907?v=4" width="100;" alt="lc6464"/>
                    <br />
                    <sub><b>LC</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/mirefly">
                    <img src="https://avatars.githubusercontent.com/u/4984681?v=4" width="100;" alt="mirefly"/>
                    <br />
                    <sub><b>mirefly</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/slark-yuxj">
                    <img src="https://avatars.githubusercontent.com/u/95608083?v=4" width="100;" alt="slark-yuxj"/>
                    <br />
                    <sub><b>YuXJ</b></sub>
                </a>
            </td>
            <td align="center">
                <a href="https://github.com/ac0d3r">
                    <img src="https://avatars.githubusercontent.com/u/26270009?v=4" width="100;" alt="ac0d3r"/>
                    <br />
                    <sub><b>zznQ</b></sub>
                </a>
            </td>
		</tr>
	<tbody>
</table>
<!-- readme: collaborators,contributors -end -->

## Stargazers over time
[![Star History Chart](https://api.star-history.com/svg?repos=moond4rk/hackbrowserdata&type=Date)](https://github.com/moond4rk/HackBrowserData)


## 404StarLink 2.0 - Galaxy
`HackBrowserData` is a part of 404Team [StarLink-Galaxy](https://github.com/knownsec/404StarLink2.0-Galaxy), if you have any questions about `HackBrowserData` or want to find a partner to communicate with, please refer to the [Starlink group](https://github.com/knownsec/404StarLink2.0-Galaxy#community).
<a href="https://github.com/knownsec/404StarLink2.0-Galaxy" target="_blank"><img src="https://raw.githubusercontent.com/knownsec/404StarLink-Project/master/logo.png" align="middle"/></a>

##  JetBrains OS licenses
`HackBrowserData` had been being developed with `GoLand` IDE under the **free JetBrains Open Source license(s)** granted by JetBrains s.r.o., hence I would like to express my thanks here.

<a href="https://www.jetbrains.com/?from=HackBrowserData" target="_blank"><img src="https://raw.githubusercontent.com/moonD4rk/staticfiles/master/picture/jetbrains-variant-4.png" width="256" align="middle"/></a>
