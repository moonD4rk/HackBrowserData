# HackBrowserData

一款支持全平台（Windows | MacOS | Linux）的浏览器数据（Password | History | Bookmark | Cookie）导出工具



### 安装编译

你可以选择下载编译好的[二进制文件](https://github.com/moonD4rk/HackBrowserData/releases)

也可以通过源码手动编译

```bash
git clone https://github.com/moonD4rk/HackBrowserData

cd HackBrowserData && go mod tidy

go build
```

### 运行

```bash
./hack-browser-data -h
NAME:
   hack-browser-data - export passwords/cookies/history/bookmarks from browser

USAGE:
   [hack-browser-data -b chrome -f json -dir results -e all]
   Get all data(password/cookie/history/bookmark) from chrome

GLOBAL OPTIONS:
   --verbose, --vv                   verbose (default: false)
   --browser value, -b value         available browsers: edge|chrome (default: "chrome")
   --results-dir value, --dir value  export dir (default: "results")
   --format value, -f value          result format, csv|json (default: "csv")
   --export-data value, -e value     all|password|cookie|history|bookmark (default: "all")
   --help, -h                        show help (default: false)

```



### 目前支持平台



| Browser                             | Password | Cookie | Bookmark | History |
| :---------------------------------- | :------: | :----: | :------: | :-----: |
| Chrome [Windows]                    |    ✔     |   ✔    |    ✔     |    ✔    |
| Chrome [MacOS]<br />(need password) |    ✔     |   ✔    |    ✔     |    ✔    |
| Edge [Windows]                      |    ✔     |   ✔    |    ✔     |    ✔    |
| Edge [MacOS]<br />(need password)   |    ✔     |   ✔    |    ✔     |    ✔    |
| 360 Secure Browser [Windows]        |    ✔     |   ✔    |    ✔     |    ✔    |
| 360 Speed Browser [Windows]         |    ✖     |   ✖    |    ✖     |    ✖    |
| FireFox [Windows]                   |    ✖     |   ✖    |    ✖     |    ✖    |
| FireFox [MacOS]                     |    ✖     |   ✖    |    ✖     |    ✖    |
| Safari [MacOS]                      |    ✖     |   ✖    |    ✖     |    ✖    |
| Internet Explorer [Windows]         |    ✖     |   ✖    |    ✖     |    ✖    |
| QQ Browser [Windows]                |    ✖     |   ✖    |    ✖     |    ✖    |
| Chrome [Linux]                      |    ✖     |   ✖    |    ✖     |    ✖    |


### Todo List

[Desktop Browser Market Share Worldwide](https://gs.statcounter.com/browser-market-share/desktop/worldwide)

| Chrome | Safari | Firefox | Edge Legacy | IE |  Other  |
| :------:| :------: | :----: | :------: | :-----: | :--: |
| 68.33% |    9.4% | 8.91% |   4.41% |    3%    |  3%  |

[Desktop Browser Market Share China](https://gs.statcounter.com/browser-market-share/desktop/china)

| Chrome | 360 Safe | Firefox | QQ Browser |  IE   | Sogou Explorer |
| :----- | :------: | :-----: | :--------: | :---: | :------------: |
| 39.85% |  22.26%  |  9.28%  |    6.5%    | 5.65% |     4.74%      |

Based on those two lists, I woulf support those browser in the future

- [x] Chrome
- [ ] Safari
- [ ] Firefox
- [x] Edge
- [x] 360 browser
- [ ] IE