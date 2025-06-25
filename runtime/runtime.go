package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime/pprof"
	"runtime/trace"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mkuroda13/CJanus-MPLR25/util"
)

type runtime struct {
	labels
	symtab
	rev        bool
	instset    []instentry
	file       *[]string
	pcs        []*pc
	dag        dag
	runonce    chan bool
	rundone    chan bool
	rundoner   chan bool
	runever    chan bool
	runclose   chan bool
	dagupdate  chan bool
	newproc    chan int
	termproc   chan procchange
	exmap      map[pidandpc]string
	pidmap     map[string]int
	start_time time.Time
	skiplock   int
}

type procchange struct {
	term   int
	origin int
}

type instentry struct {
	re  *regexp.Regexp
	fwd func(*runtime, *pc, []string) (func(), func() int)
	bwd func(*runtime, *pc, []string) (func(), func() int)
}

func (r *runtime) getChildPC(parent *pc, childid int) *pc {
	a := make([]int, 0)
	a = append(a, parent.treepid...)
	a = append(a, childid)
	s := fmt.Sprint(a)
	p, ex := r.pidmap[s]
	if ex {
		return r.pcs[p]
	}
	npc := &pc{}
	r.pidmap[s] = len(r.pcs)
	r.pcs = append(r.pcs, npc)
	npc.originpid = parent.pid
	npc.pid = len(r.pcs) - 1
	npc.treepid = a
	npc.localtable = make(map[string][]localaddr)
	return npc
}

func (r *runtime) runPC(head int) {
	pid, ex := r.pidmap["[]"]
	if !ex {
		npc := &pc{}
		r.pidmap["[]"] = 0
		r.pcs = append(r.pcs, npc)
		npc.originpid = 0
		npc.pid = 0
		npc.treepid = make([]int, 0)
		npc.localtable = make(map[string][]localaddr)
	}
	r.pcs[pid].head = head
	r.newproc <- pid
	go r.pcs[pid].execute(r)
}

func (r *runtime) runPCCall(parent *pc, childid int, head int, group *sync.WaitGroup, suspended chan bool) {
	p := r.getChildPC(parent, childid)
	p.head = head
	r.newproc <- p.pid
	go func() {
		defer group.Done()
		a := p.execute(r)
		if a == 0 {
			suspended <- true
		}
	}()
}

func (r *runtime) getPC(pid int) (p *pc, _ bool) {
	if len(r.pcs) <= pid || pid < 0 {
		return nil, false
	}
	p = r.pcs[pid]
	return p, p != nil
}

//Generic debug rule
//focus is a pid selected by debugger
//
//in backwards-step-by-step, non-focus process will be reversed as much as possible until the only thing that can be reversed is the focus proc
//in backwards normal execution, all process can be reversed in any order allowed by dag
//when node is removed on normal dag, it will be marked as inactive
//
//in forwards-step-by-step, non-focus process will be ran as much as possible until the only thing that can be run is the focus proc, excluding non-annotated region
//dag evaluation will be done such that if they were to be run, they will not create any outgoing edges for any other inactive nodes
//-> 1. check the node's incoming edges, if any of the connected node is inactive, it cannot run
//-> if node's pc is less than actual procs pc, then that node is active, otherwise inactive
//if focus process is outside of dag, non-focus process will halt
//in forwards normal execution, all process can be executed in any order allowed by dag, including those who is not annotated

//channels/sync stuff. all channels that broadcast will be queued to not block caller
//USED IN STEP BY STEP
//runonce (chan bool): broadcasted from debug, true=bwd direction, reciever will evaluate if they are both in dag & runnable, if yes then run, otherwise block until rundone.
//rundone (chan bool): send from successfully reversed proc, receiver(debug) will treat it as one step done and send rundoner
//rundoner (chan bool): broadcasted from debug, proc will return to halt
//USED IN NORMAL RUN
//runever (chan bool): broadcasted from debug, receiver will enter run mode where they evaluate if its in dag, if not, run normally until halt
//if in dag, evaluate if next is runnable, if yes then run, otherwise block until dagupdate
//needs unclog before runclose, otherwise procs will respawn
//dagupdate (chan bool): broadcasted from proc when dag is modified such that other procs have chance to run again
//(all bwd instruction & fwd instruction that is in dag (thus make inactive node active again, to help with "any of the connected node is inactive then no run" restriction))
//runclose (chan bool): broadcasted from debug when stopped, or from proc when EOF. will terminate all procs. however, it does not delete proc's info such as pid, pc, head
//needs unclog before runever, otherwise procs will immidiately shut off
//OTHER
//newproc (chan): pid of newly created pc. debug's focus will change to that if it matches with target focus, disgarded by debug otherwise
//termproc (chan): conbi of terminating pc's pid and it's origin. debug's focus will change to origin if pid matches with focus, disgarded by debug otherwise

func (r *runtime) debug(debugsym <-chan string, done <-chan bool) {
	//runmode
	//0 means stopped
	//1 means executing
	//2 means awaiting for step by step excution to end
	if PROFILE {
		PROFILE_FILE, _ = os.Create("a.prof")
		fmt.Print("Debug >> Profiling started\n")
		pprof.StartCPUProfile(PROFILE_FILE)
	}
	if TRACING {
		TRACING_FILE, _ = os.Create("trace.out")
		fmt.Print("Debug >> Tracing started\n")
		trace.Start(TRACING_FILE)
	}
	if TIMING_DEBUG {
		r.start_time = time.Now()
	}
	reg := regexp.MustCompile(`focus\s+(\d+)`)
	focus_tgt := 0
	focus_now := 0
	r.runPC(r.GetBegin("main"))
	running := 1
	r.runever <- false
	stepexret := make(chan execresult, 1)
	for {
		switch running {
		case 1:
			select {
			case <-done:
				return
			//set running to false and await further debugsym
			case <-r.runclose:
				//broadcast
				r.runclose <- true
				running = 0
				fmt.Print("Debug >> Execution finished\n")
				if TIMING_DEBUG {
					end_time := time.Now()
					elapsed := end_time.Sub(r.start_time)
					fmt.Print("========Timing Info========\n")
					fmt.Printf("Begin: %s\n", r.start_time)
					fmt.Printf("End: %s\n", end_time)
					fmt.Printf("Elapsed: %s\n", elapsed)
				}
				if PROFILE {
					pprof.StopCPUProfile()
				}
				if TRACING {
					trace.Stop()
				}
			//stop the execution midway
			case s := <-debugsym:
				switch s {
				case "\n":
					util.Unclog(r.runclose)
					util.Unclog(r.runever)
					r.runclose <- true
					running = 0
					fmt.Print("Debug >> Execution suspended by user\n")
				default:
					fmt.Print("Debug >> Unknown command\n")
				}
			case pid := <-r.newproc:
				if pid == focus_tgt && pid != focus_now {
					focus_now = pid
					//temp disabled
					//fmt.Printf("Debug >> Focus change -> %d\n", focus_now)
				}
			case c := <-r.termproc:
				if c.term == focus_now {
					focus_now = c.origin
					//fmt.Printf("Debug >> Focus change %d -> %d\n", c.term, c.origin)
				}
			}
		case 2:
			//step by step
		L1:
			select {
			case <-done:
				return
			case <-stepexret:
				running = 0
			case <-r.rundone:
				//broadcast rundoner
				util.Unclog(r.rundoner)
				r.rundoner <- true
				tgtpc, _ := r.getPC(focus_now)
				if r.isRunnable(tgtpc, r.rev) && !tgtpc.executing {
					//if reversible, do it
					p, ex := r.getPC(focus_now)
					util.Unclog(stepexret)
					if ex {
						go func() {
							stepexret <- p.executeBlock(r, r.rev)
						}()
					} else {
						panic("Focused PC doesnt exist")
					}
					select {
					case <-stepexret:
						running = 0
						break L1

					case pid := <-r.newproc:
						//if newproc is created first, its a call method, so another runonce is needed
						//maybe a dirty solution
						//resend pid change for later
						//no block
						go func() { r.newproc <- pid }()
					}
				}
				//otherwise run another ones once
				util.Unclog(r.runonce)
				r.runonce <- r.rev
			case pid := <-r.newproc:
				if pid == focus_tgt && pid != focus_now {
					focus_now = pid
					fmt.Printf("Debug >> Focus change -> %d\n", focus_now)
				}
			case c := <-r.termproc:
				if c.term == focus_now {
					focus_now = c.origin
					fmt.Printf("Debug >> Focus change %d -> %d\n", c.term, c.origin)
				}
			}
		case 0:
			//if not running
			select {
			case <-done:
				return
			case pid := <-r.newproc:
				if pid == focus_tgt {
					focus_now = pid
				}
			case c := <-r.termproc:
				if c.term == focus_now {
					focus_now = c.origin
				}
			case s := <-debugsym:
				switch s {
				case "\n":
					//run one step of focused proc
					util.Unclog(r.runclose)
					util.Unclog(r.runever)
					util.Unclog(r.rundone)
					running = 2
					//r.addrunPC(0,0,0) //only need to revive main pc cuz all its forks will be revived
					r.rundone <- true
				case "run\n":
					//enter run mode
					if PROFILE {
						PROFILE_FILE, _ = os.Create("a.prof")
						fmt.Print("Debug >> Profiling started\n")
						pprof.StartCPUProfile(PROFILE_FILE)
					}
					if TRACING {
						TRACING_FILE, _ = os.Create("trace.out")
						fmt.Print("Debug >> Tracing started\n")
						trace.Start(TRACING_FILE)
					}
					if TIMING_DEBUG {
						r.start_time = time.Now()
					}
					util.Unclog(r.runclose)
					util.Unclog(r.runever)
					r.runever <- r.rev
					running = 1
					st := r.GetBegin("main")
					if r.rev {
						st = r.GetEnd("main") + 1
					}
					r.runPC(st) //only need to revive main pc cuz all its forks will be revived, given variables will not used anyways
					fmt.Print("Debug >> Running...\n")
				case "fwd\n":
					r.rev = false
					fmt.Print("Debug >> Forward mode set\n")
				case "bwd\n":
					r.rev = true
					fmt.Print("Debug >> Backward mode set\n")
				case "var\n":
					r.PrintSym()
				case "dag\n":
					r.PrintDag(false)
				case "proc\n":
					r.PrintProc()
				case "deldag\n":
					r.dag = *newDag()
					fmt.Print("Debug >> Annotation DAG deleted\n")
				default:
					//set focus
					match := reg.FindStringSubmatch(s)
					if match != nil {
						i, er := strconv.ParseInt(match[1], 0, 0)
						checkerr(er)
						focus_tgt = int(i)
						fmt.Printf("Debug >> Focus changed to %d\n", i)
					} else {
						fmt.Print("Debug >> Unknown command\n")
					}
				}
			}
		}
	}
}

// DO NOT BLOCK
func newRuntime(file *[]string) *runtime {
	return &runtime{
		rev:     false,
		labels:  *newLabels(),
		symtab:  *newSymtab(),
		instset: make([]instentry, 0),
		file:    file, dag: *newDag(),
		runonce:   make(chan bool, 1),
		rundone:   make(chan bool, 1),
		rundoner:  make(chan bool, 1),
		runever:   make(chan bool, 8),
		runclose:  make(chan bool, 8),
		dagupdate: make(chan bool, 8),
		newproc:   make(chan int, 8),
		termproc:  make(chan procchange, 8),
		exmap:     make(map[pidandpc]string),
		pidmap:    make(map[string]int),
	}
}

func (r *runtime) addrun(re string, fwd func(*runtime, *pc, []string) (func(), func() int), bwd func(*runtime, *pc, []string) (func(), func() int)) {
	reg, _ := regexp.Compile(re)
	r.instset = append(r.instset, instentry{reg, fwd, bwd})
}

func (r *runtime) initInstset() {
	// x += a
	r.addrun(`^([\w\d$#.:\[\]]+)\s*([+\-^])=\s*([\w\d$#.:\[\]+\-*/!<=>&|%\(\) ]+)\s*$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		var i func() int
		var d addr
		var df func()
		var f func() int
		expr := arg[3]
		dest := arg[1]
		switch arg[2] {
		case "+":
			f = func() int {
				r.WLockAddr(p, d)
				v := r.ReadAddr(d, p)
				r.WriteAddr(d, v+i(), p)
				r.WUnlockAddr(p, d)
				df()
				return 0
			}
		case "-":
			f = func() int {
				r.WLockAddr(p, d)
				v := r.ReadAddr(d, p)
				r.WriteAddr(d, v-i(), p)
				r.WUnlockAddr(p, d)
				df()
				return 0
			}
		case "^":
			f = func() int {
				r.WLockAddr(p, d)
				v := r.ReadAddr(d, p)
				r.WriteAddr(d, v^i(), p)
				r.WUnlockAddr(p, d)
				df()
				return 0
			}
		}
		return func() {
			i, _ = EvalExpr(expr, r, p)
			d, df = r.GetAddrOfAllocable(dest, p)
		}, f
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			var i func() int
			var d addr
			var df func()
			var f func() int
			expr := arg[3]
			dest := arg[1]
			switch arg[2] {
			case "+":
				f = func() int {
					r.WLockAddr(p, d)
					v := r.ReadAddr(d, p)
					r.WriteAddr(d, v-i(), p)
					r.WUnlockAddr(p, d)
					df()
					return 0
				}
			case "-":
				f = func() int {
					r.WLockAddr(p, d)
					v := r.ReadAddr(d, p)
					r.WriteAddr(d, v+i(), p)
					r.WUnlockAddr(p, d)
					df()
					return 0
				}
			case "^":
				f = func() int {
					r.WLockAddr(p, d)
					v := r.ReadAddr(d, p)
					r.WriteAddr(d, v^i(), p)
					r.WUnlockAddr(p, d)
					df()
					return 0
				}
			}
			return func() {
				i, _ = EvalExpr(expr, r, p)
				d, df = r.GetAddrOfAllocable(dest, p)
			}, f
		})
	//print x
	r.addrun(`^print\s+([\w\d$#.:\[\]]+)$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {},
			func() int {
				v := r.ReadSym(arg[1], p)
				fmt.Printf(">>out: %s %d\n", arg[1], v)
				return 0
			}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//how do i undo print lol
			//use curses?
			return func() {},
				func() int {
					v := r.ReadSym(arg[1], p)
					fmt.Printf("<<out: %s %d\n", arg[1], v)
					return 0
				}
		})
	//skip
	r.addrun(`^skip`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			return 0
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			return func() {}, func() int {
				return 0
			}
		})
	//-> L
	r.addrun(`^->\s*([\w\d$#.:]+)$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			p.head = r.labels.GetComeFrom(arg[1])
			return 4
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			return func() {}, func() int {
				return 0
			}
		})
	//L <-
	r.addrun(`^([\w\d$#.:]+)\s*<-$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			return 0
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			return func() {}, func() int {
				//bwd
				p.head = r.labels.GetGoto(arg[1]) + 1
				return 4
			}
		})
	//a <=> b
	//TODO: REWRITE LATER
	r.addrun(`^([\w\d$#.:\[\]]+)\s*<=>\s*([\w\d$#.:\[\]]+)$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			a := r.ReadSym(arg[1], p)
			b := r.ReadSym(arg[2], p)
			r.WriteSym(arg[1], b, p)
			r.WriteSym(arg[2], a, p)
			return 0
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			return func() {}, func() int {
				a := r.ReadSym(arg[1], p)
				b := r.ReadSym(arg[2], p)
				r.WriteSym(arg[1], b, p)
				r.WriteSym(arg[2], a, p)
				return 0
			}
		})
	//a == b -> L1;L2
	r.addrun(`^([\w\d$#.:\[\]+\-*/!<=>&|%\(\) ]+)\s*->\s*([\w\d$#.:]+)\s*;\s*([\w\d$#.:]+)\s*$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			i, _ := EvalExpr(arg[1], r, p)
			if i() != 0 {
				p.head = r.labels.GetComeFrom(arg[2])
			} else {
				p.head = r.labels.GetComeFrom(arg[3])
			}
			return 4
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			return func() {}, func() int {
				i, _ := EvalExpr(arg[1], r, p)
				i()
				return 0
			}
		})
	r.addrun(`^([\w\d$#.:]+)\s*;\s*([\w\d$#.:]+)\s*<-\s*([\w\d$#.:\[\]+\-*/!<=>&|%\(\) ]+)$`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			i, _ := EvalExpr(arg[3], r, p)
			i()
			return 0
		}
	},
		func(r *runtime, p *pc, arg []string) (func(), func() int) {
			//bwd
			return func() {}, func() int {
				i, _ := EvalExpr(arg[3], r, p)
				if i() != 0 {
					p.head = r.labels.GetGoto(arg[1]) + 1
				} else {
					p.head = r.labels.GetGoto(arg[2]) + 1
				}
				return 4
			}
		})
	r.addrun(`^begin\s+([\w\d]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			if !SEQUENTIAL && p.nest != 0 {
				panic("Process started at non-zero nest depth")
			}
			return 0
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			if !SEQUENTIAL && p.nest != 0 {
				panic("Process ended at non-zero nest depth")
			}
			if arg[1] == "main" {
				return 1
			}
			return 2
		}
	})
	r.addrun(`^end\s+([\w\d$#.:]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			if !SEQUENTIAL && p.nest != 0 {
				panic("Process ended at non-zero nest depth")
			}
			if arg[1] == "main" {
				return 1
			}
			return 2
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			if !SEQUENTIAL && p.nest != 0 {
				panic("Process started at non-zero nest depth")
			}
			return 0
		}
	})
	r.addrun(`^call\s+(.*)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//arg[1] is l(,l)* and must be splited with "," and trim whitespace
		//pid is assigned in order written in source
		//fwd
		return func() {}, func() int {
			procnames := strings.Split(arg[1], ",")
			if SEQUENTIAL {
				if len(procnames) >= 2 {
					panic("Sequential mode only allows one call")
				}
				head := r.GetBegin(strings.TrimSpace(procnames[0]))
				lasthead := p.head
				p.head = head
				rt := p.execute(r)
				p.head = lasthead
				if rt == 0 {
					return 3
				}
				return 0

			} else {
				var wg sync.WaitGroup
				suspendproc := make(chan bool, len(procnames)) //do not block
				for i, v := range procnames {
					wg.Add(1)
					v = strings.TrimSpace(v)
					head := r.GetBegin(v)
					//head will not be overwritten if the proc does exist
					r.runPCCall(p, i, head, &wg, suspendproc)
				}
				exmut.Unlock()
				//we do not need to consider getting proc reversed while waiting
				//upon getting reversed, proc of pid 0 will run execute() first, which will run this again if its forking for another procs, waking up other process
				wg.Wait()
				exmut.Lock()
				select {
				case <-suspendproc:
					//if proc is suspended, it needs to respawn hence this statement nedds to run again
					//giving value 3 so that executeOnce will not modify head
					return 3
				default:
					return 0
				}
			}
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			procnames := strings.Split(arg[1], ",")
			if SEQUENTIAL {
				if len(procnames) >= 2 {
					panic("Sequential mode only allows one call")
				}
				head := r.GetEnd(strings.TrimSpace(procnames[0]))
				lasthead := p.head
				p.head = head + 1
				rt := p.execute(r)
				p.head = lasthead
				if rt == 0 {
					return 3
				}
				return 0

			} else {
				var wg sync.WaitGroup
				suspendproc := make(chan bool, len(procnames))
				for i, v := range procnames {
					wg.Add(1)
					v = strings.TrimSpace(v)
					head := r.GetEnd(v) + 1
					r.runPCCall(p, i, head, &wg, suspendproc)
				}
				exmut.Unlock()
				//we do not need to consider getting proc reversed while waiting
				wg.Wait()
				exmut.Lock()
				select {
				case <-suspendproc:
					//if proc is suspended, it needs to respawn hence this statement nedds to run again
					//giving value 3 so that executeOnce will not modify head
					return 3
				default:
					return 0
				}
			}
		}
	})
	r.addrun(`^push`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			if BLOCK_DEBUG {
				fmt.Print("bin" + p.getBlock() + "f\n")
			}
			p.indent()
			if BLOCK_DEBUG {
				fmt.Print("bou" + p.getBlock() + "f\n")
			}
			return 0
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			if BLOCK_DEBUG {
				fmt.Print("bin" + p.getBlock() + "f\n")
			}
			p.unindent()
			if BLOCK_DEBUG {
				fmt.Print("bou" + p.getBlock() + "f\n")
			}
			return 0
		}
	})
	r.addrun(`^pop`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			if BLOCK_DEBUG {
				fmt.Print("bin" + p.getBlock() + "f\n")
			}
			p.unindent()
			if BLOCK_DEBUG {
				fmt.Print("bou" + p.getBlock() + "f\n")
			}
			return 0
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			if BLOCK_DEBUG {
				fmt.Print("bin" + p.getBlock() + "f\n")
			}
			p.indent()
			if BLOCK_DEBUG {
				fmt.Print("bou" + p.getBlock() + "f\n")
			}
			return 0
		}
	})
	r.addrun(`^set\s+([\w\d$#.:\[\]\@]+)\s+([\w\d$#.:\[\]]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			r.SetSym(arg[1], arg[2], p)
			return 0
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			r.UnsetSym(arg[1], arg[2], p)
			return 0
		}
	})
	r.addrun(`^unset\s+([\w\d$#.:\[\]\@]+)\s+([\w\d$#.:\[\]]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {}, func() int {
			r.UnsetSym(arg[1], arg[2], p)
			return 0
		}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {}, func() int {
			r.SetSym(arg[1], arg[2], p)
			return 0
		}
	})
	r.addrun(`^V\s+([\w\d$#.:\[\]]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		a := r.GetAddr(arg[1], p)
		return func() {
			}, func() int {
				r.WaitV(p, a)
				r.WLockAddr(p, a)
				r.WriteAddr(a, 1, p)
				r.WUnlockAddr(p, a)
				return 0
			}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		a := r.GetAddr(arg[1], p)
		return func() {
			}, func() int {
				r.WaitP(p, a)
				r.WLockAddr(p, a)
				r.WriteAddr(a, 0, p)
				r.WUnlockAddr(p, a)
				return 0
			}
	})
	r.addrun(`^P\s+([\w\d$#.:\[\]]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		a := r.GetAddr(arg[1], p)
		return func() {
			}, func() int {
				r.WaitP(p, a)
				r.WLockAddr(p, a)
				r.WriteAddr(a, 0, p)
				r.WUnlockAddr(p, a)
				return 0
			}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		a := r.GetAddr(arg[1], p)
		return func() {
			}, func() int {
				r.WaitV(p, a)
				r.WLockAddr(p, a)
				r.WriteAddr(a, 1, p)
				r.WUnlockAddr(p, a)
				return 0
			}
	})
	r.addrun(`^lock\s+([\w\d\s.:\[\],]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {
				vars := strings.Split(arg[1], ",")
				for _, v := range vars {
					v = strings.TrimSpace(v)
					if v != "" {
						a := r.GetAddr(v, p)
						r.ReadAddr(a, p)
						r.LockAddr(p, a)
					}
				}
			}, func() int {
				return 0
			}

	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {
				vars := strings.Split(arg[1], ",")
				for _, v := range vars {
					v = strings.TrimSpace(v)
					if v != "" {
						a := r.GetAddr(v, p)
						r.ReadAddr(a, p)
						r.UnlockAddr(p, a)
					}
				}
			}, func() int {
				return 0
			}
	})
	r.addrun(`^unlock\s+([\w\d\s.:\[\],]+)`, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//fwd
		return func() {
				vars := strings.Split(arg[1], ",")
				for _, v := range vars {
					v = strings.TrimSpace(v)
					if v != "" {
						a := r.GetAddr(v, p)
						r.ReadAddr(a, p)
						r.UnlockAddr(p, a)
					}
				}
			}, func() int {
				return 0
			}
	}, func(r *runtime, p *pc, arg []string) (func(), func() int) {
		//bwd
		return func() {
				vars := strings.Split(arg[1], ",")
				for _, v := range vars {
					v = strings.TrimSpace(v)
					if v != "" {
						a := r.GetAddr(v, p)
						r.ReadAddr(a, p)
						r.LockAddr(p, a)
					}
				}
			}, func() int {
				return 0
			}
	})
}
