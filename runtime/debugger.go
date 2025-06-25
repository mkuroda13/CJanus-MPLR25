package main

//go:generate $GOPATH/bin/goyacc -o expr.go -p "expr" expr.y

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/mkuroda13/CJanus-MPLR25/util"
)

// if not 0, add sleep of x microsecond after every instruction
var SLOW_DEBUG time.Duration = 0

// if true, try to group same process together in dag display
var SEQ_DAG = false

// if true, display dag update information to stdout
var DAG_DEBUG = false

// if true, display variable read/write information to stdout
var VAR_DEBUG = true

// if true, display block updates to stdout
var BLOCK_DEBUG = true

// if true, store and compare fwd and bwd execution input and match them
var EXEC_DEBUG = true

// if true, display which process got semaphore with V/P operation
var SEM_DEBUG = true

// if true, time the execution
var TIMING_DEBUG = false

// enables sequential mode
var SEQUENTIAL = false

// stop printing executed block
var SUPPRESSOUTPUT = false

// enable profiling
var PROFILE = false
var PROFILE_FILE *os.File

// enable tracing
var TRACING = false
var TRACING_FILE *os.File

// Simulate slower paring by adding artificial delay before executing instructions
var SLOW_PARSE = false

// Strictly follow basic-block-wise atomicity, even if instruction does not modify variables
var STRICT = false

// do not record dag
var NODAG = false

// Allow expression of arbitary length
var EXTEXPR = false

// Display lock/unlock of addresses
var LOCK_DEBUG = false

func main() {
	VERBOSE := true
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [args] file\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.DurationVar(&SLOW_DEBUG, "s", 0, "Add sleep of specified amount after every instruction. Higher values will result in more alternating execution order.")
	flag.BoolVar(&DAG_DEBUG, "dd", false, "Display Annotation DAG updates to stdout")
	flag.BoolVar(&VAR_DEBUG, "dv", false, "Display variable reads/writes to stdout")
	flag.BoolVar(&BLOCK_DEBUG, "db", false, "Display block changes to stdout")
	flag.BoolVar(&SEM_DEBUG, "ds", false, "Display semaphore update upon V/P operation to stdout")
	flag.BoolVar(&LOCK_DEBUG, "dl", false, "Display lock/unlock of addresses to stdout")
	flag.BoolVar(&VERBOSE, "v", false, "verbose, will enable -dd, -dv, -db, -ds, -dl, and -e")
	flag.BoolVar(&EXEC_DEBUG, "e", false, "If enabled, execution will fail if block executed backwards does not match corresponding forward one")
	flag.BoolVar(&TIMING_DEBUG, "t", false, "If enabled, execution time is measured and displayed when done")
	flag.BoolVar(&SEQUENTIAL, "seq", false, "If enabled, goroutines, and locks will not be used. Call statement can only be called one at a time, errors otherwise")
	flag.BoolVar(&SUPPRESSOUTPUT, "silent", false, "Stops printing executed blocks. Will still print message indicating program is finished")
	flag.BoolVar(&PROFILE, "profile", false, "If enabled, do a CPU profile")
	flag.BoolVar(&TRACING, "trace", false, "If enabled, do tracing")
	flag.BoolVar(&NODAG, "nodag", false, "If enabled, does not record annotation DAG")
	flag.BoolVar(&SLOW_PARSE, "slowparse", false, "Simulate slower parsing by adding artificial delay before executing instructions")
	flag.BoolVar(&STRICT, "strict", false, "Strictly follow basic-block-wise atomicity, even if instruction does not modify variables")
	flag.BoolVar(&EXTEXPR, "extexpr", false, "Allow expression of arbitary length")
	flag.Parse()

	if VERBOSE {
		DAG_DEBUG = true
		VAR_DEBUG = true
		BLOCK_DEBUG = true
		EXEC_DEBUG = true
		SEM_DEBUG = true
		LOCK_DEBUG = true
	}
	if PROFILE {
		util.SetRuntimeBlockProfileRate(1)
		util.SetRuntimeMutexProfileRate(1)
	}
	debug(flag.Arg(0))
}
func debug(filename string) {
	//open the file, eventually replace this with Open()
	file, err := os.Open(filename)
	checkerr(err)
	f := make([]string, 0)
	runtime := newRuntime(&f)
	scanner := bufio.NewScanner(file)
	//set up regex for -> and <- label statements
	goreg := regexp.MustCompile(`^[\w\d$.:\[\]+\-*/!<=>&|%]*\s*->\s*([\w\d$.:]+)\s*(?:;\s*([\w\d$.:]+)\s*)?`)
	comereg := regexp.MustCompile(`^\s*([\w\d$.:]+)\s*(?:;\s*([\w\d$.:]+)\s*)?<-[\w\d$.:\[\]+\-*/!<=>&|%]*`)
	//set up regex for begin label and end label statements
	beginreg := regexp.MustCompile(`^begin\s+([\w\d$.:]+)$`)
	endreg := regexp.MustCompile(`^end\s+([\w\d$.:]+)$`)
	//set up regex for empty lines
	empreg := regexp.MustCompile(`^\s*$`)
	//set up regex for
	lineno := 0
	//scan each line
	for scanner.Scan() {
		t := scanner.Text()
		//skip empty line
		if !empreg.MatchString(t) {
			//if label is found, add them to runtime.labels
			gom := goreg.FindStringSubmatch(t)
			com := comereg.FindStringSubmatch(t)
			if gom != nil {
				for i := 1; i <= 2; i++ {
					if len(gom[i]) != 0 {
						runtime.labels.AddGoto(gom[i], lineno)
					}
				}
			}
			if com != nil {
				for i := 1; i <= 2; i++ {
					if len(com[i]) != 0 {
						runtime.labels.AddComeFrom(com[i], lineno)
					}
				}
			}
			//same for begin/end
			bgm := beginreg.FindStringSubmatch(t)
			enm := endreg.FindStringSubmatch(t)
			if bgm != nil {
				runtime.labels.AddBegin(bgm[1], lineno)
			}
			if enm != nil {
				runtime.labels.AddEnd(enm[1], lineno)
			}
			if lineno%3 == 1 && (enm != nil || gom != nil) {
				f = append(f, "skip")
				lineno++
			}
			if lineno%3 == 2 && enm == nil && gom == nil {
				//if inst
				s := runtime.RegisterNewLabel(lineno)
				f = append(f, "-> "+s, s+" <-")
				lineno += 2
			}
			//append the whole line (regardless of regex) to f
			f = append(f, t)
			lineno++
		}
	}
	//fmt.Print(f)
	runtime.initInstset()
	debugsym := make(chan string)
	done := make(chan bool)

	go handlesig(done)
	go readstdin(debugsym, done)
	go runtime.debug(debugsym, done)
	<-done
}
func readstdin(debugsym chan<- string, done <-chan bool) {
	in := bufio.NewReader(os.Stdin)
	for {
		select {
		case <-done:
			return
		default:
			r, err := in.ReadString('\n')
			checkerr(err)
			debugsym <- r
		}
	}
}

func handlesig(done chan bool) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-done:
			return
		case sig := <-sigs:
			fmt.Print("\n", sig, "\n")
			close(done)
			return
		}
	}
}
