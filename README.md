# HackBrowserData

[English Readme](https://github.com/moonD4rk/HackBrowserData/blob/master/README_EN.md) 

hack-browser-data 是一个浏览器数据（密码|历史记录|Cookies|书签）导出工具，支持全平台主流浏览器。

### 各平台浏览器支持情况

#### Windows

| 浏览器                      | 密码 | Cookie | 书签 | 历史记录 |
| :--------------------------- | :------: | :----: | :------: | :-----: |
| Google Chrome (全版本) |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox                    |    ✅     |   ✅    |    ✅     |    ✅    |
| Microsoft Edge               |    ✅     |   ✅    |    ✅     |    ✅    |
| 360 急速浏览器    |    ✅     |   ✅    |    ✅     |    ✅    |
| QQ 浏览器               |    ✅     |   ✅    |    ✅     |    ✅    |
| IE 浏览器        |    ❌     |   ❌    |    ❌     |    ❌    |

#### MacOS

由于 MacOS 的安全性设置，基于 `Chromium` 内核浏览器解密时需要用户密码

| Browser                      | Password | Cookie | Bookmark | History |
| :--------------------------- | :------: | :----: | :------: | :-----: |
| Google Chrome<br />需要密码  |    ✅     |   ✅    |    ✅     |    ✅    |
| Firefox                      |    ✅     |   ✅    |    ✅     |    ✅    |
| Microsoft Edge<br />需要密码 |    ✅     |   ✅    |    ✅     |    ✅    |
| Safari                       |    ❌     |   ❌    |    ❌     |    ❌    |

#### Linux

| Browser       | Password | Cookie | Bookmark | History |
| :------------ | :------: | :----: | :------: | :-----: |
| Firefox       |    ✅     |   ✅    |    ✅     |    ✅    |
| Google Chrome |    ❌     |   ❌    |    ❌     |    ❌    |


### 安装运行

基于Golang，如图方便可以下载我编译好的[二进制文件 ](https://github.com/moonD4rk/HackBrowserData/releases) 就行

#### 自己编译

支持版本 `go 1.11+`

```bash
git clone https://github.com/moonD4rk/HackBrowserData

cd HackBrowserData

go get -v -t -d ./...

go build
```

#### 运行

```shell
PS C:\hack> .\hack.exe -h
NAME:
   hack-browser-data - Export passwords/cookies/history/bookmarks from browser

USAGE:
   [hack-browser-data -b chrome -f json -dir results -e all]
   Get all data(password/cookie/history/bookmark) from chrome

GLOBAL OPTIONS:
   --verbose, --vv                   Verbose (default: false)
   --browser value, -b value         Available browsers: all|360|qq|firefox|chrome|edge (default: "all")
   --results-dir value, --dir value  Export dir (default: "results")
   --format value, -f value          Format, csv|json (default: "csv")
   --export-data value, -e value     all|password|cookie|history|bookmark (default: "all")
   --help, -h                        show help (default: false)

PS C:\hack> .\hack.exe -b all -f json -e all --dir windows-results
[x]:  Get 6 history, filename is windows-results/Chrome_cookie.json
[x]:  Get 6 history, filename is windows-results/Chrome_history.json
[x]:  Get 1 history, filename is windows-results/Chrome_password.json
[x]:  Get 1 history, filename is windows-results/Microsoft_Edge_password.json
[x]:  Get 45 history, filename is windows-results/Microsoft_Edge_cookie.json
[x]:  Get 54 history, filename is windows-results/Microsoft_Edge_history.json
[x]:  Get 18 history, filename is windows-results/360speed_history.json
[x]:  Get 6 bookmarks, filename is windows-results/360speed_bookmark.json
[x]:  Get 1 history, filename is windows-results/360speed_password.json
[x]:  Get 19 history, filename is windows-results/360speed_cookie.json
[x]:  Get 12 bookmarks, filename is windows-results/qq_bookmark.json
[x]:  Get 1 history, filename is windows-results/qq_password.json
[x]:  Get 14 history, filename is windows-results/qq_cookie.json
[x]:  Get 12 history, filename is windows-results/qq_history.json
[x]:  Get 10 history, filename is windows-results/Firefox_cookie.json
[x]:  Get 33 history, filename is windows-results/Firefox_history.json
[x]:  Get 28 bookmarks, filename is windows-results/Firefox_bookmark.json
[x]:  Get 1 history, filename is windows-results/Firefox_password.json
```


### TODO

[Desktop Browser Market Share Worldwide](https://gs.statcounter.com/browser-market-share/desktop/worldwide)

| Chrome | Safari | Firefox | Edge Legacy |  IE  | Other |
| :----: | :----: | :-----: | :---------: | :--: | :---: |
| 68.33% |  9.4%  |  8.91%  |    4.41%    |  3%  |  3%   |

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