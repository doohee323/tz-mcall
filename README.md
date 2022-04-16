# tz-mcall

Concurrence with golang for multiple request (HTTP) or shell command.

-. install
```
	- go
		download and install 
		https://golang.org
        
        #WORKDIR=/Volumes/workspace/go
        #WORKDIR=/Volumes/workspace/go
        
		mkdir -p /Volumes/workspace/go
		cd /Volumes/workspace/go

		mkdir bin pkg src
		mkdir -p src/github.com
		mkdir -p src/github.com/doohee323

		vi ~/.bash_profile
		//export GOROOT=/usr/local/go
		export GOROOT=/usr/local/opt/go/libexec
		export GOPATH=/Volumes/workspace/go
		export PATH=$GOPATH/bin:.:$PATH
		source .bash_profile
		
	- glide
		sudo su
		export GOROOT=/usr/local/go
		export GOPATH=/Volumes/workspace/go
		export PATH=$GOPATH/bin:.:$PATH
		// curl https://glide.sh/get | sh
		// sudo ln -s /Volumes/workspace/go/bin/glide /usr/local/bin/glide
		brew install glide
		cf. https://github.com/Masterminds/glide
```

-. build:
```
	cd $GOPATH/src/github.com/doohee323
	git clone https://github.com/doohee323/tz-mcall.git
	cd tz-mcall

	glide install
	glide update
	
	# It contains as below
	export GO111MODULE=on
	#go env -w GO111MODULE=auto
	go mod init
	go mod tidy
	go get ./...
	go mod vendor
	go get -t github.com/doohee323/tz-mcall

	glide install
	glide update
	
	//glide get github.com/spf13/viper
	go get github.com/spf13/viper

	go version
	#sudo ln -s /usr/local/go/bin/go /usr/local/bin/go
	go clean --cache
	go build
```
	
-. run:
```
	- case 1: run command
		tz-mcall -i="ls -al"
		tz-mcall -t=get -i=http://localhost:3000/healthcheck
		tz-mcall -t=post -i=http://localhost:8000/uptime_list?company_id=1^start_time=1464636372^end_time=1464722772

		cf) post with curl		
		params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}' 
		curl -d "type=cmd&params=`echo $params | base64`" http://localhost:3000/mcall

		params='{"inputs":[{"input":"http://google.com/config","id":"aaa","pswd":"bbb"},{"input":"http://google.com/aaa","id":"ccc"}]}' 
		curl -d "type=post&params=`echo $params | base64`" http://localhost:3000/mcall
		
	- case 2: use configration file
		vi /tz-mcall/etc/mcall.yaml
		[request]
		type=cmd
		input={"inputs":[{"input":"ls -al"},{"input":"ls"}]}
	
		tz-mcall -c=/etc/mcall/mcall.yaml
		//tz-mcall -c=/Volumes/workspace/go/src/github.com/doohee.hong/tz-mcall/etc/mcall.yaml
		
	- case 3: write result on web
		tz-mcall -w=true
		
		ex) 
        params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
        curl http://localhost:3000/mcall/cmd/`echo $params | base64`

        params='{"inputs":[{"input":"http://google.com/config","id":"aaa","pswd":"bbb"},{"input":"http://google.com/aaa","id":"ccc"}]}'
        curl http://localhost:3000/mcall/get/`echo $params | base64`
```

-. paramters: 
```
	-t: request type ex) get, post, cmd, default: cmd
	-i: request url or command, it can be multiple with comma. 
		ex) http://localhost:8000/test, ls -al
			http://localhost:8000/test1, http://localhost:8000/test2
	-w: webserver on/off ex) on, default: off
	-p: webserver port ex) default: 3000
	-f: return format ex) json, plain, default: json
	-e: return result with encoding ex) std, url
	-n: number of worker ex) default: 10
	-l: log level ex) debug, info, error, default: debug
	-lf: log file ex) /var/log/tz-mcall/tz-mcall.log, default: pwd
	-c: configration file ex) /etc/tz-mcall/tz-mcall.conf, default: none
	
	cf. If parameter has space(" "), you need to replace with "`" in the JSON paramter.
		ex) -c="add domains fortinet.com"  -> -c=\"add`domains`fortinet.com\"
		
	ex) after "tz-mcall -w=true"
	curl -d "type=cmd&params={"inputs":[{"input":"ls -al"},{"input":"pwd"}]}"  http://localhost:3000/mcall
	=> need to be encoded.
	
	params='type=post&params={"inputs":[{"input":"http://google.com/test1","id":"aaa","pswd":"bbb"},{"input":"http://google.com/test2","id":"aaa","pswd":"bbb"}]}'
	curl -d $params  http://localhost:3000/mcall
	
	params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
	curl http://localhost:3000/mcall?type=post&params=`echo $params | base64`
	
	http://localhost:3000/mcall?type=post&params={"inputs":[{"input":"http://google.com/test1","id":"aaa","pswd":"bbb"},{"input":"http://google.com/test2","id":"aaa","pswd":"bbb"}]}
		  
```

-. to use:
```
	go get -u github.com/doohee.hong/tz-mcall/mcall
```
params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
curl http://localhost:3000/mcall/cmd/`echo $params | base64`

-. parsing
```
    - host check
        - port healthcheck
            tz-mcall -i="telnet localhost 3000" | grep "'^]'" | wc -l
        - file exist 
            tz-mcall -i="ls /etc/hosts" | grep "/etc/hosts" | wc -l
    - url check from host
        tz-mcall -t=get -i=http://localhost:3000/healthcheck

    - only get the result
        tz-mcall -i="ls -al" -e=std | jq '.result' | awk '{print substr($1, 2, length($1)-2)}' | base64 --decode
```
