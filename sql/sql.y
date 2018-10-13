%{

  package sql

  type expr struct {
    typ int             /* NUMBER, IDENT, STRING, or operator */
    oprd []expr         /* if typ is an operator */
    val string          /* if typ is not an operator */
  }
    
  type selectStmt struct {
    fields []string
    tables []string
    where expr
    limit string
  }

  var parseResult selectStmt
%}

%union {
  val string  /* NUMBER, IDENT, STRING, and keywords */
  flds []string
  tbls []string
  expr expr
  slct selectStmt
}

%type  <slct> select select_stmt
%type  <flds> fields
%type  <tbls> tables
%type  <expr> expr

%token <val> SELECT FROM WHERE LIMIT TRAIN COLUMN
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
;

fields
: '*'              { $$ = $$[:0] }
| IDENT            { $$ = append($$, $1) }
| fields ',' IDENT { $$ = append($$, $3) }
;

tables
: IDENT            { $$ = append($$, $1) }
| tables ',' IDENT { $$ = append($$, $3) }
;

expr
: NUMBER         { $$ = expr{typ : NUMBER, val : $1} }
| IDENT          { $$ = expr{typ : IDENT,  val : $1} }
| STRING         { $$ = expr{typ : STRING, val : $1} }
| '(' expr ')'   { $$ = $2 }
| expr '+' expr  { $$ = expr{typ : '+', oprd : []expr{$1, $3}} }
| expr '-' expr  { $$ = expr{typ : '-', oprd : []expr{$1, $3}} }
| expr '*' expr  { $$ = expr{typ : '*', oprd : []expr{$1, $3}} }
| expr '/' expr  { $$ = expr{typ : '/', oprd : []expr{$1, $3}} }
| expr '%' expr  { $$ = expr{typ : '%', oprd : []expr{$1, $3}} }
| expr '=' expr  { $$ = expr{typ : '=', oprd : []expr{$1, $3}} }
| expr '<' expr  { $$ = expr{typ : '<', oprd : []expr{$1, $3}} }
| expr '>' expr  { $$ = expr{typ : '>', oprd : []expr{$1, $3}} }
| expr LE  expr  { $$ = expr{typ : LE,  oprd : []expr{$1, $3}} }
| expr GE  expr  { $$ = expr{typ : GE,  oprd : []expr{$1, $3}} }
| expr AND expr  { $$ = expr{typ : AND, oprd : []expr{$1, $3}} }
| expr OR  expr  { $$ = expr{typ : OR,  oprd : []expr{$1, $3}} }
| NOT expr %prec NOT    { $$ = expr{typ : NOT, oprd : []expr{$2}} }
| '-' expr %prec UMINUS { $$ = expr{typ : '-', oprd : []expr{$2}} }
;
%%
