# HackBrowserData

![CI](https://github.com/moonD4rk/HackBrowserData/workflows/CI/badge.svg?branch=master) ![Build Hack-Browser-Data Release](https://github.com/moonD4rk/HackBrowserData/workflows/Build%20Hack-Browser-Data%20Release/badge.svg)

[中文说明](https://github.com/moonD4rk/HackBrowserData/blob/master/README_ZH.md) 

hack-browser-data is an open-source tool that could help you decrypt data ( passwords|bookmarks|cookies|history ) from the browser. It supports the most popular browsers on the market and runs on Windows, macOS and Linux.

> Statement: This tool is limited to security research only, and the user assumes all legal and related responsibilities arising from its use! The author assumes no legal responsibility!

### Supported Browser

#### Windows
| Browser                             | Password | Cookie | Bookmark | History |
| :---------------------------------- | :------: | :----: | :------: | :-----: |
| Google Chrome |    ✅    |   ✅   |    ✅    |    ✅    |
| Firefox |    ✅    |   ✅   |    ✅    |    ✅    |
| Microsoft Edge |    ✅    |   ✅   |    ✅    |    ✅    |
| 360 Speed Browser |    ✅    |   ✅   |    ✅    |    ✅    |
| QQ Browser |    ✅    |   ✅   |    ✅    |    ✅    |
| Brave Browser |    ✅    |   ✅   |    ✅    |    ✅    |
| Internet Explorer |    ❌    |   ❌   |    ❌    |    ❌    |

#### MacOS

Based on Apple's security policy, some browsers **require a current user password** to decrypt.

| Browser                             | Password | Cookie | Bookmark | History |
| :---------------------------------- | :------: | :----: | :------: | :-----: |
| Google Chrome |    ✅    |   ✅   |    ✅    |    ✅    |
| Firefox |    ✅    |   ✅   |    ✅    |    ✅    |
| Microsoft Edge |    ✅    |   ✅   |    ✅    |    ✅    |
| Brave Browser |    ✅    |   ✅   |    ✅    |    ✅    |
| Safari |    ❌    |   ❌   |    ❌    |    ❌    |

#### Linux

| Browser                             | Password | Cookie | Bookmark | History |
| :---------------------------------- | :------: | :----: | :------: | :-----: |
| Firefox |    ✅    |   ✅   |    ✅    |    ✅    |
| Google Chrome |    ✅    |   ✅   |    ✅    |    ✅    |
| Microsoft Edge Dev |    ✅    |   ✅   |    ✅    |    ✅    |
| Brave Browser |    ✅    |   ✅   |    ✅    |    ✅    |


### Install

Installation of hack-browser-data is dead-simple, just download [the release for your system](https://github.com/moonD4rk/HackBrowserData/releases) and run the binary.

> In some situations, this security tool will be treated as a virus by Windows Defender or other antivirus software and can not be executed, after version 0.2.6 will use UPX try to simply bypass, then no longer with antivirus software to do unnecessary security confrontations.The code is all open source, you can modify and compile by yourself.

#### Building from source

support `go 1.11+`

```bash
git clone https://github.com/moonD4rk/HackBrowserData

cd HackBrowserData

go get -v -t -d ./...

go build
```

##### Cross compile

Need install target OS's `gcc` library, here's an example of use `Mac` building for `Windows` and `Linux`

**Windows**

```shell
brew install mingw-w64

CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="x86_64-w64-mingw32-gcc" go build
```

**Linux**

````shell
brew install FiloSottile/musl-cross/musl-cross

CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ GOARCH=amd64 GOOS=linux CGO_ENABLED=1 go build -ldflags "-linkmode external -extldflags -static"
````

#### Run

You can double-click to run, or use command line.

```
PS C:\test> .\hack-browser-data.exe -h
NAME:
   hack-browser-data - Export passwords/cookies/history/bookmarks from browser
USAGE:
   [hack-browser-data -b chrome -f json -dir results -cc]
   Get all data(password/cookie/history/bookmark) from chrome
VERSION:
   0.2.7
GLOBAL OPTIONS:
   --verbose, --vv                   Verbose (default: false)
   --compress, --cc                  Compress result to zip (default: false)
   --browser value, -b value         Available browsers: all|edge|firefox|chrome (default: "all")
   --results-dir value, --dir value  Export dir (default: "results")
   --format value, -f value          Format, csv|json|console (default: "csv")
   --help, -h                        show help (default: false)
   --version, -v                     print the version (default: false)

PS C:\test>  .\hack-browser-data.exe -b all -f json --dir results -cc
[x]:  Get 44 cookies, filename is results/microsoft_edge_cookie.json
[x]:  Get 54 history, filename is results/microsoft_edge_history.json
[x]:  Get 1 passwords, filename is results/microsoft_edge_password.json
[x]:  Get 4 bookmarks, filename is results/microsoft_edge_bookmark.json
[x]:  Get 6 bookmarks, filename is results/360speed_bookmark.json
[x]:  Get 19 cookies, filename is results/360speed_cookie.json
[x]:  Get 18 history, filename is results/360speed_history.json
[x]:  Get 1 passwords, filename is results/360speed_password.json
[x]:  Get 12 history, filename is results/qq_history.json
[x]:  Get 1 passwords, filename is results/qq_password.json
[x]:  Get 12 bookmarks, filename is results/qq_bookmark.json
[x]:  Get 14 cookies, filename is results/qq_cookie.json
[x]:  Get 28 bookmarks, filename is results/firefox_bookmark.json
[x]:  Get 10 cookies, filename is results/firefox_cookie.json
[x]:  Get 33 history, filename is results/firefox_history.json
[x]:  Get 1 passwords, filename is results/firefox_password.json
[x]:  Get 1 passwords, filename is results/chrome_password.json
[x]:  Get 4 bookmarks, filename is results/chrome_bookmark.json
[x]:  Get 6 cookies, filename is results/chrome_cookie.json
[x]:  Get 6 history, filename is results/chrome_history.json
[x]:  Compress success, zip filename is results/archive.zip
```


### TODO

[Desktop Browser Market Share Worldwide](https://gs.statcounter.com/browser-market-share/desktop/worldwide)

| Chrome | Safari | Firefox | Edge Legacy | IE |  Other  |
| :------:| :------: | :----: | :------: | :-----: | :--: |
| 68.33% |    9.4% | 8.91% |   4.41% |    3%    |  3%  |

[Desktop Browser Market Share China](https://gs.statcounter.com/browser-market-share/desktop/china)

| Chrome | 360 Safe | Firefox | QQ Browser |  IE   | Sogou Explorer |
| :----- | :------: | :-----: | :--------: | :---: | :------------: |
| 39.85% |  22.26%  |  9.28%  |    6.5%    | 5.65% |     4.74%      |

- [x] Chrome
- [x] QQ browser
- [x] Edge
- [x] 360 speed browser
- [x] Firefox
- [ ] Safari
- [ ] IE