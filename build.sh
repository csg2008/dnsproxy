#!/bin/bash
# 20170921 chen.s.g <chensg@imohe.com> dns proxy build

APPNAME=dnsproxy
APPEXT=$(go env | grep GOEXE | awk -F '=' '{print $2}' | sed 's/"//g' )
VERSION=$(git tag | tail -1)
GOVER=$(go version | grep -Eo 'go([0-9]+\.)+[0-9]+')
GITBRANCH=$(git branch | awk '{print $NF}')
GITLOGREV=$(git log -1 | grep commit | head -n 1 | awk '{print $NF}')
builddate=$(date "+%Y%m%d%H%M%S")

go build -ldflags "-X main.appName=${APPNAME} -X main.buildVersion=${VERSION} -X main.buildDate=${builddate} -X main.buildRev=${GITBRANCH}.${GITLOGREV}.${GOVER}" -o ./bin/${APPNAME}${APPEXT} .