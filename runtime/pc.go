package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mkuroda13/CJanus-MPLR25/util"
)

type pc struct {
	head       int   //LineNo of next instruction to be run
	pc         int   //Ctr that increases by 1 every exec
	pid        int   //Display pid, unique integer with 0 begin root process
	treepid    []int //Tree pid, may not be unique if multiple parcall on same proc
	originpid  int   //pid of the caller, focus will fall back to that once terminated
	executing  bool
	nest       int                    //counter of how much indent is nested
	localtable map[string][]localaddr //last one is newest
}

type localaddr struct {
	addr addr
	nest int
}

const (
	HIST_COLON = -1
	HIST_AT    = -2
)

func (p *pc) indent() {
	p.nest += 1
}

func (p *pc) unindent() {
	p.nest -= 1
}

type execresult int

const (
	EXR_NORMAL execresult = iota
	EXR_EOF
	EXR_PROCEND
)

func (p *pc) stringfyTreePid() string {
	s := ""
	for i, a := range p.treepid {
		if i != 0 {
			s += "."
		}
		s += strconv.Itoa(a)
	}
	return s
}

func (p *pc) execute(r *runtime) int {
	//0 means suspended
	//1 means reached EOF
	//2 means terminated
	for {
		select {
		case rev := <-r.runonce:
			//run once mode
			if r.isRunnable(p, rev) && !p.executing {
				//check if dag allows it to run, if yes then execute, send rundone
				ret := p.executeBlock(r, rev)
				r.rundone <- true
				switch ret {
				case 1:
					//EOF
					util.Unclog(r.runclose)
					r.runclose <- true
					return 1
				case 2:
					//End of non main proc
					return 2
				}
			} else {
				//otherwise broadcast msg to another, block until rundone, which will also be broadcasted
				r.runonce <- rev
				<-r.rundoner
				r.rundoner <- true
			}
		case rev := <-r.runever:
			//run forever mode
			//broadcast run msg
			r.runever <- rev
			for {
				select {
				case <-r.runclose:
					r.runclose <- true
					return 0
				default:
					if !SEQUENTIAL {
						exmut.Lock()
					}
					if r.isRunnable(p, rev) {
						//check if dag allows it to run, if yes then execute, send dagupdate if modified dag
						ret := p.executeBlock(r, rev)
						//TODO add if dag actually does update check
						util.Unclog(r.dagupdate) // unclogs buffer so that it will not be blocked
						r.dagupdate <- true
						switch ret {
						case 1:
							//EOF, suspend the execution
							util.Unclog(r.runclose)
							r.runclose <- true
							return 1
						case 2:
							//End of non main proc
							r.termproc <- procchange{p.pid, p.originpid}
							return 2
						}
					} else {
						if !SEQUENTIAL {
							exmut.Unlock()
						}
						<-time.After(1 * time.Microsecond)
					}
				}
			}
		case <-r.runclose:
			r.runclose <- true
			return 0
		}
	}
}

func (p *pc) getinst(r *runtime, inst string, rev bool) (func(), func() int) {
	for _, i := range r.instset {
		match := i.re.FindStringSubmatch(inst)
		if match != nil {
			if rev {
				return i.bwd(r, p, match)
			} else {
				return i.fwd(r, p, match)
			}
		}
	}
	fmt.Printf("Instruction unknown [%s]\n", inst)
	panic("Instruction unknown")
}

func (p *pc) executeBlock(r *runtime, rev bool) execresult {
	//We made sure that block that is executed is next/previous 3 lines, by adding entry/exit that doesn't exist in source
	//head points to next instruction to run in fwd, instruction just ran in bwd
	p.executing = true
	defer func() {
		p.executing = false
	}()
	inst := make([]string, 0, 3)
	insf := make([]func(), 0, 3)
	insa := make([]func() int, 0, 3)
	//TODO temp solution
	conc := false
	rt := EXR_NORMAL
	if rev {
		if p.head-2 < 0 {
			p.head = 0
			return EXR_EOF
		}
		p.pc--
		for i := range 3 {
			inst = append(inst, (*r.file)[p.head-i-1])
			f, a := p.getinst(r, (*r.file)[p.head-i-1], rev)
			insf = append(insf, f)
			insa = append(insa, a)
		}
	} else {
		if len(*(r.file)) <= p.head {
			p.head = len(*(r.file)) - 1
			return EXR_EOF
		}
		for i := range 3 {
			inst = append(inst, (*r.file)[p.head+i])
			f, a := p.getinst(r, (*r.file)[p.head+i], rev)
			insf = append(insf, f)
			insa = append(insa, a)
		}
	}

	for i := range 3 {
		f := insf[i]
		f()
	}

	if SLOW_PARSE {
		m := 0
		for m <= 500000 {
			m++
		}
	}

	if conc && !STRICT {
		r.skiplock++
	}
L1:
	for i := range 3 {
		f := insa[i]
		ret := f()
		switch ret {
		case 0:
			if rev {
				p.head--
			} else {
				p.head++
			}
			continue
		case 1:
			if rev {
				p.head--
			} else {
				p.head++
			}
			rt = EXR_EOF
			break L1
		case 2:
			if rev {
				p.head--
			} else {
				p.head++
			}
			rt = EXR_PROCEND
			break L1
		case 3:
			continue
		case 4:
			continue
		}
	}
	if !SUPPRESSOUTPUT {
		fmt.Print("P", p.pid, ",", p.pc, ">\n", strings.Join(inst, " "), "\n")
	}
	if EXEC_DEBUG {
		if !rev {
			r.exmap[pidandpc{p.pid, p.pc}] = strings.Join(inst, "")
		} else {
			s := ""
			for i := range 3 {
				s += inst[2-i]
			}
			if s != r.exmap[pidandpc{p.pid, p.pc}] {
				panic("Unmatched execution: P" + strconv.Itoa(p.pid) + "," + strconv.Itoa(p.pc) + "\n" +
					"Expected: \n" + r.exmap[pidandpc{p.pid, p.pc}] +
					"\nGot: \n" + s)
			}
		}
	}
	if !rev {
		p.pc++
	}
	if !SEQUENTIAL {
		exmut.Unlock()
	}
	if SLOW_DEBUG != 0 {
		time.Sleep(SLOW_DEBUG)
	}
	return rt
}

//	type eMutex struct{
//		sync.Mutex
//	}
//
//	func (m *eMutex) Lock(){
//		m.Mutex.Lock()
//		print("Locked mutex\n")
//	}
//
//	func (m *eMutex) Unlock(){
//		m.Mutex.Unlock()
//		print("Unlocked mutex\n")
//	}
//
// var exmut *eMutex = &eMutex{}
var exmut *sync.Mutex = &sync.Mutex{}

func (p *pc) getBlock() string {
	return ":" + strconv.Itoa(p.nest)
}

func (r *runtime) PrintProc() {
	bctr := 0
	for _, p := range r.pcs {
		fmt.Printf("(%d,%d) ", p.pid, p.pc)
		bctr += p.pc
	}
	fmt.Print("\n")
}

func checkerr(e error) {
	if e != nil {
		panic(e)
	}
}
