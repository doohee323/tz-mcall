#!/usr/bin/env bash

#ROOTDIR=/Volumes/workspace
#ROOTDIR=/Volumes/workspace/ejn/ejn-devops-utils/projects
#ROOTDIR=/vagrant/projects

cd $ROOTDIR

mkdir -p $ROOTDIR/go
cd $ROOTDIR/go

curl -OL https://go.dev/dl/go1.18.1.linux-amd64.tar.gz
sha256sum go1.18.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xvf go1.18.1.linux-amd64.tar.gz

vi ~/.bash_profile
export GOROOT=/usr/local/go
#export GOROOT=/usr/local/opt/go/libexec
export GOPATH=$ROOTDIR/go
export PATH=$GOPATH/bin:.:$PATH
source ~/.bash_profile

go version

mkdir bin pkg src
mkdir -p src/github.com
mkdir -p src/github.com/ejnkr

cd $GOPATH/src/github.com/ejnkr
git clone https://github.com/ejnkr/tz-mcall.git
cd tz-mcall

export GO111MODULE=on
#go env -w GO111MODULE=auto
#go mod init github.com/ejnkr/tz-mcall
go mod init
go mod tidy
go get ./...
go mod vendor
go get -t github.com/ejnkr/tz-mcall

sudo apt update -y
sudo apt install golang-glide -y

glide install
glide update

#glide get github.com/spf13/viper
go get github.com/spf13/viper

#sudo ln -s /usr/local/go/bin/go /usr/local/bin/go
go clean --cache
go build

tz-mcall -i="ls -al" -f="plain"

exit

sudo chown -Rf vagrant:vagrant /var/run/docker.sock
docker build -f docker/Dockerfile -t tz-mcall:latest .
#docker run -d -p 8080:8080 tz-mcall:latest
docker run -p 8090:8090 -it tz-mcall:latest /app/tz-mcall -c=/app/mcall.yaml

#docker exec -it 6fd3377766c6 /bin/bash
#docker kill 6fd3377766c6

params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
curl http://localhost:8090/mcall/cmd/`echo $params | base64`

params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
curl http://k8s.mcall.ejncorp.com/mcall/cmd/`echo $params | base64`

