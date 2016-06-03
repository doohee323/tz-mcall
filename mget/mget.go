package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/pat"
	logging "github.com/op/go-logging"
	"github.com/vaughan0/go-ini"
	"io"
	"io/ioutil"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	INPUTS []string
	STYPE  string
)

var (
	cfg        ini.File
	CONFIGFILE string
	WORKERNUM  = 10
	HTTPHOST   = "localhost"
	HTTPPORT   = "8080"
)

var (
	LOGFILE   *os.File
	LOGFMT                  = "%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
	LOGFORMAT               = logging.MustStringFormatter(LOGFMT)
	LOG                     = logging.MustGetLogger("logfile")
	GLOGLEVEL logging.Level = logging.DEBUG
)

type FetchedResult struct {
	input   string
	content string
}

type FetchedInput struct {
	m map[string]error
	sync.Mutex
}

type Queryer interface {
	Query()
}

type CallFetch struct {
	fetchedInput *FetchedInput
	p            *Pipeline
	result       chan FetchedResult
	input        string
}

func htmlFetch(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	LOG.Debug("==== %s", input)
	res, err := http.Get(input)
	if err != nil {
		LOG.Panic(err)
		return "", err
	}
	defer res.Body.Close()
	doc, err := ioutil.ReadAll(res.Body)
	if err != nil {
		LOG.Panic(err)
		return "", err
	}
	return string(doc), nil
}

func (g *CallFetch) Request(input string) {
	g.p.request <- &CallFetch{
		fetchedInput: g.fetchedInput,
		p:            g.p,
		result:       g.result,
		input:        input,
	}
}

func (g *CallFetch) parseContent(input string, doc string) <-chan string {
	content := make(chan string)
	go func() {
		content <- doc
		chk := false
		val := ""
		g.fetchedInput.Lock()
		for n := range INPUTS {
			if _, ok := g.fetchedInput.m[INPUTS[n]]; !ok {
				chk = true
				val = INPUTS[n]
				g.Request(val)
				break
			}
		}
		if chk == false {
		}
		g.fetchedInput.Unlock()
	}()
	return content
}

func (g *CallFetch) Query() {
	g.fetchedInput.Lock()
	if _, ok := g.fetchedInput.m[g.input]; ok {
		g.fetchedInput.Unlock()
		return
	}
	g.fetchedInput.Unlock()

	doc, err := htmlFetch(g.input)
	if err != nil {
		go func(u string) {
			g.Request(u)
		}(g.input)
		return
	}

	g.fetchedInput.Lock()
	g.fetchedInput.m[g.input] = err
	g.fetchedInput.Unlock()

	content := <-g.parseContent(g.input, doc)
	g.result <- FetchedResult{g.input, content}
}

type Pipeline struct {
	request chan Queryer
	done    chan struct{}
	wg      *sync.WaitGroup
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		request: make(chan Queryer),
		done:    make(chan struct{}),
		wg:      new(sync.WaitGroup),
	}
}

func (p *Pipeline) Worker() {
	for r := range p.request {
		select {
		case <-p.done:
			return
		default:
			r.Query()
		}
	}
}

func (p *Pipeline) Run() {
	p.wg.Add(WORKERNUM)
	for i := 0; i < WORKERNUM; i++ {
		go func() {
			p.Worker()
			p.wg.Done()
		}()
	}

	go func() {
		p.wg.Wait()
	}()
}

func mainExec() map[string]string {
	start := time.Now()
	r := new(big.Int)
	fmt.Println(r.Binomial(1000, 10))

	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	p := NewPipeline()
	p.Run()

	call := &CallFetch{
		fetchedInput: &FetchedInput{m: make(map[string]error)},
		p:            p,
		result:       make(chan FetchedResult),
		input:        "",
	}
	p.request <- call

	result := make(map[string]string)
	count := 0
	LOG.Debug("=len(INPUTS)=== %d", len(INPUTS))
	for a := range call.result {
		LOG.Debug("==== %d %s", a.input, a.content)
		count++
		countStr := strconv.Itoa(count)
		result[countStr] = a.content
		LOG.Debug("==== count: %d", count)
		if count > len(INPUTS) {
			close(p.done)
			break
		}
	}

	elapsed := time.Since(start)
	LOG.Debug("It took %s", elapsed)
	return result
}

// http://localhost:8080/mcall/get/%7B%22inputs%22%3A%5B%7B%22input%22%3A%22http%3A%2F%2Fcore.local.xdn.com%2Ftest1%22%2C%22id%22%3A%22aaa%22%2C%22pswd%22%3A%22bbb%22%7D%2C%7B%22input%22%3A%22http%3A%2F%2Fcore.local.xdn.com%2Ftest2%22%2C%22id%22%3A%22aaa%22%2C%22pswd%22%3A%22bbb%22%7D%5D%7D
func getHandle(w http.ResponseWriter, r *http.Request) {
	STYPE = r.URL.Query().Get(":type")
	paramStr := r.URL.Query().Get(":params")
	fmt.Println(STYPE, paramStr)

	getInput(paramStr)
	b := makeResponse()
	w.Write(b)
}

// http://localhost:8080/mcall?type=post&params={"inputs":[{"input":"http://core.local.xdn.com/test1","id":"aaa","pswd":"bbb"},{"input":"http://core.local.xdn.com/test2","id":"aaa","pswd":"bbb"}]}
func postHandle(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		LOG.Debugf("ParseForm %s", err)
	}
	LOG.Debugf("\n what we got was %+v\n", r.Form)

	if STYPE = r.FormValue("type"); STYPE == "" {
		LOG.Warning(fmt.Sprintf("bad STYPE received %+v", r.Form["type"]))
		return
	}

	var paramStr = ""
	if paramStr = r.FormValue("params"); paramStr == "" {
		LOG.Warning(fmt.Sprintf("bad params received %+v", r.Form["params"]))
		return
	}
	fmt.Println(STYPE, paramStr)

	getInput(paramStr)

	b := makeResponse()
	fmt.Println(b)
	io.WriteString(w, string(b))
	//	io.WriteString(w, "test")
}

func makeResponse() []byte {
	res := make(map[string]string)
	result := mainExec()

	res["status"] = "OK"
	res["ts"] = time.Now().String()
	str, err := json.Marshal(result)
	fmt.Println(err)
	fmt.Println(str)
	res["count"] = strconv.Itoa(len(result) - 1)
	res["result"] = string(str)

	b, err := json.Marshal(res)
	if err != nil {
		LOG.Errorf("error: %s", err)
	}
	return b
}

func webserver() {
	killch := make(chan os.Signal, 1)
	signal.Notify(killch, os.Interrupt)
	signal.Notify(killch, syscall.SIGTERM)
	signal.Notify(killch, syscall.SIGINT)
	signal.Notify(killch, syscall.SIGQUIT)
	go func() {
		<-killch
		LOG.Fatalf("Interrupt %s", time.Now().String())
	}()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		r := pat.New()
		r.Get("/mcall/{type}/{params}", http.HandlerFunc(getHandle))
		r.Post("/mcall", http.HandlerFunc(postHandle))
		http.Handle("/", r)
		LOG.Debug("Listening %s : %s", HTTPHOST, HTTPPORT)
		err := http.ListenAndServe(HTTPHOST+":"+HTTPPORT, nil)
		if err != nil {
			LOG.Fatalf("ListenAndServe: %s", err)
		}
		wg.Done()
	}()

	wg.Wait()
}

func getInput(aInput string) {
	type Inputs struct {
		Inputs []map[string]interface{} `json:"inputs"`
	}
	var data Inputs
	err := json.Unmarshal([]byte(aInput), &data)
	if err != nil {
		fmt.Println("Unmarshal error %s", err)
	}
	for i := range data.Inputs {
		input := data.Inputs[i]["input"]
		INPUTS = append(INPUTS, input.(string))
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// 2way of run
// - 1st: mget web
// 		call from brower: http://localhost:8080/main/core/1418,1419,2502,2694,2932,2933,2695
// - 2nd: mget core/graphite 1418,1419,2502,2694,2932,2933,2695
//////////////////////////////////////////////////////////////////////////////////////////////////////
func main() {

	if len(os.Args) < 2 {
		fmt.Println("No parameter!")
		return
	} else {
		// mcall --t=get --i=http://core.local.xdn.com/1/stats/uptime_list?company_id=1^start_time=1464636372^end_time=1464722772^hc_id=1418
		// mcall --c=/Users/dhong/Documents/workspace/go/src/tz.com/tz_mcall/etc/mcall.cfg
		allArgs := os.Args[1:]
		fmt.Println("==== %s", allArgs)
		////[ argument ]////////////////////////////////////////////////////////////////////////////////
		web_enble := false
		for i := range allArgs {
			str := allArgs[i]
			key := str[0:strings.Index(str, "=")]
			val := str[strings.Index(str, "=")+1 : len(str)]
			switch key {
			case "--t":
				STYPE = val
			case "--i":
				INPUTS = append(INPUTS, val)
			case "--c":
				CONFIGFILE = val
			case "--w":
				if val == "on" {
					web_enble = true
				}
			}
		}

		////[ configuratin file ]////////////////////////////////////////////////////////////////////////////////
		var logfile = ""
		if CONFIGFILE != "" {
			cfg, err := ini.LoadFile(CONFIGFILE)
			if err != nil {
				fmt.Println("parse config "+CONFIGFILE+" file error: ", err)
			}

			workerNumber, ok := cfg.Get("worker", "number")
			fmt.Println("workerNumber: ", workerNumber)
			if !ok {
				fmt.Println("'file' missing from 'worker", "number")
			} else {
				WORKERNUM, _ = strconv.Atoi(workerNumber)
			}

			webEnbleStr, ok := cfg.Get("webserver", "enable")
			fmt.Println("web_enble: ", webEnbleStr)
			if !ok {
				fmt.Println("'enable' missing from 'webserver", "enable")
			}

			if webEnbleStr == "on" {
				web_enble = true
				httpost, ok := cfg.Get("webserver", "host")
				fmt.Println("httpost: ", httpost)
				if !ok {
					fmt.Println("'host' missing from 'webserver", "host")
				} else {
					HTTPHOST = httpost
				}

				httpport, ok := cfg.Get("webserver", "port")
				fmt.Println("httpport: ", httpport)
				if !ok {
					fmt.Println("'port' missing from 'webserver", "port")
				} else {
					HTTPPORT = httpport
				}
			} else {
				input, ok := cfg.Get("request", "input")
				if !ok {
					fmt.Println("'input' missing from 'request' section")
				}
				getInput(input)
			}
		}

		////[ log file ]////////////////////////////////////////////////////////////////////////////////
		if logfile == "" {
			logfile = "/var/log/mcall/mcall.log"
		}
		if _, err := os.Stat(logfile); err != nil {
			logfile, _ := os.Getwd()
			logfile = logfile + "/mget.log"
		}

		LOGFILE, err := os.OpenFile(logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			LOG.Fatalf("Log file error: %s %s", logfile, err)
		}
		defer func() {
			LOGFILE.WriteString(fmt.Sprintf("closing %s", time.UnixDate))
			LOGFILE.Close()
		}()

		logback := logging.NewLogBackend(LOGFILE, "", 0)
		logformatted := logging.NewBackendFormatter(logback, LOGFORMAT)
		loglevel := "DEBUG"
		GLOGLEVEL, err := logging.LogLevel(loglevel)
		if err != nil {
			GLOGLEVEL = logging.DEBUG
		}
		logging.SetBackend(logformatted)
		logging.SetLevel(GLOGLEVEL, "")

		////[ run app ]////////////////////////////////////////////////////////////////////////////////
		if web_enble == true {
			webserver()
		} else {
			mainExec()
		}
	}
}
