%{

package sql

type selec struct {
  fields []string
  tables []string
  where *expr
  limit int
}

type expression struct {
  optr int
  oprd []*expr  /* valid if optr >= 0; */
  val string    /* valid if optr < 0 */
}

%}

%union {
  val string  /* both NUMBER and IDENT have value as string. */
  expr *expression
  sel selec
}


%token  <sel>           SELECT FROM
%token  <expr>          WHERE
%token  <str>           IDENT NUMBER

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
