package main

import (
	"fmt"
)

type dag struct{
	edges map[dagentry]*dagedges
	last map[string]dagentry
}
type dagentry struct{
	pid int
	pc int
}
type dagedges struct{
	inedges []dagedge
	outedges []dagedge
	lock int
}
type dagedge struct {
	pid     int
	pc      int
	wt      bool
	sym string
}
func (e *dagedges) appendin(pid int, pc int, wt bool, sym string){
	e.inedges = append(e.inedges, dagedge{pid,pc,wt,sym})
}
func (e *dagedges) appendout(pid int, pc int, wt bool, sym string){
	e.outedges = append(e.outedges, dagedge{pid,pc,wt,sym})
}
func (r *runtime) addEdge(pid int, pc int, addr addr, wt bool, dstpid int, dstpc int){
	if pid == dstpid && pc > dstpc {
		panic("It should not happen")
	}
	if pid == dstpid && pc == dstpc {
		return
	}
	for _,e := range r.getDagEdges(dstpid,dstpc).inedges {
		if e.pid == pid && e.pc == pc && e.sym == addr.toSymName() && e.wt == wt{
			return
		}
	}
	r.getDagEdges(dstpid,dstpc).appendin(pid,pc,wt,addr.toSymName())
	r.getDagEdges(pid,pc).appendout(dstpid,dstpc,wt,addr.toSymName())
	if DAG_DEBUG{
		fmt.Printf("    Dag added [%d,%d]-(%s(%s),%s)->[%d,%d]\n", pid, pc, addr.toSymName(), addr.toString(), func() string {
			if wt {
				return "wt"
			}
			return "rd"
		}(), dstpid, dstpc)
	}
}

func (r *runtime) getDagEdges(pid int, pc int) *dagedges{
	e, ex := r.dag.edges[dagentry{pid,pc}]
	if !ex{
		e = &dagedges{make([]dagedge, 0),make([]dagedge, 0),0}
		r.dag.edges[dagentry{pid,pc}] = e
	}
	return e
}
func (r *runtime) isDagNodeActive(pid int, pc int) bool {
	p,ex := r.getPC(pid)
	if !ex {
		//BOT is treated as always active
		return pid == -1		
	}
	return p.pc > pc
}

type pidandpc struct {
	pid int
	pc  int
}
func (r *runtime) LockDagNode(pid int, pc int) bool {
	if pid == -1 && pc == -1 {
		return true
	}
	e := r.getDagEdges(pid,pc)
	if e.lock < 0 {
		return false
	}
	e.lock += 1
	if LOCK_DEBUG {
		fmt.Printf("RLock: (%d,%d)\n",pid,pc)
	}
	return true
}

func (r *runtime) PrevNode(pid int,pc int, a addr) (int,int){
	e := r.getDagEdges(pid,pc)
	for _,edge := range e.inedges{
		if edge.wt && edge.sym == a.toSymName(){
			return edge.pid, edge.pc
		}
	}
	return -1,-1
}

func (r *runtime) UnlockDagNode(pid int, pc int) bool {
	if pid == -1 && pc == -1 {
		return true
	}
	e := r.getDagEdges(pid,pc)
	if e.lock <= 0{
		panic(fmt.Sprintf("Unlock of already unlocked node (%d,%d)",pid,pc))
	}
	e.lock--
	if LOCK_DEBUG {
		fmt.Printf("RUnlock: (%d,%d)\n",pid,pc)
	}
	return true
}

func (r *runtime) WLockDagNode(pid int, pc int) bool {
	if pid == -1 && pc == -1 {
		return true
	}
	e := r.getDagEdges(pid,pc)
	if e.lock != 0 {
		return false
	}
	e.lock = -1
	if LOCK_DEBUG {
		fmt.Printf("WLock: (%d,%d)\n",pid,pc)
	}
	return true
}

func (r *runtime) WUnlockDagNode(pid int, pc int) bool {
	if pid == -1 && pc == -1 {
		return true
	}
	e := r.getDagEdges(pid,pc)
	if e.lock >= 0{
		panic(fmt.Sprintf("Unlock of already unlocked node (%d,%d)",pid,pc))
	}
	e.lock++
	if LOCK_DEBUG {
		fmt.Printf("WUnlock: (%d,%d)\n",pid,pc)
	}
	return true
}

func newDag() *dag {
	return &dag{edges: make(map[dagentry]*dagedges), last: make(map[string]dagentry)}
}



func (r *runtime) isRunnable(p *pc, rev bool) bool {
	if NODAG {
		return true
	}
	if rev {
		//no outgoing edges
		//ITS REVERSE SO NEXT EXECUTED TARGET IS PC-1 NOT PC
		for _,e := range r.getDagEdges(p.pid,p.pc-1).outedges {
			//every outgoing nodes must be inactive 
			if r.isDagNodeActive(e.pid,e.pc) {
				return false
			}
		}
		for _,e := range r.getDagEdges(p.pid,p.pc-1).inedges {
			//every incoming read nodes must not have outgoing write edges of the same variable
			if !e.wt{
				for _,e1 := range r.getDagEdges(e.pid,e.pc).outedges {
					if e.sym == e1.sym && r.isDagNodeActive(e1.pid,e1.pc) && !(p.pid == e1.pid && p.pc-1 == e1.pc) && e1.wt {
						return false
					}
				}
			}
		}
	} else {
		for _,e := range r.getDagEdges(p.pid,p.pc).inedges {
			//every incoming nodes must be active 
			if !r.isDagNodeActive(e.pid,e.pc) {
				return false
			}
			//and if edge is write, every read edges of the same varname from that incoming node must be finished
			if e.wt {
				for _,e1 := range r.getDagEdges(e.pid,e.pc).outedges {
					//if outgoing edge is leading to original node, we can allow it
					if e.sym == e1.sym && !r.isDagNodeActive(e1.pid,e1.pc) && !(p.pid == e1.pid && p.pc == e1.pc) {
						return false
					}
				}
			}
		}
	}
	return true
}
func (r *runtime) PrintDag(animate bool) {
	for k,v := range r.dag.edges{
		for _,i := range v.inedges{
			fmt.Printf("[%d,%d]-(%s,%s)-> ",i.pid,i.pc,i.sym,func(a bool)string{
				if a {
					return "wt"
				}
				return "rd"
			}(i.wt))
		}
		fmt.Print("\n")
		fmt.Printf("[[%d,%d]]\n",k.pid,k.pc)
		for _,i := range v.outedges{
			fmt.Printf("-(%s,%s)->[%d,%d] ",i.sym,func(a bool)string{
				if a {
					return "wt"
				}
				return "rd"
			}(i.wt),i.pid,i.pc)
		}
		fmt.Print("\n\n")
	}
}
// func (r *runtime) PrintDag(animate bool) {
// 	fmt.Print("Constructing graph...\n")
// 	files := make([]*image.Paletted, 0, len(r.dag.exechistory))
// 	//construct fill dag image first
// 	graph, _ := r.dag.gviz.Graph()
// 	if SEQ_DAG {
// 		graph.SetRankSeparator(0.02)
// 	}
// 	subgraphs := make([]*cgraph.Graph, 0)
// 	bot, _ := graph.CreateNodeByName("BOT")
// 	if SEQ_DAG {
// 		for pid := range r.pcs {
// 			sg,_ := graph.CreateSubGraphByName("P"+strconv.Itoa(pid))
// 			sg.SetBackgroundColor("gray")
// 			sg.SetStyle(cgraph.SolidGraphStyle)
// 			subgraphs = append(subgraphs, sg)
// 		}
// 	}
// 	for pid, v := range r.dag.incedges {
// 		for pc, edges := range v {
// 			p, ex := r.getPC(pid)
// 			c := 0
// 			if pid < len(r.dag.outedgectr) {
// 				if pc < len(r.dag.outedgectr[pid]) {
// 					c = r.dag.outedgectr[pid][pc]
// 				}
// 			}
// 			if len(edges) != 0 || c != 0 {
// 				var gnode *cgraph.Node
// 				if SEQ_DAG {
// 					gnode, _ = subgraphs[pid].NodeByName("(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc) + ")")
// 				} else {
// 					gnode, _ = graph.NodeByName("(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc) + ")")
// 				}
// 				if gnode == nil {
// 					if SEQ_DAG {
// 						gnode, _ = subgraphs[pid].CreateNodeByName("(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc) + ")")
// 					} else {
// 						gnode, _ = graph.CreateNodeByName("(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc) + ")")
// 					}
// 				}
// 				act := true
// 				if animate {
// 					gnode.SetStyle("invis")
// 				} else {
// 					if !ex {
// 						act = false
// 					} else if p.pc <= pc {
// 						act = false
// 					}
// 					if !act {
// 						gnode.SetColor("gray")
// 					}
// 				}
// 				//render invis edges that connect to previous pc of same proc
// 				if SEQ_DAG {
// 					var pnode *cgraph.Node
// 					pnode, _ = subgraphs[pid].NodeByName("(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc-1) + ")")
// 					if pnode != nil {
// 						pedge, _ := graph.CreateEdgeByName("", pnode, gnode)
// 						pedge.SetWeight(100000)
// 						pedge.SetStyle("invis")
// 					}
// 				}
// 				//render all other incoming edges
// 			L1:
// 				for _, e := range edges {
// 					if !e.wt {
// 						for _, ae := range edges {
// 							if ae.pid == e.pid && ae.pc == e.pc && ae.varname == e.varname && ae.wt {
// 								continue L1
// 							}
// 						}
// 					}
// 					var tnode *cgraph.Node
// 					if e.pid == -1 && e.pc == -1 {
// 						tnode = bot
// 					} else {
// 						if SEQ_DAG {
// 							tnode, _ = subgraphs[e.pid].NodeByName("(" + strconv.Itoa(e.pid) + "," + strconv.Itoa(e.pc) + ")")
// 						} else {
// 							tnode, _ = graph.NodeByName("(" + strconv.Itoa(e.pid) + "," + strconv.Itoa(e.pc) + ")")
// 						}
// 						if tnode == nil {
// 							if SEQ_DAG {
// 								tnode, _ = subgraphs[e.pid].CreateNodeByName("(" + strconv.Itoa(e.pid) + "," + strconv.Itoa(e.pc) + ")")
// 							} else {
// 								tnode, _ = graph.CreateNodeByName("(" + strconv.Itoa(e.pid) + "," + strconv.Itoa(e.pc) + ")")
// 							}
// 							gnode.SetGroup(strconv.Itoa(e.pid))
// 						}
// 					}
// 					if animate {
// 						if e.pid != -1 || e.pc != -1 {
// 							tnode.SetStyle("invis")
// 						}
// 					} else {
// 						tp, ex := r.getPC(e.pid)
// 						act := true
// 						if !ex {
// 							act = false
// 						} else if tp.pc <= e.pc {
// 							act = false
// 						}
// 						if e.pid == -1 && e.pc == -1 {
// 							act = true
// 						}
// 						if !act {
// 							tnode.SetColor("gray")
// 						}
// 					}
// 					gedge, _ := graph.CreateEdgeByName("(" + strconv.Itoa(e.pid) + "," + strconv.Itoa(e.pc) + ")->(" + strconv.Itoa(pid) + "," + strconv.Itoa(pc) + ")", tnode, gnode)
// 					gedge.SetLabel(e.varname)
// 					if !e.wt {
// 						gedge.SetStyle("dashed")
// 					}
// 					if animate {
// 						gedge.SetColor("white")
// 						gedge.SetFontColor("white")
// 					} else {
// 						if !act {
// 							gedge.SetColor("gray")
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// 	if !animate {
// 		fmt.Print("Rendering...\n")
// 		var buf bytes.Buffer
// 		r.dag.gviz.Render(*r.dag.gvcontext, graph, "dot", &buf)
// 		fmt.Println(buf.String())
// 		e := r.dag.gviz.RenderFilename(*r.dag.gvcontext,graph, graphviz.PNG, "dag.png")
// 		if e != nil{
// 			fmt.Print(e)
// 		}
		
// 		fmt.Printf("Done!\n")
// 		return
// 	}
// 	pcmax := make(map[int]int)
// 	pccur := make(map[int]int)
// 	instnode, _ := graph.CreateNodeByName("inst")
// 	instnode.SetShape(cgraph.BoxShape)
// 	for imindex, p := range r.dag.exechistory {
// 		fmt.Printf("Drawing DAG... (%d/%d)\n", imindex+1, len(r.dag.exechistory))
// 		instnode.SetLabel("(" + strconv.Itoa(p.pid) + "," + strconv.Itoa(p.pc) + ")" + "\n" + p.exec)
// 		v, ex := pcmax[p.pid]
// 		if !ex {
// 			pcmax[p.pid] = p.pc
// 		} else if p.pc > v {
// 			pcmax[p.pid] = p.pc
// 		}
// 		pccur[p.pid] = p.pc
// 		for npid, mpc := range pcmax {
// 			for npc := range mpc + 1 {
// 				var gnode *cgraph.Node
// 				if SEQ_DAG {
// 					gnode, _ = subgraphs[npid].NodeByName("(" + strconv.Itoa(npid) + "," + strconv.Itoa(npc) + ")")
// 				} else {
// 					gnode, _ = graph.NodeByName("(" + strconv.Itoa(npid) + "," + strconv.Itoa(npc) + ")")
// 				}
// 				if gnode != nil {
// 					c, ex := pccur[npid]
// 					if !ex {
// 						c = 0
// 					}
// 					if (npc > c && !p.rev) || (npc >= c && p.rev) {
// 						gnode.SetStyle("")
// 						gnode.SetColor("gray")
// 						gnode.SetFontColor("gray")
// 					} else {
// 						gnode.SetStyle("")
// 						gnode.SetColor("")
// 						gnode.SetFontColor("")
// 					}
// 					e,_ := graph.FirstIn(gnode)
// 					for e != nil {
// 						if (npc > c && !p.rev) || (npc >= c && p.rev) {
// 							e.SetColor("gray")
// 							e.SetFontColor("gray")
// 						} else {
// 							e.SetColor("")
// 							e.SetFontColor("")
// 						}
// 						e,_ = graph.NextIn(e)
// 					}

// 				}
// 			}
// 		}
// 		im, _ := r.dag.gviz.RenderImage(*r.dag.gvcontext,graph)
// 		bounds := im.Bounds()
// 		pl := image.NewPaletted(bounds, palette.WebSafe)
// 		draw.Draw(pl, bounds, im, bounds.Min, draw.Src)
// 		files = append(files, pl)
// 	}
// 	delays := make([]int, 0, len(r.dag.exechistory))
// 	for _ = range len(r.dag.exechistory) {
// 		delays = append(delays, 50)
// 	}
// 	f, _ := os.Create("dag.gif")
// 	defer f.Close()
// 	gif.EncodeAll(f, &gif.GIF{
// 		Image: files,
// 		Delay: delays,
// 	})
// 	fmt.Print("Done!\n")
// }
