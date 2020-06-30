# HackBrowserData

hack-browser-data is an open-source tool that could help you export data from browser. It supports the most popular browsers on the market and runs on Windows and macOS.

### Supported Browser

| Browser                             | Password | Cookie | Bookmark | History |
| :---------------------------------- | :------: | :----: | :------: | :-----: |
| Chrome <= 80 [Windows]       |    ✅    |   ✅   |    ✅    |    ✅    |
| Chrome  > 80 [Windows]      |    ✅    |   ✅   |    ✅    |    ✅    |
| Chrome [MacOS]<br />(require password) |    ✅    |   ✅   |    ✅    |    ✅    |
| Edge [Windows]                      |    ✅    |   ✅   |    ✅    |    ✅    |
| Edge [MacOS]<br />(require password)   |    ✅    |   ✅   |    ✅    |    ✅    |
| 360 Speed Browser [Windows]        |    ✅    |   ✅   |    ✅    |    ✅    |
| QQ Browser [Windows]                |    ✅    |   ✅   |    ✅    |    ✅    |
| FireFox [Windows]                   |    ❌   |   ❌   |    ❌     |    ❌    |
| FireFox [MacOS]                     |    ❌    |   ❌   |    ❌    |    ❌    |
| Safari [MacOS]                      |    ❌    |   ❌   |    ❌    |    ❌    |
| Internet Explorer [Windows]         |    ❌    |   ❌   |    ❌    |    ❌    |
| 360 Secure Browser [Windows]         |    ❌    |   ❌   |    ❌    |    ❌    |
| Chrome [Linux]                      |    ❌    |   ❌   |    ❌    |    ❌    |

### Install

Installation of hack-browser-data is dead-simple, just download [the release for your system](https://github.com/moonD4rk/HackBrowserData/releases) and run the binary.

#### Building from source

```bash
git clone https://github.com/moonD4rk/HackBrowserData

cd HackBrowserData && go mod tidy

go build
```

#### Run

```shell
✗ .\hack-browser-data.exe -h
NAME:
   hack-browser-data - Export passwords/cookies/history/bookmarks from browser

USAGE:
   [hack-browser-data -b chrome -f json -dir results -e all]
   Get all data(password/cookie/history/bookmark) from chrome

GLOBAL OPTIONS:
   --verbose, --vv                   verbose (default: false)
   --browser value, -b value         available browsers: chrome|edge|360speed|qq (default: "chrome")
   --results-dir value, --dir value  Export dir (default: "results")
   --format value, -f value          result format, csv|json (default: "csv")
   --export-data value, -e value     all|password|cookie|history|bookmark (default: "all")
   --help, -h                        show help (default: false)
```



```shell
✗ ./hack-browser-data.exe -b chrome -f json -dir results -e all
[x]:  Get 538 bookmarks, filename is results/bookmarks_chrome.json 
[x]:  Get 1610 cookies, filename is results/cookies_chrome.json 
[x]:  Get 44050 history, filename is results/history_chrome.json 
[x]:  Get 457 login data, filename is results/login_data_chrome.json 
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
- [ ] 360 secure browser
- [ ] Safari
- [ ] Firefox
- [ ] IE