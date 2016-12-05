package main

import (
//	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gorilla/pat"
	logging "github.com/op/go-logging"
	"github.com/vaughan0/go-ini"
	"io"
	"io/ioutil"
	_ "math/big"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	CFG        ini.File
	CONFIGFILE string
	WORKERNUM  = 10
	INPUTS     []string
	WAITSTRS   []string
	STYPE      string
	WEBENABLED = false
	HTTPHOST   = "localhost"
	HTTPPORT   = "8080"
)

var (
	LOGFILE   *os.File
	LOGFMT                  = "%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
	LOGFORMAT               = logging.MustStringFormatter(LOGFMT)
	LOG                     = logging.MustGetLogger("logfile")
	GLOGLEVEL logging.Level = logging.DEBUG
	logfile   string
	loglevel  string
)

type FetchedResult struct {
	input   string
	content string
}

type FetchedInput struct {
	m map[string]error
	sync.Mutex
}

type Commander interface {
	command()
}

type CallFetch struct {
	fetchedInput *FetchedInput
	p            *Pipeline
	result       chan FetchedResult
	input        string
	waitStr      string
}

func fetchHtml(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	LOG.Debug("= input: ", input)
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
	} else {
		LOG.Debug(string(doc))
	}
	return string(doc), nil
}

func fetchCmd(input string, waitStr string) (string, error) {
	if input == "" {
		return "", nil
	}

	LOG.Debug("==============================================================")
	LOG.Debug("= input: ", input)
	LOG.Debug("= waitStr: ", waitStr)
	doc, err := exeCmd(input, waitStr)
	if err != nil {
		LOG.Panic(err)
		return "", err
	} else {
		LOG.Debug(doc)
		fmt.Printf("%s", doc)
	}
	return string(doc), nil
}

type ResultDoc struct {
	Raw   string `json:"raw"`
	Error string `json:"error"`
}

func exeCmd(str string, waitStr string) (string, error) {
	res := ResultDoc{}

	parts := strings.Fields(str)
	cmdName := parts[0]
	LOG.Debug("= cmdName: ", cmdName)
	args := parts[1:len(parts)]
	LOG.Debug("= args: ", args)

	//get a pointer to a proc
	cmd := exec.Command(cmdName, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		fmt.Printf("Error: %s", err)
	}
	err = cmd.Start()
	if err != nil {
		fmt.Printf("Start error %s", err)
	}

	stdo, g := ioutil.ReadAll(stdout)
	stde, f := ioutil.ReadAll(stderr)

	d := cmd.Wait()

	if d != nil {
		fmt.Println(d)
	}

	if g != nil {
		fmt.Println(g)
	}

	if f != nil {
		fmt.Println(f)
		res.Error = string(stde)
		return "", errors.New(res.Error)
	}

	fmt.Printf("Standard err is %s \n", stde)
	fmt.Printf("res.Error is %s \n", res.Error)
	res.Raw = string(stdo)
	fmt.Printf("Standard out is %s \n", stdo)

	return res.Raw, nil
}

func (g *CallFetch) request(input string, waitStr string) {
	g.p.request <- &CallFetch{
		fetchedInput: g.fetchedInput,
		p:            g.p,
		result:       g.result,
		input:        input,
		waitStr:      waitStr,
	}
}

func (g *CallFetch) parseContent(input string, doc string) <-chan string {
	content := make(chan string)
	go func() {
		content <- doc
		chk := false
		val := ""
		waitStr := ""
		g.fetchedInput.Lock()
		for n := range INPUTS {
			if _, ok := g.fetchedInput.m[INPUTS[n]]; !ok {
				chk = true
				val = INPUTS[n]
				waitStr = WAITSTRS[n]
				g.request(val, waitStr)
				break
			}
		}
		if chk == false {
		}
		g.fetchedInput.Unlock()
	}()
	return content
}

func (g *CallFetch) command() {
	g.fetchedInput.Lock()
	if _, ok := g.fetchedInput.m[g.input]; ok {
		g.fetchedInput.Unlock()
		return
	}
	g.fetchedInput.Unlock()

	var doc string
	var err error
	if g.input != "" {
		if STYPE == "cmd" {
			doc, err = fetchCmd(g.input, g.waitStr)
			if err != nil {
				go func(u string, w string) {
					g.request(u, w)
				}(g.input, g.waitStr)
				return
			}
		} else {
			doc, err = fetchHtml(g.input)
			if err != nil {
				go func(u string, w string) {
					g.request(u, w)
				}(g.input, g.waitStr)
				return
			}
		}
	}

	g.fetchedInput.Lock()
	g.fetchedInput.m[g.input] = err
	g.fetchedInput.Unlock()

	content := <-g.parseContent(g.input, doc)
	g.result <- FetchedResult{g.input, content}
}

type Pipeline struct {
	request chan Commander
	done    chan struct{}
	wg      *sync.WaitGroup
}

func NewPipeline() *Pipeline {
	return &Pipeline{
		request: make(chan Commander),
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
			r.command()
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

func execCmd() map[string]string {
	start := time.Now()
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	p := NewPipeline()
	p.Run()

	call := &CallFetch{
		fetchedInput: &FetchedInput{m: make(map[string]error)},
		p:            p,
		result:       make(chan FetchedResult),
		input:        "",
		waitStr:      "",
	}
	p.request <- call

	result := make(map[string]string)
	count := 0
	LOG.Debug("============ len(INPUTS): ", len(INPUTS))
	for a := range call.result {
		LOG.Debug("============ a: ", a)
		count++
		countStr := strconv.Itoa(count)
		result[countStr] = a.content
		LOG.Debug("============ count: ", count)
		if count >= len(INPUTS) {
			LOG.Debug("============ closed ")
			close(p.done)
			break
		} else {
			LOG.Debug("============ test ")
		}
	}

	elapsed := time.Since(start)
	LOG.Debug("elapsed: ", elapsed)
	return result
}

// http://localhost:8080/mcall/cmd/{"inputs":[{"input":"ls -al"},{"input":"ls"}]}
func getHandle(w http.ResponseWriter, r *http.Request) {
	STYPE = r.URL.Query().Get(":type")
	paramStr := r.URL.Query().Get(":params")
	LOG.Debug(STYPE, paramStr)

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
	LOG.Debug(STYPE, paramStr)

	getInput(paramStr)
	b := makeResponse()
	io.WriteString(w, string(b))
}

func makeResponse() []byte {
	result := execCmd()

	res := make(map[string]string)
	res["status"] = "OK"
	res["ts"] = time.Now().String()
	str, err := json.Marshal(result)
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
		LOG.Debug("Listening: ", HTTPHOST, HTTPPORT)
		err := http.ListenAndServe(HTTPHOST+":"+HTTPPORT, nil)
		if err != nil {
			LOG.Fatalf("ListenAndServe: ", err)
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
		LOG.Panic("Unmarshal error %s", err)
	}
	INPUTS = make([]string, 1)
	WAITSTRS = make([]string, 1)
	for i := range data.Inputs {
		input := data.Inputs[i]["input"]
		INPUTS = append(INPUTS, input.(string))
		waitStr := data.Inputs[i]["waitStr"]
		if waitStr != nil {
			WAITSTRS = append(WAITSTRS, waitStr.(string))
		} else {
			WAITSTRS = append(WAITSTRS, "")
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// 2 ways of run
// - 1st: mcall web
// 		call from brower: http://localhost:8080/main/core/1418,1419,2502,2694,2932,2933,2695
// - 2nd: mcall core/graphite 1418,1419,2502,2694,2932,2933,2695
//////////////////////////////////////////////////////////////////////////////////////////////////////
func main() {
	if len(os.Args) < 2 {
		fmt.Println("No parameter!")
		return
	}

	////[ argument ]////////////////////////////////////////////////////////////////////////////////
	var (
		help = flag.Bool("help", false, "Show these options")
		vt   = flag.String("t", "cmd", "Type")
		vi   = flag.String("i", "", "input")
		vc   = flag.String("c", "", "configuration file path")
		vw   = flag.Bool("w", false, "run webserver")
		vp   = flag.String("p", "8080", "webserver port")
		//			vf   = flag.String("f", "json", "return format")
		vlf = flag.String("logfile", "/var/log/mcall/mcall.log", "Logfile destination. STDOUT | STDERR or file path")
		vll = flag.String("loglevel", "DEBUG", "Loglevel CRITICAL, ERROR, WARNING, NOTICE, INFO, DEBUG")
	)
	flag.Parse()
	var args = Args{"help": *help, "t": *vt, "i": *vi, "c": *vc, "w": *vw, "vp": *vp, "logfile": *vlf, "loglevel": *vll}
	mainExec(args)
}

type Args map[string]interface{}

func mainExec(args Args) map[string]string {
	var rslt = map[string]string{}
	var (
		help = args["help"]
		vt   = args["t"]
		vi   = args["i"]
		vc   = args["c"]
		vw   = args["w"]
		vp   = args["p"]
		//			vf   = args["f"]
		vlf = args["logfile"]
		vll = args["loglevel"]
	)

	if help == true {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if vt != nil {
		STYPE = vt.(string)
	} else {
		STYPE = "cmd"
	}
	if vi != nil {
		INPUTS = append(INPUTS, vi.(string))
	}
	if vt != nil {
		CONFIGFILE = vc.(string)
	}
	if vw != nil {
		WEBENABLED = vw.(bool)
	}
	if vp != nil {
		HTTPPORT = vp.(string)
	} else {
		HTTPPORT = "8080"
	}
	if vlf != nil {
		logfile = vlf.(string)
	}
	if vll != nil {
		loglevel = vll.(string)
	} else {
		loglevel = "DEBUG"
	}

	////[ configuratin file ]////////////////////////////////////////////////////////////////////////////////
	if CONFIGFILE != "" {
		CFG, err := ini.LoadFile(CONFIGFILE)
		if err != nil {
			fmt.Println("parse config "+CONFIGFILE+" file error: ", err)
		}

		loglevel, _ = CFG.Get("log", "level")
		logfile, _ = CFG.Get("log", "file")

		workerNumber, ok := CFG.Get("worker", "number")
		if !ok {
			fmt.Println("'file' missing from 'worker", "number")
		} else {
			WORKERNUM, _ = strconv.Atoi(workerNumber)
		}

		webEnbleStr, ok := CFG.Get("webserver", "enable")
		if !ok {
			fmt.Println("'enable' missing from 'webserver", "enable")
		} else {
			if webEnbleStr == "false" {
				WEBENABLED = true
			} else {
				WEBENABLED = false
			}
		}

		if WEBENABLED == true {
			httpost, ok := CFG.Get("webserver", "host")
			if !ok {
				fmt.Println("'host' missing from 'webserver", "host")
			} else {
				HTTPHOST = httpost
			}

			httpport, ok := CFG.Get("webserver", "port")
			if !ok {
				fmt.Println("'port' missing from 'webserver", "port")
			} else {
				HTTPPORT = httpport
			}
		} else {
			input, ok := CFG.Get("request", "input")
			if !ok {
				fmt.Println("'input' missing from 'request' section")
			}
			stype, _ := CFG.Get("request", "type")
			if stype != "" {
				STYPE = stype
			}
			getInput(input)
		}
	}

	////[ log file ]////////////////////////////////////////////////////////////////////////////////
	if _, err := os.Stat(logfile); err != nil {
		logfile, _ = os.Getwd()
		logfile = logfile + "/mcall.log"
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
	GLOGLEVEL, err := logging.LogLevel(loglevel)
	if err != nil {
		GLOGLEVEL = logging.DEBUG
	}
	logging.SetBackend(logformatted)
	logging.SetLevel(GLOGLEVEL, "")

	LOG.Debug("workerNumber: ", WORKERNUM)
	LOG.Debug("type: ", STYPE)
	LOG.Debug("webEnabled: ", WEBENABLED)
	LOG.Debug("httphost: ", HTTPHOST)
	LOG.Debug("httpport: ", HTTPPORT)

	////[ run app ]////////////////////////////////////////////////////////////////////////////////
	if WEBENABLED == true {
		webserver()
	} else {
		rslt = execCmd()
	}
	return rslt
}
