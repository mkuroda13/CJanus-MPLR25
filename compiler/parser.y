%{
package main

import (
	"strconv"
)
%}

%union {
	num int
	ident string
	varid string
	tag int
	arrentry arrentry
}
%token <num> NUM
%token <ident> IDENT
%token PLUS MINUS XOR MULT DIV MOD AND OR BITAND BITOR LEQ GEQ NEQ EQ LES GRT LSB RSB LCB RCB COMMA LPR RPR
%token PROCEDURE MAIN INT IF THEN ELSE FI FROM DO LOOP UNTIL LOCAL DELOCAL CALL UNCALL SKIP BEGIN END P V SYNC WAIT ACQUIRE
%type <varid> variable expr expr2 expr3 expr4 expr5 expr6 lab
%type <arrentry> arrentry
%type <tag> tag

%%
prog : pmain procs

procs : proc procs
|

pmain : 
{	
	if !prep{
		cmp.exec("begin main")
	}
}
globvardecls
statements
{
	if !prep{
		cmp.exec("end main")
	}
}


globvardecls : globvardecl globvardecls
|

globvardecl : INT IDENT
{
	if !prep{
		cmp.registerGlobal($2,T_INT)
		cmp.exec("set #"+$2+" "+cmp.getMem(1))
	}
}			| SYNC IDENT
{
	if !prep{
		cmp.registerGlobal($2,T_SYNC)
		cmp.exec("set #"+$2+" "+cmp.getMem(1))
	}
}
			| INT IDENT LSB NUM RSB
{
	if !prep{
		cmp.registerGlobal($2,T_INTA)
		cmp.exec("set #"+$2+" "+cmp.getMem($4))
	}
}

proc : PROCEDURE IDENT 
{
	if prep {
		cmp.declpname = $2
	} else {
		cmp.setProc($2)
		cmp.exec("begin "+$2)
	}
}
LPR argdecls RPR statements
{
	if !prep{
		cmp.exec("end "+$2)
	}
}

argdecls: argdecl argdeclmore
		  |
		  

argdeclmore: COMMA argdecl argdeclmore
		   |

argdecl: INT IDENT
{
	if prep{
		cmp.registerDecl($2,T_INT)
	}
}
		| INT LSB RSB IDENT
{
	if prep{
		cmp.registerDecl($4,T_INTA)
	}
}
		| SYNC IDENT
{
	if prep{
		cmp.registerDecl($2,T_SYNC)
	}
}

statements : statement statements
|

statements_plus : statement statements

statement : assign_statement
			| arrassign_statement
			| if_statement
			| loop_statement
			| call_statement
			| SKIP
			| local_block
			| v_statement
			| p_statement

assign_statement : IDENT PLUS EQ tag
{
	if !prep {
		cmp.beginRecord($4)
	}
}
	expr
{
	if !prep{
		cmp.endRecord($4)
		cmp.lockrecord($4)
		cmp.execrecord($4)
		cmp.exec($1+" += " + $6)
		cmp.unexecrecord($4)
		cmp.unlockrecord($4)
	}

}
	| IDENT MINUS EQ tag
{
	if !prep {
		cmp.beginRecord($4)
	}
}
	expr
{
	if !prep{
		cmp.endRecord($4)
		cmp.lockrecord($4)
		cmp.execrecord($4)
		cmp.exec($1+" -= " + $6)
		cmp.unexecrecord($4)
		cmp.unlockrecord($4)
	}

}
	| IDENT XOR EQ tag
{
	if !prep {
		cmp.beginRecord($4)
	}
}
		expr
{
	if !prep{
		cmp.endRecord($4)
		cmp.lockrecord($4)
		cmp.execrecord($4)
		cmp.exec($1+" ^= " + $6)
		cmp.unexecrecord($4)
		cmp.unlockrecord($4)
	}

}

arrentry : IDENT LSB tag
{
	if !prep {
		cmp.beginRecord($3)
	}
}
expr RSB
{
	if !prep {
		cmp.endRecord($3)
		$$ = arrentry{$1+"["+$5+"]",$3}
	}
}

arrassign_statement : arrentry PLUS EQ
{
	if !prep {
		cmp.beginRecord($1.tag)
	}
}
expr
{
	if !prep{
		cmp.endRecord($1.tag)
		cmp.lockrecord($1.tag)
		cmp.execrecord($1.tag)
		cmp.exec($1.val+" += " + $5)
		cmp.unexecrecord($1.tag)
		cmp.unlockrecord($1.tag)
	}

}
| arrentry MINUS EQ
{
	if !prep {
		cmp.beginRecord($1.tag)
	}
}
expr
{
	if !prep{
		cmp.endRecord($1.tag)
		cmp.lockrecord($1.tag)
		cmp.execrecord($1.tag)
		cmp.exec($1.val+" -= " + $5)
		cmp.unexecrecord($1.tag)
		cmp.unlockrecord($1.tag)
	}

}
| arrentry XOR EQ
{
	if !prep {
		cmp.beginRecord($1.tag)
	}
}
expr
{
	if !prep{
		cmp.endRecord($1.tag)
		cmp.lockrecord($1.tag)
		cmp.execrecord($1.tag)
		cmp.exec($1.val+" ^= " + $5)
		cmp.unexecrecord($1.tag)
		cmp.unlockrecord($1.tag)
	}

}

if_statement : IF lab lab lab lab tag tag
{
	if !prep {
		cmp.beginRecord($6)
	}
}
expr
{
	if !prep{
		cmp.endRecord($6)
		cmp.lockrecord($6)
		cmp.execrecord($6)
		cmp.exec($9 + " -> " + $2 + ";" +$3)
		cmp.exec($2 + " <-")
		cmp.unexecrecord($6)
		cmp.unlockrecord($6)
	}
}
THEN statements_plus
{
	if !prep{
		cmp.lockrecord($7)
		cmp.execrecord($7)
		cmp.exec("-> " + $4)
		cmp.exec($3 + " <-")
		cmp.unexecrecord($6)
		cmp.unlockrecord($6)
	}
}
ELSE statements_plus
{
	if !prep{
		cmp.lockrecord($7)
		cmp.execrecord($7)
		cmp.exec("-> " + $5)
		cmp.beginRecord($7)
	}
}
FI expr
{
	if !prep{
		cmp.endRecord($7)
		cmp.exec($4 + ";" + $5 + " <- " + $18)
		cmp.unexecrecord($7)
		cmp.unlockrecord($7)
	}
}

lab : 
{
	if !prep{
		$$ = cmp.getLabel()
	}
}

loop_statement : FROM lab lab lab lab lab tag tag
{
	if !prep {
		cmp.beginRecord($7)
	}
}
expr
{
	if !prep{
		cmp.endRecord($7)
		cmp.lockrecord($7)
		cmp.execrecord($7)
		cmp.exec("-> " + $2)
		cmp.exec($2 + ";" +$6 + " <- "+$10)
		cmp.unexecrecord($7)
		cmp.unlockrecord($7)
	}
}
DO statements_plus
{
	if !prep{
		cmp.exec("-> " + $3)
		cmp.exec($5 +" <-")
		cmp.unexecrecord($8)
		cmp.unlockrecord($8)
	}
}
LOOP statements_plus UNTIL 
{
	if !prep{
		cmp.lockrecord($7)
		cmp.execrecord($7)
		cmp.exec("-> " + $6)
		cmp.exec($3 + " <-")
		cmp.beginRecord($8)
	}
}
expr
{
	if !prep{
		cmp.endRecord($8)
		cmp.lockrecord($8)
		cmp.execrecord($8)
		cmp.exec($19 + " -> "+$4 + ";" + $5 )
		cmp.exec($4 + " <-")
		cmp.unexecrecord($8)
		cmp.unlockrecord($8)
	}
}

local_block:
LOCAL
{
	if !prep{
		cmp.exec("push")
		cmp.indent()
	}
}
INT IDENT EQ tag tag
{
	if !prep{
		cmp.registerLocal($4)
		cmp.beginRecord($6)
	}
}
expr
{
	if !prep{
		cmp.endRecord($6)
		cmp.lockrecord($6)
		cmp.execrecord($6)
		cmp.exec("$"+$4+" += "+$9)
		cmp.unexecrecord($6)
		cmp.unlockrecord($6)
	}
}
statements_plus DELOCAL INT IDENT EQ 
{
	if !prep{
		cmp.beginRecord($7)
	}
}
expr
{
	if !prep {
		cmp.endRecord($7)
		cmp.lockrecord($7)
		cmp.execrecord($7)
		cmp.exec("$"+$14+" -= " + $17)
		cmp.unexecrecord($7)
		cmp.unlockrecord($7)
		cmp.exec("pop")
		cmp.unindent()
	}
}

args: arg argmore
	|
	

argmore: COMMA arg argmore
		|

arg: IDENT
{
	if !prep {
		i,t := cmp.getProcArgs(cmp.callpname)
		t[cmp.argindex].match(cmp.typeOf($1))
		cmp.exec("set $"+i[cmp.argindex] + "@" + strconv.Itoa(len(cmp.procrec)-1) + " " + $1)
		cmp.unexec("unset $"+i[cmp.argindex] + "@" + strconv.Itoa(len(cmp.procrec)-1) + " "+ $1)
		cmp.argindex++
	}
}

v_statement : ACQUIRE IDENT
{
	if !prep {
		cmp.exec("V "+$2)
	}
}
p_statement : WAIT IDENT
{
	if !prep {
		cmp.exec("P "+$2)
	}
}

call_statement : CALL tag
{
	if !prep{
		cmp.procrec = make([]string,0)
		cmp.beginRecord($2)
	}
}
proccalls
{
	if !prep{
		cmp.endRecord($2)
		cmp.execrecord($2)
		s := ""
		for i,p := range cmp.procrec {
			if i != 0 {
				s += ", "
			}
			s += p
		}
		cmp.exec("call "+s)
		cmp.unexecrecord($2)
		cmp.procrec = make([]string,0)
	}
}

proccall : IDENT
{
	if !prep{
		cmp.callpname = $1
		cmp.procrec = append(cmp.procrec,$1)
		cmp.argindex = 0	
	}
}
LPR args RPR
{
	if !prep {
		cmp.argindex = 0
	}
}

proccalls : proccall proccallmore

proccallmore : COMMA proccall proccallmore
			 | 

tag : 
{
	$$ = cmp.getTag()
}

expr : expr2
{
	$$ = $1
}
		| expr OR expr2
		{
			if !prep {
				tmp := cmp.getTmp()
				cmp.exec("$" + tmp + " += " + $1 + " ||" + $3)
				cmp.unexec("$" + tmp + " -= " + $1 + " ||" + $3)
				$$ = tmp
			}
		}
expr2 : expr3
{
	$$ = $1
}
		| expr2 AND expr3
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " && " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " && " + $3)
				$$ = tmp
			}
		}

expr3 : expr4
{
	$$ = $1
}
		| expr3 LEQ expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " <= " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " <= " + $3)
				$$ = tmp
			}
		}
		| expr3 GEQ expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " >= " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " >= " + $3)
				$$ = tmp
			}
		}
		| expr3 EQ expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " == " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " == " + $3)
				$$ = tmp
			}
		}
		| expr3 NEQ expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " != " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " != " + $3)
				$$ = tmp
			}
		}
		| expr3 LES expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " < " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " < " + $3)
				$$ = tmp
			}
		}
		| expr3 GRT expr4
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " > " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " > " + $3)
				$$ = tmp
			}
		}
expr4 : expr5
{
	$$ = $1
}
		| expr4 PLUS expr5
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " + " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " + " + $3)
				$$ = tmp
			}
		}
		| expr4 MINUS expr5
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " - " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " - " + $3)
				$$ = tmp
			}
		}
		| expr4 BITOR expr5
		| expr4 XOR expr5
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " ^ " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " ^ " + $3)
				$$ = tmp
			}
		}
expr5 : expr6
{
	$$ = $1
}
		| expr5 MULT expr6
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " * " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " * " + $3)
				$$ = tmp
			}
		}
		| expr5 DIV expr6
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " / " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " / " + $3)
				$$ = tmp
			}
		}
		| expr5 MOD expr6
		{
			if !prep{
				tmp := cmp.getTmp()
				cmp.exec("$"+tmp + " += " + $1 + " % " + $3)
				cmp.unexec("$"+tmp + " -= " + $1 + " % " + $3)
				$$ = tmp
			}
		}
		| expr5 BITAND expr6

expr6 : variable
{
	$$ = $1
}
		| LPR expr RPR
{
	$$ = $2
}

variable : NUM
{
	if !prep{
		a := strconv.Itoa($1)
		tmp := cmp.getTmp()
		cmp.exec("$"+tmp + " += " + a)
		cmp.unexec("$"+tmp + " -= " + a)
		$$ = tmp
	}
}
		 | IDENT
{
	if !prep{
		cmp.addUsedVar($1)
		T_INT.match(cmp.typeOf($1))
		tmp := cmp.getTmp()
		cmp.exec("$"+tmp + " += " + $1)
		cmp.unexec("$"+tmp + " -= " + $1)
		$$ = tmp
	}	
}
		| IDENT LSB expr RSB
{
	if !prep{
		cmp.addUsedVar($1)
		T_INTA.match(cmp.typeOf($1))
		tmp := cmp.getTmp()
		cmp.exec("$"+tmp + " += " + $1 + "[" + $3 + "]")
		cmp.unexec("$"+tmp + " -= " + $1 + "[" + $3 + "]")
		$$ = tmp
	}	
}
%%