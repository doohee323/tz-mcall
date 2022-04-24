package main

import (
	"bytes"
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
	TIMEOUT    = 10
	SUBJECT    string
	INPUTS     []string
	STYPES     []string
	STYPE      string
	NAMES      []string
	NAME       string
	FORMAT     string
	WEBENABLED = false
	BASE64     string
	HTTPHOST   = "localhost"
	HTTPPORT   = "3000"
	ESCONF     ES
)

type ES struct {
	host      string
	id        string
	password  string
	indexName string
}

var (
	LOGFMT    = "%{color}%{time:15:04:05.000000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}"
	LOGFORMAT = logging.MustStringFormatter(LOGFMT)
	LOG       = logging.MustGetLogger("logfile")
	logfile   string
	loglevel  string
)

type FetchedResult struct {
	input   string
	name    string
	err     string
	content string
	ts      string
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
	input        string
	sType        string
	name         string
	result       chan FetchedResult
}

func fetchHtml(input string) (string, error) {
	if input == "" {
		return "", nil
	}
	LOG.Debug("= input: ", input)
	res, err := http.Get(input)
	if err != nil {
		LOG.Error(err)
		return err.Error(), err
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
		return doc, err
	} else {
		LOG.Debug(doc)
	}
	return doc, nil
}

type ResultDoc struct {
	Raw   string `json:"raw"`
	Error string `json:"error"`
}

func exeCmd(str string) (string, error) {
	res := ResultDoc{}
	resultchan := make(chan string)
	errchan := make(chan error, 10)
	parts := strings.Fields(str)
	cmdName := parts[0]
	LOG.Debug("= cmdName: ", cmdName)
	args := parts[1:len(parts)]
	LOG.Debug("= args: ", args)
	// replace "`" to " "
	for n := range args {
		if args[n] == "'Content-Type_application/json'" {
			args[n] = "'Content-Type: application/json'"
		} else {
			args[n] = strings.Replace(args[n], "`", " ", -1)
		}
	}
	cmd := exec.Command(cmdName, args...)
	stdout, err := cmd.StdoutPipe()
	if werr, ok := err.(*exec.ExitError); ok {
		if s := werr.Error(); s != "0" {
			errchan <- err
		}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		LOG.Error("Error: %s", err)
	}
	go func() {
		stdo, err := ioutil.ReadAll(stdout)
		if err != nil {
			errchan <- err
		}
		resultchan <- string(stdo[:])
		stde, f := ioutil.ReadAll(stderr)
		if f != nil {
			LOG.Error(f)
			res.Error = string(stde)
		}
	}()
	err = cmd.Start()
	if err != nil {
		errchan <- err
		res.Error = fmt.Sprintf("Runner: %s", err.Error())
		if res.Error != "" {
			res.Raw = res.Error
		}
		LOG.Debug("= res.Error2: ", res.Error)
		return res.Raw, errors.New(res.Error)
	}

loop:
	for {
		select {
		case <-time.After(time.Duration(TIMEOUT) * time.Second):
			cmd.Process.Kill()
			res.Error = "Runner: timedout"
			res.Raw = res.Error
			LOG.Debug("= res.Error1: ", res.Error)
			break loop
		case err := <-errchan:
			res.Error = fmt.Sprintf("Runner: %s", err.Error())
			if res.Error != "" {
				res.Raw = res.Error
			}
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

func (g *CallFetch) request(input string, sType string, name string) {
	g.p.request <- &CallFetch{
		fetchedInput: g.fetchedInput,
		p:            g.p,
		input:        input,
		sType:        sType,
		name:         name,
		result:       g.result,
	}
}

func (g *CallFetch) parseContent(doc string) <-chan string {
	content := make(chan string)
	go func() {
		content <- doc
		chk := false
		input := ""
		sType := ""
		name := ""
		g.fetchedInput.Lock()
		for n := range INPUTS {
			if _, ok := g.fetchedInput.m[INPUTS[n]]; !ok {
				chk = true
				input = INPUTS[n]
				sType = STYPES[n]
				name = NAMES[n]
				g.request(input, sType, name)
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
		if g.sType == "cmd" {
			doc, err = fetchCmd(g.input)
			//if err != nil {
			//	go func(u string) {
			//		g.request(u, sType)
			//	}(g.input)
			//	return
			//}
		} else {
			doc, err = fetchHtml(g.input)
			//if err != nil {
			//	go func(u string) {
			//		g.request(u, sType)
			//	}(g.input)
			//	return
			//}
		}
	}
	g.fetchedInput.Lock()
	g.fetchedInput.m[g.input] = err
	g.fetchedInput.Unlock()
	content := <-g.parseContent(doc)
	var errCode string
	if err != nil {
		errCode = "-1"
	} else {
		errCode = "0"
	}
	now := time.Now().UTC()
	g.result <- FetchedResult{g.input, g.name, errCode, content, now.Format("2006-01-02T15:04:05.000")}
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

func execCmd() []map[string]string {
	start := time.Now()
	numCPUs := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPUs)

	p := NewPipeline()
	p.Run()

	var sType string
	if len(STYPES) < 1 {
		sType = "cmd"
	} else {
		sType = STYPES[0]
	}
	var name string
	if len(NAMES) < 1 {
		name = NAME
	} else {
		name = NAMES[0]
	}
	call := &CallFetch{
		fetchedInput: &FetchedInput{m: make(map[string]error)},
		p:            p,
		input:        INPUTS[0],
		sType:        sType,
		name:         name,
		result:       make(chan FetchedResult),
	}
	p.request <- call

	var result = make([]map[string]string, 0)
	count := 0
	LOG.Debug("============ len(INPUTS): ", len(INPUTS))
	for a := range call.result {
		count++
		var arry = make(map[string]string)
		if FORMAT == "json" {
			var rslt string
			str, _ := json.Marshal(a.content)
			if BASE64 == "std" {
				rslt = base64.StdEncoding.EncodeToString(str)
			} else if BASE64 == "url" {
				rslt = base64.URLEncoding.EncodeToString(str)
			} else {
				rslt = string(str)
			}
			if SUBJECT != "" {
				arry["subject"] = SUBJECT
			}
			arry["input"] = a.input
			arry["name"] = a.name
			arry["errorCode"] = a.err
			arry["result"] = rslt
			arry["ts"] = a.ts
		} else {
			arry["result"] = a.content
		}
		result = append(result, arry)
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

// http://localhost:3000/mcall/cmd/{"inputs":[{"input":"ls -al"},{"input":"ls"}]}
func getHandle(w http.ResponseWriter, r *http.Request) {
	STYPE = r.URL.Query().Get(":type")
	NAME = r.URL.Query().Get(":name")
	paramStr := r.URL.Query().Get(":params")
	LOG.Debug(STYPE, paramStr)
	getInput(paramStr)
	b := makeResponse()
	w.Write(b)
}

// http://localhost:3000/mcall?type=post&params={"inputs":[{"input":"ls -al"},{"input":"pwd"}]}
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
	NAME = r.FormValue("name")
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
	if FORMAT == "json" {
		b, err := json.Marshal(result)
		if err != nil {
			LOG.Errorf("error: %s", err)
		}
		outStr := string(b)
		fmt.Println(outStr)
		if ESCONF.host != "" {
			//outStr = "{\"values\": " + outStr + "}"
			//outStr = strings.Replace(outStr, "\"\\\"", "\"", -1)
			//outStr = strings.Replace(outStr, "\\\"", "", -1)
			//outStr = strings.Replace(outStr, "\"", "\\\"", -1)
			//esCmd := "curl -XPOST -u '" + adminPassword + "' " + esUrl + "/" + indexName + "/doc -H 'Content-Type: application/json' -d \"" + outStr + "\""

			// make a json
			outStr = "{ \"index\":{} }\n" + outStr[1:len(outStr)-1] + "\n"
			outStr = strings.Replace(outStr, "\"\\\"", "\"", -1)
			outStr = strings.Replace(outStr, "\\\"", "", -1)
			outStr = strings.Replace(outStr, "},{", "}\n{ \"index\":{} }\n{", -1)
			var timestamp = strconv.FormatInt(time.Now().Unix(), 10)
			jsonFile := fmt.Sprintf("/tmp/test_%s.json", timestamp)
			jsFile, err := os.OpenFile(jsonFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				LOG.Fatalf("jsFile error: %s %s", jsFile, err)
			}
			jsFile.WriteString(outStr)
			jsFile.Close()

			// make a shell
			adminPassword := ESCONF.id + ":" + ESCONF.password
			esUrl := ESCONF.host
			indexName := ESCONF.indexName
			esCmd := "curl -XPOST -u '" + adminPassword + "' " + esUrl + "/" + indexName + "/_bulk -H 'Content-Type: application/json' --data-binary @" + jsonFile
			shFile := fmt.Sprintf("/tmp/test_%s.sh", timestamp)
			esFile, err := os.OpenFile(shFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
			if err != nil {
				LOG.Fatalf("esFile error: %s %s", esFile, err)
			}
			esFile.WriteString(esCmd)
			esFile.Close()
			doc, err := exeCmd("bash " + shFile)
			if err != nil {
				LOG.Error(err)
			} else {
				LOG.Debug(doc)
				os.Remove(jsonFile)
				os.Remove(shFile)
			}
		}
		return b
	} else {
		var rslt []string
		for i := range result {
			rslt = append(rslt, "\n")
			rslt = append(rslt, result[i]["result"])
			rslt = append(rslt, "=============================================================")
			rslt = append(rslt, "\n")
		}
		fmt.Println(rslt)
		return []byte("")
	}
}

func PrettyString(str string) (string, error) {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(str), "", "    "); err != nil {
		return "", err
	}
	return prettyJSON.String(), nil
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
		r.Get("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "OK")
		})
		r.Get("/mcall/{type}/{params}", getHandle)
		r.Post("/mcall", postHandle)
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
		err = json.Unmarshal(rawDecodedText, &data)
	}
	if err != nil {
		LOG.Error("Unmarshal error %s", err)
	} else {
		INPUTS = make([]string, 0)
		NAMES = make([]string, 0)
		for i := range data.Inputs {
			if value, exist := data.Inputs[i]["input"]; exist {
				INPUTS = append(INPUTS, value.(string))
			}
			if value, exist := data.Inputs[i]["type"]; exist {
				STYPES = append(STYPES, value.(string))
			}
			if value, exist := data.Inputs[i]["name"]; exist {
				NAMES = append(NAMES, value.(string))
			}
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////
// 2 ways to run
// - 1st: mcall web
// 		call from browser: http://localhost:3000/main/core/1418,1419,2502,2694,2932,2933,2695
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
		vp   = flag.String("p", "3000", "webserver port")
		vf   = flag.String("f", "json", "return format")
		ve   = flag.String("e", "", "return result with encoding")
		vn   = flag.String("n", "", "request name")
		vlf  = flag.String("logfile", "./mcall.log", "Logfile destination. STDOUT | STDERR or file path")
		vll  = flag.String("loglevel", "DEBUG", "Loglevel CRITICAL, ERROR, WARNING, NOTICE, INFO, DEBUG")
	)
	flag.Parse()
	var args = Args{"help": *help, "t": *vt, "i": *vi, "c": *vc, "w": *vw, "p": *vp, "f": *vf, "e": *ve, "n": *vn, "logfile": *vlf, "loglevel": *vll}
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
		vf   = args["f"]
		ve   = args["e"]
		vn   = args["n"]
		vlf  = args["logfile"]
		vll  = args["loglevel"]
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
		HTTPPORT = "3000"
	}
	if vf != nil {
		FORMAT = vf.(string)
	} else {
		FORMAT = "json"
	}
	if ve != nil {
		BASE64 = ve.(string)
	}
	if vn != nil {
		NAME = vn.(string)
	}
	if vlf != nil {
		logfile = vlf.(string)
	} else {
		logfile = "/var/log/mcall/mcall.log"
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
		WEBENABLED = viper.GetBool("webserver.enable")
		FORMAT = viper.GetString("response.format")
		BASE64 = viper.GetString("response.encoding.type")

		ESCONF = ES{viper.GetString("response.es.host"),
			viper.GetString("response.es.id"),
			viper.GetString("response.es.password"),
			viper.GetString("response.es.index_name")}

		if WEBENABLED == true {
			HTTPHOST = viper.GetString("webserver.host")
			HTTPPORT = viper.GetString("webserver.port")
		} else {
			SUBJECT = viper.GetString("request.subject")
			TIMEOUT = viper.GetInt("request.timeout")
			input := viper.GetString("request.input")
			STYPE = viper.GetString("request.type")
			NAME = viper.GetString("request.name")
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

	LOG.Debug("WORKERNUM: ", WORKERNUM)
	LOG.Debug("STYPE: ", STYPE)
	LOG.Debug("WEBENABLED: ", WEBENABLED)
	LOG.Debug("HTTPHOST: ", HTTPHOST)
	LOG.Debug("HTTPPORT: ", HTTPPORT)

	////[ run app ]////////////////////////////////////////////////////////////////////////////////
	var rslt = map[string]string{}
	if WEBENABLED == true {
		webserver()
	} else {
		makeResponse()
	}
	return rslt
}
