 %{
package main
import (
	"strconv"
	"regexp"
	"strings"
)
%}

%union {
	num func()int
	constnum int
	ident string
}

%token <constnum> NUM
%token <ident> IDENT
%token PLUS MINUS XOR MULT DIV MOD AND OR BITAND BITOR LEQ GEQ NEQ EQ LES GRT LPR RPR LSB RSB
%type <num> variable expr1 expr2 expr3 expr4 expr5 expr6

%%
expr : expr1
{
	if l,ok := exprlex.(*exprLex); ok {
		f := $1
		l.retfunc = func()int{
			return f()
		}
	} else {panic("exprlex is somehow not of type exprLex!?")}
}
expr1 : expr2
{
	f := $1
	$$ = func()int{return f()}
}
		| expr1 OR expr2
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int {
				if f1() != 0 || f2() != 0{
					return 1
				} else {
					return 0
				}
			}
		}
expr2 : expr3
{
	f := $1
	$$ = func()int{return f()}
}
		| expr2 AND expr3
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int {
				if f1() != 0 && f2() != 0{
					return 1
				} else {
					return 0
				}
			}
		}

expr3 : expr4
{
	f := $1
	$$ = func()int{return f()}
}
		| expr3 LEQ expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() <= f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 GEQ expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() >= f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 EQ expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() == f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 EQ EQ expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $4
			$$ = func()int{
				if f1() == f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 NEQ expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() != f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 LES expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() < f2(){
					return 1
				} else {
					return 0
				}
			}
		}
		| expr3 GRT expr4
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{
				if f1() > f2(){
					return 1
				} else {
					return 0
				}
			}
		}
expr4 : expr5
{
	f := $1
	$$ = func()int{return f()}
}
		| expr4 PLUS expr5
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() + f2()}
		}
		| expr4 MINUS expr5
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() - f2()}
		}
		| expr4 BITOR expr5
		| expr4 XOR expr5
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() ^ f2()}
		}
expr5 : expr6
{
	f := $1
	$$ = func()int{return f()}
}
		| expr5 MULT expr6
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() * f2()}
		}
		| expr5 DIV expr6
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() / f2()}
		}
		| expr5 MOD expr6
		{
			if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
			f1 := $1
			f2 := $3
			$$ = func()int{return f1() % f2()}
		}
		| expr5 BITAND expr6

expr6 : variable
{
	f := $1
	$$ = func()int{return f()}
}
		| LPR expr1 RPR
{
	f := $2
	$$ = func()int{return f()}
}
		| MINUS variable
{
	if !EXTEXPR{
				if l,ok := exprlex.(*exprLex); ok {
					if !l.exdone{
						l.exdone = true
					} else {panic("multiple expr without extexpr option")}
				} else {panic("exprlex is somehow not of type exprLex!?")}
			}
	$$ = func()int{return -1 * $2()}
}

variable : NUM
{
	f := $1
	$$ = func()int{return f}
}
		 | IDENT
{
	if l,ok := exprlex.(*exprLex); ok {
		adr := l.r.GetAddr($1,l.p)
		l.read = append(l.read,adr)
		$$ = func()int{
			l.r.LockAddr(l.p,adr)
			v := l.r.ReadAddr(adr,l.p)
			l.r.UnlockAddr(l.p,adr)
			return v
		}
	} else {panic("exprlex is somehow not of type exprLex!?")}
}
		 | IDENT LSB expr1 RSB
{
	if l,ok := exprlex.(*exprLex); ok {
		adr := l.r.GetAddr($1+"["+strconv.Itoa($3())+"]",l.p)
		l.read = append(l.read,adr)
		$$ = func()int{
			l.r.LockAddr(l.p,adr)
			v := l.r.ReadAddr(adr,l.p)
			l.r.UnlockAddr(l.p,adr)
			return v
		}
	} else {panic("exprlex is somehow not of type exprLex!?")}
}
%%
//will not written concurrently, so it can be global
var tokens = initTokens()

type exprLex struct {
	input string
	retfunc func()int
	read []addr
	r *runtime
	p *pc
	exdone bool
}

type token struct{
	reg *regexp.Regexp
	process func(match string, yylval *exprSymType) int
}
func initTokens() []token{
	tokens := make([]token,0,30)
	//add token stuff
	//special chars
	tokens = append(tokens, token{regexp.MustCompile(`^\+`),
	func(match string, yylval *exprSymType) int {
		return PLUS
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^-`),
	func(match string, yylval *exprSymType) int {
		return MINUS
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\^`),
	func(match string, yylval *exprSymType) int {
		return XOR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\*`),
	func(match string, yylval *exprSymType) int {
		return MULT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^/`),
	func(match string, yylval *exprSymType) int {
		return DIV
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^%`),
	func(match string, yylval *exprSymType) int {
		return MOD
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^&&`),
	func(match string, yylval *exprSymType) int {
		return AND
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\|\|`),
	func(match string, yylval *exprSymType) int {
		return OR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^&`),
	func(match string, yylval *exprSymType) int {
		return BITAND
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\|`),
	func(match string, yylval *exprSymType) int {
		return BITOR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^<=`),
	func(match string, yylval *exprSymType) int {
		return LEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^>=`),
	func(match string, yylval *exprSymType) int {
		return GEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^!=`),
	func(match string, yylval *exprSymType) int {
		return NEQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^=`),
	func(match string, yylval *exprSymType) int {
		return EQ
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^<`),
	func(match string, yylval *exprSymType) int {
		return LES
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^>`),
	func(match string, yylval *exprSymType) int {
		return GRT
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\(`),
	func(match string, yylval *exprSymType) int {
		return LPR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\)`),
	func(match string, yylval *exprSymType) int {
		return RPR
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\[`),
	func(match string, yylval *exprSymType) int {
		return LSB
	}})
	tokens = append(tokens, token{regexp.MustCompile(`^\]`),
	func(match string, yylval *exprSymType) int {
		return RSB
	}})
	//integer constants
	tokens = append(tokens, token{regexp.MustCompile(`^-?\d+`),
	func(match string, yylval *exprSymType) int {
		n,_ := strconv.Atoi(match)
		yylval.constnum = n
		return NUM
	}})
	//identifier (variable / func / whatever)
	tokens = append(tokens, token{regexp.MustCompile(`^[\w\d\[\]\$\#]+`),
	func(match string, yylval *exprSymType) int {
		yylval.ident = match
		return IDENT
	}})
	return tokens
}
func (x *exprLex) Lex(yylval *exprSymType) int{
	x.input = strings.TrimLeft(x.input,"\r\n\t\f\v ")//remove whitespaces
	if len(x.input) == 0{
		return 0
	}
	for _, v := range tokens {
		s := v.reg.FindString(x.input)
		if s != ""{
			x.input = strings.TrimPrefix(x.input,s)
			return v.process(s,yylval)
		}
	}
	panic("Token not found")
}

func (x *exprLex) Error(s string){
	panic("Syntax Error: " + s)
}

func EvalExpr(s string, rt *runtime, pc *pc) (func()int, []addr) {
	lexer := &exprLex{s,nil,make([]addr,0,1),rt,pc,false}
	exprParse(lexer)
	return lexer.retfunc,lexer.read
}