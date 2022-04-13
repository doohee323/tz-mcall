# tz-mcall

Concurrence with golang for multiple request (HTTP) or shell command.

-. install
```
	- go
		download and install 
		https://golang.org
        
		mkdir -p /Volumes/workspace/go
		cd /Volumes/workspace/go

		mkdir bin pkg src
		mkdir src/github.com
		mkdir src/github.com/doohee323

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
	glide get github.com/op/go-logging
	glide get github.com/gorilla/pat
	glide get github.com/gorilla/mux
	glide get github.com/vaughan0/go-ini
	glide get github.com/stretchr/testify
	glide get golang.org/x/net/html
	glide get github.com/stretchr/testify/assert

	go version
	#sudo ln -s /usr/local/go/bin/go /usr/local/bin/go
	go clean --cache
	go build
```
	
-. run:
```
	- case 1: run command
		tz-mcall -i="ls -al"
		tz-mcall -t=get -i=http://google.com/config
		tz-mcall -t=post -i=http://localhost:8000/uptime_list?company_id=1^start_time=1464636372^end_time=1464722772

		cf) post with curl		
		curl -d "type=cmd&params={"inputs":[{"input":"ls -al"}]}"  http://localhost:8080/mcall 
		# params value is needed to url encoding like this,
		# curl -d "type=cmd&params=%7B%22inputs%22%3A%5B%7B%22input%22%3A%22ls%20-al%22%7D%5D%7D"  http://localhost:8080/mcall
		
	- case 2: use configration file
		vi /tz-mcall/etc/mcall.cfg
		[request]
		type=cmd
		input={"inputs":[{"input":"ls -al"},{"input":"ls"}]}
	
		tz-mcall -c=/etc/mcall/mcall.cfg
		//tz-mcall -c=/Volumes/workspace/go/src/github.com/doohee323/tz-mcall/etc/mcall.cfg
		
	- case 3: write result on web
		tz-mcall -w=true
		open brower and call with url, like http://localhost:8080/mcall/get/${params}
		ex) 
        params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
        curl http://localhost:8080/mcall/cmd/`echo $params | base64`

        params='{"inputs":[{"input":"http://google.com/config","id":"aaa","pswd":"bbb"},{"input":"http://google.com/aaa","id":"ccc"}]}'
        curl http://localhost:8080/mcall/get/`echo $params | base64`
```

-. paramters: 
```
	-t: request type ex) get, post, cmd, default: cmd
	-i: request url or command, it can be multiple with comma. 
		ex) http://localhost:8000/test, ls -al
			http://localhost:8000/test1, http://localhost:8000/test2
	-w: webserver on/off ex) on, default: off
	-p: webserver port ex) default: 8080
	-f: return format ex) json, plain, default: json
	-n: number of worker ex) default: 10
	-l: log level ex) debug, info, error, default: debug
	-lf: log file ex) /var/log/tz_mcall/tz_mcall.log, default: pwd
	-c: configration file ex) /etc/tz_mcall/tz_mcall.conf, default: none
	
	cf. If parameter has space(" "), you need to replace with "`" in the JSON paramter.
		ex) -c="add domains fortinet.com"  -> -c=\"add`domains`fortinet.com\"
		
	ex) webcall example
	curl -d "type=cmd&params={"inputs":[{"input":"ls -al"},{"input":"pwd"}]}"  http://localhost:8080/mcall
	=> need to be encoded.
	
	params='type=post&params={"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
	curl -d `echo $params | base64`  http://localhost:8080/mcall
	
	params='{"inputs":[{"input":"ls -al"},{"input":"pwd"}]}'
	curl http://localhost:8080/mcall?type=post&params=`echo $params | base64`
	
	http://localhost:8080/mcall?type=post&params={"inputs":[{"input":"http://core.local.xdn.com/test1","id":"aaa","pswd":"bbb"},{"input":"http://core.local.xdn.com/test2","id":"aaa","pswd":"bbb"}]}
		  
```

-. to use:
```
	go get -u github.com/doohee323/tz_mcall/mcall
```
	
