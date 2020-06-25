# HackBrowserData

一款支持全平台（Windows | MacOS | Linux）的浏览器数据（Password | History | Bookmark | Cookie）导出工具



### 安装编译

你可以选择下载编译好的[二进制文件](https://github.com/moonD4rk/HackBrowserData/releases)

也可以通过源码手动编译

```bash
git clone https://github.com/moonD4rk/HackBrowserData

cd HackBrowserData && go mod tidy
```

### 运行

```bash
./hack-browser-data -h
NAME:
   hack-browser-data - export passwords/cookies/history/bookmarks from browser

VERSION:
   0.1.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --browser value, -b value      browser name, all|chrome|safari (default: "all")
   --results-dir value, -d value  export dir (default: "results")
   --format value, -f value       result format, csv|json (default: "csv")
   --export-data value, -e value  all|password|cookie|history|bookmark (default: "all")
   --help, -h                     show help (default: false)
   --version, -v                  print the version (default: false)
```



### 目前支持平台

| Browser                           | Password | Cookie | Bookmark | History |
| :-------------------------------- | :------: | :----: | :------: | :-----: |
| Windows Chrome                    |    ✔     |   ✔    |    ✔     |    ✔    |
| MacOS Chrome<br />(need password) |    ✔     |   ✔    |    ✔     |    ✔    |
| Linux Chrome                      |    ✖     |   ✖    |    ✖     |    ✖    |
| Windows Edge                      |    ✖     |   ✖    |    ✖     |    ✖    |
| MacOS Edge                        |    ✖     |   ✖    |    ✖     |    ✖    |
| Linux Edge                        |    ✖     |   ✖    |    ✖     |    ✖    |
| MacOS Safari                      |    ✖     |   ✖    |    ✖     |    ✖    |
| MacOS Keychain                    |    ✖     |        |          |         |

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
- [ ] Edge
- [ ] 360 browser
- [ ] IE