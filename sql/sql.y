%{
  package sql

  import (
    "bytes"
    "fmt"
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

  /* construct a variadic expr */
  func variadic(typ int, op string, ods []expr) expr {
    return expr{
      sexp : append([]expr{atomic(typ, op)}, ods...),
    }
  }
    
  type selectStmt struct {
    fields []string
    tables []string
    where expr
    limit string
    estimator string
    attrs map[string]expr
    columns []expr
    into string
  }

  var parseResult selectStmt

  func attrsUnion(as1, as2 map[string]expr) map[string]expr {
      for k, v := range as2 {
          if _, ok := as1[k]; ok {
              log.Panicf("attr %q already specified", as2)
          }
          as1[k] = v
      }
      return as1
  }
%}

%union {
  val string  /* NUMBER, IDENT, STRING, and keywords */
  flds []string
  tbls []string
  expr expr
  expl []expr
  atrs map[string]expr
  slct selectStmt
}

%type  <slct> select select_stmt
%type  <flds> fields
%type  <tbls> tables
%type  <expr> expr funcall column
%type  <expl> exprlist pythonlist columns
%type  <atrs> attr
%type  <atrs> attrs

%token <val> SELECT FROM WHERE LIMIT TRAIN WITH COLUMN INTO
%token <val> IDENT NUMBER STRING

%left <val> AND OR
%left <val> '>' '<' '=' GE LE 
%left <val> '+' '-'
%left <val> '*' '/' '%'
%left <val> NOT
%left <val> POWER  /* think about the example "NOT base ** -3" */
%left <val> UMINUS 

%%

select_stmt
: select ';' { parseResult = $1 }
      
select
: SELECT fields         { $$.fields = $2 }
| select FROM tables    { $$.tables = $3 }
| select LIMIT NUMBER   { $$.limit = $3 }
| select WHERE expr     { $$.where = $3 }
| select TRAIN IDENT    { $$.estimator = $3 }
| select WITH attrs     { $$.attrs = $3 }
| select COLUMN columns { $$.columns = $3 }
| select INTO IDENT     { $$.into = $3 }
;

fields
: '*'              { $$ = $$[:0] }
| IDENT            { $$ = append($$, $1) }
| fields ',' IDENT { $$ = append($$, $3) }
;

column
: '*'     { $$ = atomic(IDENT, "*") }
| IDENT   { $$ = atomic(IDENT, $1)  }
| funcall { $$ = $1 }
;

columns
: column             { $$ = []expr{$1}     }
| columns ',' column { $$ = append($1, $3) }
;
      
tables
: IDENT            { $$ = []string{$1} }
| tables ',' IDENT { $$ = append($1, $3) }
;

attr
: IDENT '=' expr    { $$ = map[string]expr{$1 : $3} }
;

attrs
: attr              { $$ = $1 }
| attrs ',' attr    { $$ = attrsUnion($1, $3) }
;
      
funcall
: IDENT '(' ')'          { $$ = funcall($1, nil) }
| IDENT '(' exprlist ')' { $$ = funcall($1, $3)  }
;

exprlist
: expr              { $$ = []expr{$1}     }
| exprlist ',' expr { $$ = append($1, $3) }
;

pythonlist
: '[' ']'           { $$ = nil }
| '[' exprlist ']'  { $$ = $2  }
;

expr
: NUMBER         { $$ = atomic(NUMBER, $1) }
| IDENT          { $$ = atomic(IDENT, $1)  }
| STRING         { $$ = atomic(STRING, $1) }
| pythonlist     { $$ = variadic('[', "square", $1) }
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

func (e expr) String() string {
    var w bytes.Buffer

    if e.typ == 0 { /* a compound expression */ 
        switch e.sexp[0].typ {
        case '+', '*', '/', '%', '=', '<', '>', LE, GE, AND, OR:
            if len(e.sexp) != 3 {
		log.Panicf("Expecting binary expression, got %.10q", e.sexp)
	    }
	    return fmt.Sprintf("%s %s %s", e.sexp[1], e.sexp[0].val, e.sexp[2])
        case '-':
	    switch len(e.sexp) {
	    case 2:
	        return fmt.Sprintf(" -%s", e.sexp[1])
	    case 3:
	        return fmt.Sprintf("%s - %s", e.sexp[1], e.sexp[2])
	    default:
	        log.Panicf("Expecting either unary or binary -, got %.10q", e.sexp)
	    }
	case '(':
	    if len(e.sexp) != 2 {
		log.Panicf("Expecting ( ) as unary operator, got %.10q", e.sexp)
	    }
	    return fmt.Sprintf("(%s)", e.sexp[1])
	case '[':
	    fmt.Fprintf(&w, "[")
	    for i := 1; i < len(e.sexp); i++ {
	        fmt.Fprintf(&w, "%s", e.sexp[i])
	        if i < len(e.sexp) -1 {
		    fmt.Fprintf(&w, ", ")
		}
	    }
            fmt.Fprintf(&w, "]")
	    return w.String()
	case NOT:
	    return fmt.Sprintf("NOT %s", e.sexp[1])
	case IDENT: /* function call */
	    fmt.Fprintf(&w, "%s(", e.sexp[0].val)
	    for i := 1; i < len(e.sexp); i++ {
	        fmt.Fprintf(&w, "%s", e.sexp[i])
		if i < len(e.sexp) -1 {
		    fmt.Fprintf(&w, ", ")
		}
	    }
   	    fmt.Fprintf(&w, ")")
	    return w.String()
	}
    } else {
        return fmt.Sprintf("%s", e.val)
    }

    log.Panicf("Cannot print an unknown expression")
    return ""
}
