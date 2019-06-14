# tz-mcall

Concurrence with golang for multiple request (HTTP) or shell command.

-. install
```
	- go
		download and install 
		https://golang.org

		mkdir -p /Users/dhong/go
		cd /Users/dhong/go

		mkdir bin pkg src
		mkdir src/github.com
		mkdir src/github.com/doohee323

		vi ~/.bash_profile
		export GOROOT=/usr/local/go
		export GOPATH=/Users/dhong/go
		export PATH=$GOPATH/bin:$PATH
		source .bash_profile

	- glide
		sudo su
		export GOROOT=/usr/local/go
		export GOPATH=/Users/dhong/go
		export PATH=$GOPATH/bin:$PATH
		curl https://glide.sh/get | sh
		sudo ln -s /Users/dhong/go/bin/glide /usr/local/bin/glide
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
	go build
```
	
-. run:
```
	- case 1: run command
		tz-mcall -i="ls -al"
		tz-mcall -t=get -i=http://localhost:8000/test1
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
	
		mcall -c=/etc/mcall/mcall.cfg
		
	- case 3: write result on web
		mcall -w=true
		open brower and call with url, like http://localhost:8080/mcall/get/${params}
		ex) 
		http://localhost:8080/mcall/cmd/{"inputs":[{"input":"ls -al"},{"input":"ls"}]}
		http://localhost:8080/mcall/get/{"inputs":[{"input":"http://localhost:8080/test1","id":"aaa","pswd":"bbb"},{"input":"http://localhost:8080/test2","id":"aaa","pswd":"bbb"}]}
				
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
		
	ex) recon-cli example
	curl -d "type=cmd&params={"inputs":[{"input":"/usr/share/recon-ng/recon-cli -w=fortinet.com -m=domains-contacts/pgp_search -c=\"add`domains`fortinet.com\" -x"}]}"  http://localhost:8080/mcall

	=> need to be encoded.
	
	curl -d "type=cmd&params=%7B%22inputs%22%3A%5B%7B%22input%22%3A%22%2Fusr%2Fshare%2Frecon-ng%2Frecon-cli%20-w%3Dfortinet.com%20-m%3Ddomains-contacts%2Fpgp_search%20-c%3D%5C%22add%60domains%60fortinet.com%5C%22%20-x%22%7D%5D%7D"  http://localhost:8080/mcall
		  
```

-. to use:
```
	go get -u github.com/doohee323/tz_mcall/mcall
```
	
