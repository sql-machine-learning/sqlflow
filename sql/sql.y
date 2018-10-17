%{
  package sql

  import (
    "io"
    "fmt"
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

  func operator(typ int) expr {
    return atomic(typ, "")
  }
    
  /* construct a funcall expr */
  func funcall(name string, oprd []expr) expr {
    return expr{
      sexp : append([]expr{atomic(IDENT, name)}, oprd...),
    }
  }

  /* construct a binary expr */
  func binary(typ int, od1, od2 expr) expr {
    return expr{
      sexp : append([]expr{operator(typ)}, od1, od2),
    }
  }

  /* construct a unary expr */
  func unary(typ int, od1 expr) expr {
    return expr{
      sexp : append([]expr{operator(typ)}, od1),
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

%left AND OR
%left '>' '<' '=' GE LE POWER
%left '+' '-'
%left '*' '/' '%'
%left NOT
%left UMINUS

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
| '(' expr ')'   { $$ = $2 }
| funcall        { $$ = $1 }
| expr '+' expr  { $$ = binary('+', $1, $3) }
| expr '-' expr  { $$ = binary('-', $1, $3) }
| expr '*' expr  { $$ = binary('*', $1, $3) }
| expr '/' expr  { $$ = binary('/', $1, $3) }
| expr '%' expr  { $$ = binary('%', $1, $3) }
| expr '=' expr  { $$ = binary('=', $1, $3) }
| expr '<' expr  { $$ = binary('<', $1, $3) }
| expr '>' expr  { $$ = binary('>', $1, $3) }
| expr LE  expr  { $$ = binary(LE,  $1, $3) }
| expr GE  expr  { $$ = binary(GE,  $1, $3) }
| expr AND expr  { $$ = binary(AND, $1, $3) }
| expr OR  expr  { $$ = binary(OR,  $1, $3) }
| NOT expr %prec NOT    { $$ = unary(NOT, $2) }
| '-' expr %prec UMINUS { $$ = unary('-', $2) }
;

%%

func indent(w io.Writer, indentLevel int) {
    for i := 0; i < indentLevel; i++ {
        fmt.Fprintf(w, " ")
    }
}    
 
func (e expr) printf(w io.Writer, indentLevel int) {
    indent(w, indentLevel)

    if e.typ == 0 /* atomic expr */ {
        fmt.Fprintf(w, "%s", e.val)
    }
    /* try to finish */
}
