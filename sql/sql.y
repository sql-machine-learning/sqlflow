%{
  package sql

  import (
    "fmt"
    "io"
    "log"
  )

  /* expr defines an expression as a Lisp list.  If len(val)>0,
     it is an atomic expression, in particular, NUMBER, IDENT, 
     or STRING, defined by typ and val; otherwise, it is a 
     Lisp S-expression. */
  type expr struct {
    typ int
    val string    
    sexp []expr   
  }

  /* construct an atomic expr */
  func atomic(typ int, val string) expr {
    return expr{
      typ : typ,
      val : val,
    }
  }

  /* construct a funcall expr */
  func funcall(name string, oprd []expr) expr {
    return expr{
      sexp : append([]expr{atomic(IDENT, name)}, oprd...),
    }
  }

  /* construct a unary expr */
  func unary(typ int, op string, od1 expr) expr {
    return expr{
      sexp : append([]expr{atomic(typ, op)}, od1),
    }
  }
    
  /* construct a binary expr */
  func binary(typ int, od1 expr, op string, od2 expr) expr {
    return expr{
      sexp : append([]expr{atomic(typ, op)}, od1, od2),
    }
  }

  type selectStmt struct {
    fields []string
    tables []string
    where expr
    limit string
    estimator string
  }

  var parseResult selectStmt
%}

%union {
  val string  /* NUMBER, IDENT, STRING, and keywords */
  flds []string
  tbls []string
  expr expr
  expl []expr
  slct selectStmt
}

%type  <slct> select select_stmt
%type  <flds> fields
%type  <tbls> tables
%type  <expr> expr funcall
%type  <expl> exprlist

%token <val> SELECT FROM WHERE LIMIT TRAIN WITH COLUMN
%token <val> IDENT NUMBER STRING

%left <val> AND OR
%left <val> '>' '<' '=' GE LE POWER
%left <val> '+' '-'
%left <val> '*' '/' '%'
%left <val> NOT
%left <val> UMINUS

%%

select_stmt
: select ';' { parseResult = $1 }
      
select
: SELECT fields       { $$.fields = $2 }
| select FROM tables  { $$.tables = $3 }
| select LIMIT NUMBER { $$.limit = $3 }
| select WHERE expr   { $$.where = $3 }
| select TRAIN IDENT  { $$.estimator = $3 }
;

fields
: '*'              { $$ = $$[:0] }
| IDENT            { $$ = append($$, $1) }
| fields ',' IDENT { $$ = append($$, $3) }
;

tables
: IDENT            { $$ = []string{$1} }
| tables ',' IDENT { $$ = append($1, $3) }
;

funcall
: IDENT '(' ')'          { $$ = funcall($1, nil) }
| IDENT '(' exprlist ')' { $$ = funcall($1, $3) }
;
      
exprlist
: expr              { $$ = []expr{$1} }
| exprlist ',' expr { $$ = append($1, $3) }
;

expr
: NUMBER         { $$ = atomic(NUMBER, $1) }
| IDENT          { $$ = atomic(IDENT, $1) }
| STRING         { $$ = atomic(STRING, $1) }
| '(' expr ')'   { $$ = unary('(', "paren", $2) } /* take '(' as the operator */
| funcall        { $$ = $1 }
| expr '+' expr  { $$ = binary('+', $1, $2, $3) }
| expr '-' expr  { $$ = binary('-', $1, $2, $3) }
| expr '*' expr  { $$ = binary('*', $1, $2, $3) }
| expr '/' expr  { $$ = binary('/', $1, $2, $3) }
| expr '%' expr  { $$ = binary('%', $1, $2, $3) }
| expr '=' expr  { $$ = binary('=', $1, $2, $3) }
| expr '<' expr  { $$ = binary('<', $1, $2, $3) }
| expr '>' expr  { $$ = binary('>', $1, $2, $3) }
| expr LE  expr  { $$ = binary(LE,  $1, $2, $3) }
| expr GE  expr  { $$ = binary(GE,  $1, $2, $3) }
| expr AND expr  { $$ = binary(AND, $1, $2, $3) }
| expr OR  expr  { $$ = binary(OR,  $1, $2, $3) }
| NOT expr %prec NOT    { $$ = unary(NOT, $1, $2) }
| '-' expr %prec UMINUS { $$ = unary('-', $1, $2) }
;

%%

func (e expr) print(w io.Writer) {
    if e.typ == 0 { /* a compound expression */ 
        switch e.sexp[0].typ {
        case '+', '*', '/', '%', '=', '<', '>', LE, GE, AND, OR:
            if len(e.sexp) != 3 {
		log.Panicf("Expecting binary expression, got %.10q", e.sexp)
	    }
	    e.sexp[1].print(w)
	    fmt.Fprintf(w, " %s ", e.sexp[0].val)
	    e.sexp[2].print(w)
        case '-':
	    switch len(e.sexp) {
	    case 2:
	        fmt.Fprintf(w, " -")
		e.sexp[1].print(w)
	    case 3:
	        e.sexp[1].print(w)
	        fmt.Fprintf(w, " - ")
	        e.sexp[2].print(w)
	    default:
	        log.Panicf("Expecting either unary or binary -, got %.10q", e.sexp)
	    }
	case '(':
	    if len(e.sexp) != 2 {
		log.Panicf("Expecting ( ) as unary operator, got %.10q", e.sexp)
	    }
	    fmt.Fprintf(w, " (")
	    e.sexp[1].print(w)
	    fmt.Fprintf(w, ") ")
	case NOT:
	    fmt.Fprintf(w, " NOT ")
	    e.sexp[1].print(w)
	case IDENT: /* function call */
	    fmt.Fprintf(w, " %s(", e.sexp[0].val)
	    for i := 1; i < len(e.sexp); i++ {
	      e.sexp[i].print(w)
	    }
	}
    } else {
        fmt.Fprintf(w, "%s", e.val)
    } 
}
