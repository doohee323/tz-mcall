package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gorilla/pat"
	"github.com/op/go-logging"
	"github.com/spf13/viper"
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
	CONFIGFILE string
	WORKERNUM  = 10
	INPUTS     []string
	STYPE      string
	WEBENABLED = false
	HTTPHOST   = "localhost"
	HTTPPORT   = "8080"
)

var (
	LOGFMT    = "%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
	LOGFORMAT = logging.MustStringFormatter(LOGFMT)
	LOG       = logging.MustGetLogger("logfile")
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
}

func fetchHtml(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	LOG.Debug("= input: ", input)
	res, err := http.Get(input)
	if err != nil {
		LOG.Error(err)
		return "", err
	}
	defer res.Body.Close()
	doc, err := ioutil.ReadAll(res.Body)
	if err != nil {
		LOG.Error(err)
		return "", err
	} else {
		LOG.Debug(string(doc))
	}
	return string(doc), nil
}

func fetchCmd(input string) (string, error) {
	if input == "" {
		return "", nil
	}

	LOG.Debug("= input: ", input)
	doc, err := exeCmd(input)
	if err != nil {
		LOG.Error(err)
		return "", err
	} else {
		LOG.Debug(doc)
	}
	return string(doc), nil
}

type ResultDoc struct {
	Raw   string `json:"raw"`
	Error string `json:"error"`
}

func exeCmd(str string) (string, error) {
	res := ResultDoc{}

	//make channels for out or for error
	resultchan := make(chan string)
	errchan := make(chan error, 10)

	parts := strings.Fields(str)
	cmdName := parts[0]
	LOG.Debug("= cmdName: ", cmdName)
	args := parts[1:len(parts)]
	LOG.Debug("= args: ", args)

	// replace "`" to " "
	for n := range args {
		args[n] = strings.Replace(args[n], "`", " ", -1)
	}
	cmd := exec.Command(cmdName, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		LOG.Error("Error: %s", err)
		errchan <- err
	}
	//	stderr, err := cmd.StderrPipe()
	//	if err != nil {
	//		LOG.Error("Error: %s", err)
	//	}

	//receiving command out in this thread
	go func() {
		stdo, err := ioutil.ReadAll(stdout)
		if err != nil {
			errchan <- err
		}
		resultchan <- string(stdo[:])
		//		stde, f := ioutil.ReadAll(stderr)
		//		if f != nil {
		//			LOG.Error(f)
		//			res.Error = string(stde)
		//		}
	}()

	//fireup command non blocking
	err = cmd.Start()
	if err != nil {
		errchan <- err
	}

loop:
	for {
		select {
		case <-time.After(time.Duration(360) * time.Second):
			cmd.Process.Kill()
			res.Error = "Runner: timedout"
			LOG.Debug("= res.Error1: ", res.Error)
			break loop
		case err := <-errchan:
			res.Error = fmt.Sprintf("Runner: %s", err.Error())
			LOG.Debug("= res.Error2: ", res.Error)
			break loop
		case cmdresult := <-resultchan:
			if cmdresult != "" {
				res.Raw = cmdresult
				break loop
			}
			LOG.Debug("= cmdresult: ", cmdresult)
		}
	}

	cmd.Wait()

	if res.Error == "" {
		res.Error = "Runner: OK"
		return res.Raw, nil
	}

	return res.Raw, errors.New(res.Error)
}

func (g *CallFetch) request(input string) {
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
				g.request(val)
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
			doc, err = fetchCmd(g.input)
			if err != nil {
				go func(u string) {
					g.request(u)
				}(g.input)
				return
			}
		} else {
			doc, err = fetchHtml(g.input)
			if err != nil {
				go func(u string) {
					g.request(u)
				}(g.input)
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
		input:        INPUTS[0],
	}
	p.request <- call

	result := make(map[string]string)
	count := 0
	LOG.Debug("============ len(INPUTS): ", len(INPUTS))
	for a := range call.result {
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

// http://localhost:8080/mcall?type=post&params={"inputs":[{"input":"ls -al"},{"input":"pwd"}]}
func postHandle(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		LOG.Error("ParseForm %s", err)
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
	fmt.Printf("%s", string(b))
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
	rawDecodedText, err := base64.StdEncoding.DecodeString(aInput)
	var data Inputs
	if err != nil {
		LOG.Error("base64 error %s", err)
		err = json.Unmarshal([]byte(aInput), &data)
	} else {
		err = json.Unmarshal([]byte(rawDecodedText), &data)
	}
	if err != nil {
		LOG.Error("Unmarshal error %s", err)
	} else {
		INPUTS = make([]string, 1)
		for i := range data.Inputs {
			input := data.Inputs[i]["input"]
			INPUTS = append(INPUTS, input.(string))
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// 2 ways of run
// - 1st: mcall web
// 		call from brower: http://localhost:8080/main/core/1418,1419,2502,2694,2932,2933,2695
// - 2nd: mcall on console
//		mcall -i="ls -al"
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
	if vc != nil {
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
		viper.SetConfigFile(CONFIGFILE)
		viper.SetConfigType("yaml")
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Println("parse config "+CONFIGFILE+" file error: ", err)
		}

		loglevel = viper.GetString("log.level")
		logfile = viper.GetString("log.file")

		WORKERNUM = viper.GetInt("worker.number")
		webEnbleStr := viper.GetString("webserver.enable")
		if webEnbleStr == "true" {
			WEBENABLED = true
		} else {
			WEBENABLED = false
		}

		if WEBENABLED == true {
			HTTPHOST = viper.GetString("webserver.host")
			HTTPPORT = viper.GetString("webserver.port")
		} else {
			input := viper.GetString("request.input")
			STYPE = viper.GetString("request.type")
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
	var rslt = map[string]string{}
	if WEBENABLED == true {
		webserver()
	} else {
		makeResponse()
	}
	return rslt
}
