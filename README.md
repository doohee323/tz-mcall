# tz_mcall

Concurrence with golang for multiple request (HTTP) or shell command.

1. install
	- glide
		https://github.com/Masterminds/glide
	- ~/tz_mcall> glide up
	
2. run:
	- case 1: write result on a log file
		mcall --t=get --i=http://www.google.com/test1
		vi $PWD/tz_mcall.log
		
	- case 2: use configration file
		mcall --c=etc/mcall.cfg
		
	- case 3: write result on web
		mcall --t=get -w=on
		open brower and call with http://localhost:8080/mcall/${params}

3. paramters: 
	--t: request type ex) get, post, cmd
	--i: request url or command, it can be multiple with comma. 
		ex) http://www.google.com/test, ls -al
			http://www.google.com/test1, http://www.google.com/test2
	--w: webserver on/off ex) on, default: off
	--p: webserver port ex) default: 8080
	--f: return format ex) json, plain, default: json
	--n: number of worker ex) default: 10
	--l: log level ex) debug, info, error, default: debug
	--lf: log file ex) /var/log/tz_mcall/tz_mcall.log, default: pwd
	--c: configration file ex) /etc/tz_mcall/tz_mcall.conf, default: none
	

		