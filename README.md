<div align="center">
<img src="LOGO.png" alt="hack-browser-data logo" width="440px" />
</div> 

# HackBrowserData

[![Lint](https://github.com/moonD4rk/HackBrowserData/actions/workflows/lint.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/lint.yml) [![Build](https://github.com/moonD4rk/HackBrowserData/actions/workflows/build.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/build.yml) [![Release](https://github.com/moonD4rk/HackBrowserData/actions/workflows/release.yml/badge.svg)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/release.yml) [![Tests](https://github.com/moonD4rk/HackBrowserData/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/moonD4rk/HackBrowserData/actions/workflows/test.yml) [![Coverage Status](https://coveralls.io/repos/github/moonD4rk/HackBrowserData/badge.svg)](https://coveralls.io/github/moonD4rk/HackBrowserData)

`HackBrowserData` is a command-line tool for decrypting and exporting browser data (passwords, history, cookies, bookmarks, credit cards, download history, localStorage and extensions) from the browser. It supports the most popular browsers on the market and runs on Windows, macOS and Linux.

> Disclaimer: This tool is only intended for security research. Users are responsible for all legal and related liabilities resulting from the use of this tool. The original author does not assume any legal responsibility.

## Supported Browser

### Windows
| Browser            | Password | Cookie | Bookmark | History |
|:-------------------|:--------:|:------:|:--------:|:-------:|
| Google Chrome      |    ✅     |   ✅    |    ✅     |    ✅    |
| Google Chrome Beta |    ✅     |   ✅    |    ✅     |    ✅    |
| Chromium           |    ✅     |   ✅    |    ✅     |    ✅    |
| Microsoft Edge     |    ✅     |   ✅    |    ✅     |    ✅    |
| 360 Speed          |    ✅     |   ✅    |    ✅     |    ✅    |
| QQ                 |    ✅     |   ✅    |    ✅     |    ✅    |
| Brave              |    ✅     |   ✅    |    ✅     |    ✅    |
| Opera              |    ✅     |   ✅    |    ✅     |    ✅    |
| OperaGX            |    ✅     |   ✅    |    ✅     |    ✅    |
| Vivaldi            |    ✅     |   ✅    |    ✅     |    ✅    |
| Yandex             |    ✅     |   ✅    |    ✅     |    ✅    |
| CocCoc             |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox            |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Beta       |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Dev        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox ESR        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Nightly    |    ✅     |   ✅    |    ✅     |    ✅    |
| Internet Explorer  |    ❌     |   ❌    |    ❌     |    ❌    |


### MacOS

Based on Apple's security policy, some browsers **require a current user password** to decrypt.

| Browser            | Password | Cookie | Bookmark | History |
|:-------------------|:--------:|:------:|:--------:|:-------:|
| Google Chrome      |    ✅     |   ✅    |    ✅     |    ✅    |
| Google Chrome Beta |    ✅     |   ✅    |    ✅     |    ✅    |
| Chromium           |    ✅     |   ✅    |    ✅     |    ✅    |
| Microsoft Edge     |    ✅     |   ✅    |    ✅     |    ✅    |
| Brave              |    ✅     |   ✅    |    ✅     |    ✅    |
| Opera              |    ✅     |   ✅    |    ✅     |    ✅    |
| OperaGX            |    ✅     |   ✅    |    ✅     |    ✅    |
| Vivaldi            |    ✅     |   ✅    |    ✅     |    ✅    |
| CocCoc             |    ✅     |   ✅    |    ✅     |    ✅    |
| Yandex             |    ✅     |   ✅    |    ✅     |    ✅    |
| Arc                |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox            |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Beta       |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Dev        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox ESR        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Nightly    |    ✅     |   ✅    |    ✅     |    ✅    |
| Safari             |    ❌     |   ❌    |    ❌     |    ❌    |

### Linux

| Browser            | Password | Cookie | Bookmark | History |
|:-------------------|:--------:|:------:|:--------:|:-------:|
| Google Chrome      |    ✅     |   ✅    |    ✅     |    ✅    |
| Google Chrome Beta |    ✅     |   ✅    |    ✅     |    ✅    |
| Chromium           |    ✅     |   ✅    |    ✅     |    ✅    |
| Microsoft Edge Dev |    ✅     |   ✅    |    ✅     |    ✅    |
| Brave              |    ✅     |   ✅    |    ✅     |    ✅    |
| Opera              |    ✅     |   ✅    |    ✅     |    ✅    |
| Vivaldi            |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox            |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Beta       |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Dev        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox ESR        |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox Nightly    |    ✅     |   ✅    |    ✅     |    ✅    |


## Getting started

### Install

Installation of `HackBrowserData` is dead-simple, just download [the release for your system](https://github.com/moonD4rk/HackBrowserData/releases) and run the binary.

> In some situations, this security tool will be treated as a virus by Windows Defender or other antivirus software and can not be executed. The code is all open source, you can modify and compile by yourself.

### Building from source

only support `go 1.20+` with go generics.

```bash
$ git clone https://github.com/moonD4rk/HackBrowserData

$ cd HackBrowserData/cmd/hack-browser-data

$ go build
```

### Cross compile

Here's an example of use `macOS` building for `Windows` and `Linux`

#### For Windows

```shell
GOOS=windows GOARCH=amd64 go build
```

#### For Linux

````shell
GOOS=linux GOARCH=amd64 go build
````

### Run

You can double-click to run, or use command line.

```powershell
PS C:\Users\moond4rk\Desktop> .\hack-browser-data.exe -h
NAME:
   hack-browser-data - Export passwords|bookmarks|cookies|history|credit cards|download history|localStorage|extensions from browser
USAGE:
   [hack-browser-data -b chrome -f json --dir results --zip]
   Export all browsing data (passwords/cookies/history/bookmarks) from browser
   Github Link: https://github.com/moonD4rk/HackBrowserData
VERSION:
   0.4.6

GLOBAL OPTIONS:
   --verbose, --vv                   verbose (default: false)
   --compress, --zip                 compress result to zip (default: false)
   --browser value, -b value         available browsers: all|360|brave|chrome|chrome-beta|chromium|coccoc|dc|edge|firefox|opera|opera-gx|qq|sogou|vivaldi|yandex (default: "all")
   --results-dir value, --dir value  export dir (default: "results")
   --format value, -f value          output format: csv|json (default: "csv")
   --profile-path value, -p value    custom profile dir path, get with chrome://version
   --full-export, --full             is export full browsing data (default: true)
   --help, -h                        show help
   --version, -v                     print the version

```

For example, the following is an automatic scan of the browser on the current computer, outputting the decryption results in `JSON` format and compressing as `zip`.

```powershell
PS C:\Users\moond4rk\Desktop> .\hack-browser-data.exe -b all -f json --dir results --zip

PS C:\Users\moond4rk\Desktop> ls -l .\results\
    Directory: C:\Users\moond4rk\Desktop\results
    
Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
-a----         7/15/2024  10:55 PM          44982 results.zip
```


### Run with custom browser profile folder

If you want to export data from a custom browser profile folder, you can use the `-p` parameter to specify the path of the browser profile folder. PS: use double quotes to wrap the path.
```powershell
PS C:\Users\moond4rk\Desktop> .\hack-browser-data.exe -b chrome -p "C:\Users\User\AppData\Local\Microsoft\Edge\User Data\Default"

[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_creditcard.csv success  
[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_bookmark.csv success  
[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_cookie.csv success  
[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_history.csv success  
[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_download.csv success  
[NOTICE] [browsingdata.go:59,Output] output to file results/chrome_password.csv success  
```

## Contributing

We welcome and appreciate any contributions made by the community (GitHub issues/pull requests, email feedback, etc.).

Please see the [Contribution Guide](CONTRIBUTING.md) before contributing.


## Contributors

![](/CONTRIBUTORS.svg)

## Stargazers over time
[![Star History Chart](https://api.star-history.com/svg?repos=moond4rk/hackbrowserdata&type=Date)](https://github.com/moond4rk/HackBrowserData)


## 404StarLink 2.0 - Galaxy
`HackBrowserData` is a part of 404Team [StarLink-Galaxy](https://github.com/knownsec/404StarLink2.0-Galaxy), if you have any questions about `HackBrowserData` or want to find a partner to communicate with，please refer to the [Starlink group](https://github.com/knownsec/404StarLink2.0-Galaxy#community).
<a href="https://github.com/knownsec/404StarLink2.0-Galaxy" target="_blank"><img src="https://raw.githubusercontent.com/knownsec/404StarLink-Project/master/logo.png" align="middle"/></a>

##  JetBrains OS licenses
``HackBrowserData`` had been being developed with `GoLand` IDE under the **free JetBrains Open Source license(s)** granted by JetBrains s.r.o., hence I would like to express my thanks here.

<a href="https://www.jetbrains.com/?from=HackBrowserData" target="_blank"><img src="https://raw.githubusercontent.com/moonD4rk/staticfiles/master/picture/jetbrains-variant-4.png" width="256" align="middle"/></a>

