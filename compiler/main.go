//go:generate $GOPATH/bin/goyacc -o parser.go -p "parser" parser.y
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
)
var cmp *compiler
var prep = true
// var EXPRLOCK = false
// var EXPRATOM = false

type arrentry struct {
	val string
	tag int
}

//yylval of type parserSymType provides
//yylval.<tokenname>

//Extremely dirty lexer
type token struct{
	reg *regexp.Regexp
	process func(match string, yylval *parserSymType) int
}

type ctype int

const (
	T_INT ctype = iota
	T_INTA
	T_SYNC
)
func (t ctype) String()string{
	switch t{
	case T_INT:
		return "int"
	case T_INTA:
		return "int[]"
	case T_SYNC:
		return "sync"
	default:
		panic("Unknown type")
	}
}

func getTypeFromString(name string)ctype{
	switch name{
	case "int":
		return T_INT
	case "int[]":
		return T_INTA
	case "sync":
		return T_SYNC
	default:
		panic("Unknown type")
	}
}

type compiler struct{
	output []string //output line, [[x,exec]] is treated special upon export
	nest int
	glovarmap map[string]ctype //type of global variables, in runtime registered in heapmem
	argvarmap map[string]ctype //type of arg variables, in runtime registered in varmem with nest=0
	localvars []string //local variables, guranteed to be only one allocation for each indent, and guranteed to be int, in runtime registered in varmem with nest>0

	labelindex int
	tmpindex int
	memindex int

	tagindex int //tags each expression
	execstore []string //stored execution, separated by \n
	unexecstore []string //stored unexecution
	usedvar [][]string //stored variables
	storedest int //destination of exec/unexec/var to store, -1 for no store

	declargmap map[string][]string //decleard argments
	decltypemap map[string][]ctype //decleared types
	declpname string //currently declearing process name, used in prep only
	callpname string //currently calling process name
	argindex int //current arg index
	procrec []string //currently calling procs
}

func newCompiler() *compiler{
	return &compiler{
		output: make([]string, 0),
		nest: 0,
		glovarmap: make(map[string]ctype),
		argvarmap: make(map[string]ctype),
		localvars: make([]string, 0),
		labelindex: 0,
		tmpindex: 0,
		memindex: 0,
		tagindex: 0,
		execstore: make([]string, 0),
		unexecstore: make([]string, 0),
		usedvar: make([][]string, 0),
		storedest: -1,
		declargmap: make(map[string][]string),
		decltypemap: make(map[string][]ctype),
	}	
}

func (c *compiler) registerGlobal(name string, t ctype){
	c.glovarmap[name] = t
}

func (c *compiler) registerLocal(name string){
	c.localvars[c.nest-1] = name
}

func (c *compiler) indent(){
	c.nest++
	c.localvars = append(c.localvars, "")
}

func (c *compiler) unindent(){
	c.nest--
	c.localvars = c.localvars[:c.nest]
}

func (c *compiler) registerDecl(argname string, t ctype){
	_,ex := c.declargmap[c.declpname]
	if !ex {
		c.declargmap[c.declpname] = make([]string, 0)
	}
	_,ex = c.decltypemap[c.declpname]
	if !ex {
		c.decltypemap[c.declpname] = make([]ctype, 0)
	}
	c.declargmap[c.declpname] = append(c.declargmap[c.declpname], argname)
	c.decltypemap[c.declpname] = append(c.decltypemap[c.declpname], t)
}

func (c *compiler) setProc(pname string){
	args,ex := c.declargmap[pname]
	if !ex {
		args = make([]string, 0)
	}
	tps,ex := c.decltypemap[pname]
	if !ex {
		tps = make([]ctype, 0)
	}
	clear(c.argvarmap)
	for i := range len(args){
		c.argvarmap[args[i]] = tps[i]
	}
}

func (c *compiler) typeOf(name string)ctype{
	for i := range len(c.localvars){
		lvar := c.localvars[len(c.localvars)-i-1]
		if name == lvar{
			return T_INT
		}
	}
	t,ex := c.argvarmap[name]
	if ex {
		return t
	}
	t,ex = c.glovarmap[name]
	if ex{
		return t
	}
	panic("Unknown variable")
}

func (c *compiler) getTag() int {
	t := c.tagindex
	c.tagindex++
	c.execstore = append(c.execstore, "")
	c.unexecstore = append(c.unexecstore, "")
	c.usedvar = append(c.usedvar, make([]string, 0))
	return t
}

func (c *compiler) getLabel() string{
	t := c.labelindex
	c.labelindex++
	return "l"+strconv.Itoa(t)
}

func (c *compiler) getTmp() string{
	t := c.tmpindex
	c.tmpindex++
	return "tmp"+strconv.Itoa(t)
}

func (c *compiler) getProcArgs(pname string) ([]string,[]ctype) {
	return c.declargmap[pname],c.decltypemap[pname]
}

func (c *compiler) exec(s string){
	if c.storedest == -1{
		c.output = append(c.output, s)
	} else {
		if c.execstore[c.storedest] != ""{
			c.execstore[c.storedest] += "\n"
		}
		c.execstore[c.storedest] += s
	}
}

func (c *compiler) unexec(s string){
	if c.storedest == -1{
		c.output = append(c.output, s)
	} else {

		if c.unexecstore[c.storedest] != ""{
			s += "\n"
		}
		c.unexecstore[c.storedest] = s + c.unexecstore[c.storedest]
	}
}

func (t ctype) match(t1 ctype){
	if t != t1 {
		panic("Unmatched type")
	}
}

func (c *compiler) addUsedVar(name string){
	if !slices.Contains(c.usedvar[c.storedest],name){
		c.usedvar[c.storedest] = append(c.usedvar[c.storedest], name)
	}
}

func (c *compiler) beginRecord(tag int){
	if c.storedest != -1{
		panic("Double recording")
	}
	c.storedest = tag
}

func (c *compiler) endRecord(tag int){
	if tag != c.storedest{
		panic("Recording tag mismatch")
	}
	c.storedest = -1
}

func (c *compiler) execrecord(tag int){
	c.output = append(c.output, fmt.Sprintf("[[%d,exec]]",tag))
}

func (c *compiler) unexecrecord(tag int){
	c.output = append(c.output, fmt.Sprintf("[[%d,unexec]]",tag))
}

func (c *compiler) lockrecord(tag int){
	c.output = append(c.output, fmt.Sprintf("[[%d,lock]]",tag))
}

func (c *compiler) unlockrecord(tag int){
	c.output = append(c.output, fmt.Sprintf("[[%d,unlock]]",tag))
}

func (c *compiler) getMem(size int)string{
	t := c.memindex
	c.memindex += size
	return fmt.Sprintf("M[%d]",t)
}

var replacereg = regexp.MustCompile(`\[\[(\d+),(\w+)\]\]`)

func (c *compiler) export() string{
	s := ""
	for _,v := range c.output{
		m := replacereg.FindStringSubmatch(v)
		if m != nil {
			idx,_ := strconv.Atoi(m[1])
			switch m[2]{
			case "exec":
				if c.execstore[idx] != ""{
					s += c.execstore[idx] + "\n"
				}
			case "unexec":
				if c.unexecstore[idx] != ""{
					s += c.unexecstore[idx] + "\n"
				}
			case "lock":
				if len(c.usedvar[idx]) != 0{
					s += "lock "
					for i,v := range c.usedvar[idx]{
						if i != 0 {
							s += ", "
						}
						s += v
					}
					s += "\n"
				}
			case "unlock":
				if len(c.usedvar[idx]) != 0{
					s += "unlock "
					for i,v := range c.usedvar[idx]{
						if i != 0 {
							s += ", "
						}
						s += v
					}
					s += "\n"
				}
			}
		} else {
			s += v
			s += "\n"
		}
	}
	return s
}


type progLex struct {
	input string
	tokens []token
}

func newLexer(filename string) *progLex{
	file, _ := os.ReadFile(filename)
	tokens := make([]token,0,30)
	//add token stuff
	//special chars
	tokens = append(tokens, token{regexp.MustCompile(`^\+`),
	func(match string, yylval *parserSymType) int {
		return PLUS
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^-`),
	func(match string, yylval *parserSymType) int {
		return MINUS
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\^`),
	func(match string, yylval *parserSymType) int {
		return XOR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\*`),
	func(match string, yylval *parserSymType) int {
		return MULT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^/`),
	func(match string, yylval *parserSymType) int {
		return DIV
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^%`),
	func(match string, yylval *parserSymType) int {
		return MOD
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^&&`),
	func(match string, yylval *parserSymType) int {
		return AND
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^||`),
	func(match string, yylval *parserSymType) int {
		return OR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^&`),
	func(match string, yylval *parserSymType) int {
		return BITAND
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^|`),
	func(match string, yylval *parserSymType) int {
		return BITOR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^<=`),
	func(match string, yylval *parserSymType) int {
		return LEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^>=`),
	func(match string, yylval *parserSymType) int {
		return GEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^!=`),
	func(match string, yylval *parserSymType) int {
		return NEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^=`),
	func(match string, yylval *parserSymType) int {
		return EQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^<`),
	func(match string, yylval *parserSymType) int {
		return LES
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^>`),
	func(match string, yylval *parserSymType) int {
		return GRT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\(`),
	func(match string, yylval *parserSymType) int {
		return LPR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\)`),
	func(match string, yylval *parserSymType) int {
		return RPR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\[`),
	func(match string, yylval *parserSymType) int {
		return LSB
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\]`),
	func(match string, yylval *parserSymType) int {
		return RSB
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^{`),
	func(match string, yylval *parserSymType) int {
		return LCB
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^}`),
	func(match string, yylval *parserSymType) int {
		return RCB
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^,`),
	func(match string, yylval *parserSymType) int {
		return COMMA
	}})
	//keywords
	//word boundary come in handy
	tokens = append(tokens, token{regexp.MustCompile(`^V\b`),
	func(match string, yylval *parserSymType) int {
		return V
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^P\b`),
	func(match string, yylval *parserSymType) int {
		return P
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^procedure\b`),
	func(match string, yylval *parserSymType) int {
		return PROCEDURE
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^main\b`),
	func(match string, yylval *parserSymType) int {
		return MAIN
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^int\b`),
	func(match string, yylval *parserSymType) int {
		return INT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^if\b`),
	func(match string, yylval *parserSymType) int {
		return IF
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^then\b`),
	func(match string, yylval *parserSymType) int {
		return THEN
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^else\b`),
	func(match string, yylval *parserSymType) int {
		return ELSE
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^fi\b`),
	func(match string, yylval *parserSymType) int {
		return FI
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^from\b`),
	func(match string, yylval *parserSymType) int {
		return FROM
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^do\b`),
	func(match string, yylval *parserSymType) int {
		return DO
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^loop\b`),
	func(match string, yylval *parserSymType) int {
		return LOOP
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^until\b`),
	func(match string, yylval *parserSymType) int {
		return UNTIL
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^local\b`),
	func(match string, yylval *parserSymType) int {
		return LOCAL
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^delocal\b`),
	func(match string, yylval *parserSymType) int {
		return DELOCAL
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^call\b`),
	func(match string, yylval *parserSymType) int {
		return CALL
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^uncall\b`),
	func(match string, yylval *parserSymType) int {
		return UNCALL
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^begin\b`),
	func(match string, yylval *parserSymType) int {
		return BEGIN
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^end\b`),
	func(match string, yylval *parserSymType) int {
		return END
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^skip\b`),
	func(match string, yylval *parserSymType) int {
		return SKIP
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^sync\b`),
	func(match string, yylval *parserSymType) int {
		return SYNC
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^wait\b`),
	func(match string, yylval *parserSymType) int {
		return WAIT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^acquire\b`),
	func(match string, yylval *parserSymType) int {
		return ACQUIRE
	}})
	//integer constants
	tokens = append(tokens, token{regexp.MustCompile(`^-?\d+`),
	func(match string, yylval *parserSymType) int {
		n,_ := strconv.Atoi(match)
		yylval.num = n
		return NUM
	}})
	//identifier (variable / func / whatever)
	tokens = append(tokens, token{regexp.MustCompile(`^\w+`),
	func(match string, yylval *parserSymType) int {
		yylval.ident = match
		return IDENT
	}})
	return &progLex{string(file),tokens}
}

func (x *progLex) Lex(yylval *parserSymType) int{
	x.input = strings.TrimLeft(x.input,"\r\n\t\f\v ")//remove whitespaces
	if len(x.input) == 0{
		return 0
	}
	for _, v := range x.tokens {
		s := v.reg.FindString(x.input)
		if s != ""{
			x.input = strings.TrimPrefix(x.input,s)
			return v.process(s,yylval)
		}
	}
	panic("Token not found")
}

func (x *progLex) Error(s string){
	panic(s)
}

func main(){
	flag.Usage = func(){
		fmt.Fprintf(flag.CommandLine.Output(), "usage: %s [args] file\n", os.Args[0])
		flag.PrintDefaults()
	}
	outfname := flag.String("o","a.crl","Specifies output `file`. Default is a.crl")
	// flag.BoolVar(&EXPRLOCK,"nolock",false,"Do not use lock and unlock statements to protect decomposed expression.")
	// flag.BoolVar(&EXPRATOM,"atom",false,"Do not decompose expression and leave the evaluation to do in runtime atomically.")
	flag.Parse()
	infname := flag.Arg(0)
	lexer := newLexer(infname)
	cmp = newCompiler()
	parserParse(lexer)
	prep = false
	lexer = newLexer(infname)
	//cmp.reset()
	parserParse(lexer)
	outf,_ := os.Create(*outfname)
	defer outf.Close()
	outwriter := bufio.NewWriter(outf)
	outwriter.WriteString(cmp.export())
	outwriter.Flush()
}