%{
	package sql

	import (
		"fmt"
		"sort"
		"strings"
		"sync"
	)

	/* expr defines an expression as a Lisp list.  If len(val)>0,
           it is an atomic expression, in particular, NUMBER, IDENT,
           or STRING, defined by typ and val; otherwise, it is a
           Lisp S-expression. */
	type expr struct {
		typ int
		val string
		sexp exprlist
	}

	type exprlist []*expr

	/* construct an atomic expr */
	func atomic(typ int, val string) *expr {
		return &expr{
			typ : typ,
			val : val,
		}
	}

	/* construct a funcall expr */
	func funcall(name string, oprd exprlist) *expr {
		return &expr{
			sexp : append(exprlist{atomic(IDENT, name)}, oprd...),
		}
	}

	/* construct a unary expr */
	func unary(typ int, op string, od1 *expr) *expr {
		return &expr{
			sexp : append(exprlist{atomic(typ, op)}, od1),
		}
	}

	/* construct a binary expr */
	func binary(typ int, od1 *expr, op string, od2 *expr) *expr {
		return &expr{
			sexp : append(exprlist{atomic(typ, op)}, od1, od2),
		}
	}

	/* construct a variadic expr */
	func variadic(typ int, op string, ods exprlist) *expr {
		return &expr{
			sexp : append(exprlist{atomic(typ, op)}, ods...),
		}
	}

	type extendedSelect struct {
		extended bool
		train    bool
		standardSelect
		trainClause
		predictClause
	}

	type standardSelect struct {
		fields []string
		tables []string
		where *expr
		limit string
	}

	type trainClause struct {
		estimator string
		attrs     attrs
		columns   exprlist
                label     string
		save      string
	}

	type attrs map[string]*expr

	type predictClause struct {
		model  string
		into   string
	}

	var parseResult *extendedSelect

	func attrsUnion(as1, as2 attrs) attrs {
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
  expr *expr
  expl exprlist
  atrs attrs
  eslt extendedSelect
  slct standardSelect
  tran trainClause
  infr predictClause
}

%type  <eslt> select_stmt
%type  <slct> select
%type  <tran> train_clause
%type  <infr> predict_clause
%type  <flds> fields
%type  <tbls> tables
%type  <expr> expr funcall column
%type  <expl> exprlist pythonlist columns
%type  <atrs> attr
%type  <atrs> attrs

%token <val> SELECT FROM WHERE LIMIT TRAIN PREDICT WITH COLUMN LABEL USING INTO
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
: select ';' {
	parseResult = &extendedSelect{
		extended: false,
		standardSelect: $1}
  }
| select train_clause ';' {
	parseResult = &extendedSelect{
		extended: true,
		train: true,
		standardSelect: $1,
		trainClause: $2}
  }
| select predict_clause ';' {
	parseResult = &extendedSelect{
		extended: true,
		train: false,
		standardSelect: $1,
		predictClause: $2}
  }
;

select
: SELECT fields         { $$.fields = $2 }
| select FROM tables    { $$.tables = $3 }
| select LIMIT NUMBER   { $$.limit = $3 }
| select WHERE expr     { $$.where = $3 }
;

train_clause
: TRAIN IDENT WITH attrs COLUMN columns LABEL IDENT INTO IDENT {
	$$.estimator = $2
	$$.attrs = $4
	$$.columns = $6
	$$.label = $8
	$$.save = $10
  }
;

predict_clause
: PREDICT IDENT USING IDENT {
	$$.into = $2
	$$.model = $4
}
;

fields
: '*'              { $$ = append($$, $1) }
| IDENT            { $$ = append($$, $1) }
| fields ',' IDENT { $$ = append($$, $3) }
;

column
: '*'     { $$ = atomic(IDENT, "*") }
| IDENT   { $$ = atomic(IDENT, $1)  }
| funcall { $$ = $1 }
;

columns
: column             { $$ = exprlist{$1}     }
| columns ',' column { $$ = append($1, $3) }
;

tables
: IDENT            { $$ = []string{$1} }
| tables ',' IDENT { $$ = append($1, $3) }
;

attr
: IDENT '=' expr    { $$ = attrs{$1 : $3} }
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
: expr              { $$ = exprlist{$1}     }
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

/* Like Lisp's builtin function cdr. */
func (e *expr) cdr() (r []string) {
	for i := 1; i < len(e.sexp); i++ {
		r = append(r, e.sexp[i].String())
	}
	return r
}

func (e *expr) String() string {
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
			return "[" + strings.Join(e.cdr(), ", ") + "]"
		case NOT:
			return fmt.Sprintf("NOT %s", e.sexp[1])
		case IDENT: /* function call */
			return e.sexp[0].val + "(" + strings.Join(e.cdr(), ", ") + ")"
		}
	} else {
		return fmt.Sprintf("%s", e.val)
	}

	log.Panicf("Cannot print an unknown expression")
	return ""
}

func (s standardSelect) String() string {
	r := "SELECT "
	if len(s.fields) == 0 {
		r += "*"
	} else {
		r += strings.Join(s.fields, ", ")
	}
	r += "\nFROM " + strings.Join(s.tables, ", ")
	if s.where != nil {
		r += fmt.Sprintf("\nWHERE %s", s.where)
	}
	if len(s.limit) > 0 {
		r += fmt.Sprintf("\nLIMIT %s", s.limit)
	}
	return r + ";"
}

func jsonString(s string) string {
	return strings.Replace(
		strings.Replace(
			strings.Replace(s, "\n", "\\n", -1),
			"\r", "\\r", -1),
		"\"", "\\\"", -1)
}

func (ats attrs) JSON() string {
	ks := []string{}
	for k := range ats {
		ks = append(ks, k)
	}
	sort.Strings(ks) /* Remove the randomness of map traversal. */

	for i, k := range ks {
		ks[i] = fmt.Sprintf(`"%s": "%s"`, k, jsonString(ats[k].String()))
	}
	return "{\n" + strings.Join(ks, ",\n") + "\n}"
}

func (el exprlist) JSON() string {
	ks := []string{}
	for _, e := range el {
		ks = append(ks, "\"" + jsonString(e.String()) + "\"")
	}
	return "[\n" + strings.Join(ks, ",\n") + "\n]"
}

func (s trainClause) JSON() string {
	fmter := `{
"estimator": "%s",
"attrs": %s,
"columns": %s,
"save": "%s"
}`
	return fmt.Sprintf(fmter, s.estimator, s.attrs.JSON(), s.columns.JSON(), s.save)
}

func (s predictClause) JSON() string {
	fmter := `{
"model":"%s"
}`
	return fmt.Sprintf(fmter, "\"" + s.model + "\"")
}

func (s extendedSelect) JSON() string {
	bf := `{
"extended": %t,
"train": %t,
"standardSelect": "%s"
}`
	tf := `{
"extended": %t,
"train": %t,
"standardSelect": "%s",
"trainClause": %s
}`
	nf := `{
"extended": %t,
"train": %t,
"standardSelect": "%s",
"predictClause": %s
}`
	if s.extended {
		if s.train {
			return fmt.Sprintf(tf, s.extended, s.train,
				jsonString(s.standardSelect.String()), s.trainClause.JSON())
		} else {
			return fmt.Sprintf(nf, s.extended, s.train,
				jsonString(s.standardSelect.String()), s.predictClause.JSON())
		}
	}
	return fmt.Sprintf(bf, s.extended, s.train, jsonString(s.standardSelect.String()))
}

// sqlReentrantParser makes sqlParser, generated by goyacc and using a
// global variable parseResult to return the result, reentrant.
type sqlSyncParser struct {
	pr sqlParser
}

func newParser() *sqlSyncParser {
	return &sqlSyncParser{sqlNewParser()}
}

var mu sync.Mutex

func (p *sqlSyncParser) Parse(s string) (r *extendedSelect, e error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			e, ok = r.(error)
			if !ok {
				e = fmt.Errorf("%v", r)
			}
		}
	}()

	mu.Lock()
	defer mu.Unlock()

	p.pr.Parse(newLexer(s))
	return parseResult, nil
}
