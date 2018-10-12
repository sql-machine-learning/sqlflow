%{

package sql

type expression struct {
  optr int
  oprd []*expression  /* valid if optr >= 0; */
  val string    /* valid if optr < 0 */
}

type selectStmt struct {
  fields []string
  tables []string
  where *expression
  limit int
}

%}

%union {
  val string  /* NUMBER, IDENT, STRING, and keywords */
  expr *expression
  sel selectStmt
}


%token  <sel>           SELECT FROM WHERE LIMIT TRAIN COLUMN
%token  <str>           IDENT NUMBER

%left '>' '<' '=' GE LE POWER
%left '+' '-'
%left '*' '/' '%'
%left UMINUS

%%

select : SELECT fields FROM tables ';'
        |       SELECT fields FROM tables WHERE expr ';'
;

fields : '*'
        |       IDENT
        |       fields ',' IDENT
;

tables : IDENT
        |       tables ',' IDENT
;

expr : NUMBER
        |       IDENT
        |       expr '+' expr
        |       expr '-' expr
        |       '-' expr %prec UMINUS
;
%%
