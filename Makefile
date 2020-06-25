LinuxOS=CGO_ENABLED=0 GOOS=linux GOARCH=amd64
MacOS=CGO_ENABLED=0 GOOS=darwin GOARCH=amd64
Windows=CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows GOARCH=amd64
DATE=$(shell date +'%Y-%m-%d %H:%M:%S')

win:
		$(Windows) go build -o /Users/finkployd/Desktop/hack.exe main.go
