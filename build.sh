#!/bin/sh

set -eux

PROJECT_ROOT="/go/src/github.com/${GITHUB_REPOSITORY}"

mkdir -p $PROJECT_ROOT
rmdir $PROJECT_ROOT
ln -s $GITHUB_WORKSPACE $PROJECT_ROOT
cd $PROJECT_ROOT
go get -v ./...

EXT=''

if [ $GOOS == 'windows' ]; then
EXT='.exe'
fi

if [ -x "./build.sh" ]; then
  OUTPUT=`./build.sh "${CMD_PATH}"`
else
  go build "${CMD_PATH}"
  OUTPUT="${PROJECT_NAME}${EXT}"
fi

echo ${OUTPUT}