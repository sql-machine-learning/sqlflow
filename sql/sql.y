%{
	package sql

	import (
		"fmt"
		"log"
		"sort"
		"strings"
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
		Extended bool
		Train    bool
		StandardSelect
		TrainClause
		InferClause
	}

	type StandardSelect struct {
		fields []string
		tables []string
		where *expr
		limit string
	}

	type TrainClause struct {
		Estimator string
		Attrs     Attrs
		columns   exprlist
		Save      string
	}

	type Attrs map[string]*expr

	type InferClause struct {
		Model  string
	}

	var parseResult extendedSelect

	func attrsUnion(as1, as2 Attrs) Attrs {
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
  atrs Attrs
  eslt extendedSelect
  slct StandardSelect
  tran TrainClause
  infr InferClause
}

%type  <eslt> select_stmt
%type  <slct> select
%type  <tran> train_clause
%type  <infr> infer_clause
%type  <flds> fields
%type  <tbls> tables
%type  <expr> expr funcall column
%type  <expl> exprlist pythonlist columns
%type  <atrs> attr
%type  <atrs> Attrs

%token <val> SELECT FROM WHERE LIMIT TRAIN INFER WITH COLUMN INTO
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
    parseResult.Extended = false
    parseResult.StandardSelect = $1
  }
| select train_clause ';' {
    parseResult.Extended = true
    parseResult.Train = true
    parseResult.StandardSelect = $1
    parseResult.TrainClause = $2
  }
| select infer_clause ';' {
    parseResult.Extended = true
    parseResult.Train = false
    parseResult.StandardSelect = $1
    parseResult.InferClause = $2
  }
;

select
: SELECT fields         { $$.fields = $2 }
| select FROM tables    { $$.tables = $3 }
| select LIMIT NUMBER   { $$.limit = $3 }
| select WHERE expr     { $$.where = $3 }
;

train_clause
: TRAIN IDENT WITH Attrs COLUMN columns INTO IDENT {
    $$.Estimator = $2
    $$.Attrs = $4
    $$.columns = $6
    $$.Save = $8
  }
;

infer_clause
: INFER IDENT      { $$.Model = $2 }
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
: column             { $$ = exprlist{$1}     }
| columns ',' column { $$ = append($1, $3) }
;

tables
: IDENT            { $$ = []string{$1} }
| tables ',' IDENT { $$ = append($1, $3) }
;

attr
: IDENT '=' expr    { $$ = Attrs{$1 : $3} }
;

Attrs
: attr              { $$ = $1 }
| Attrs ',' attr    { $$ = attrsUnion($1, $3) }
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

func (s StandardSelect) String() string {
	r := "SELECT " + strings.Join(s.fields, ", ") +
		" FROM " + strings.Join(s.tables, ", ")
	if s.where != nil {
		r += fmt.Sprintf(" WHERE %s", s.where)
	}
	if len(s.limit) > 0 {
		r += fmt.Sprintf(" LIMIT %s", s.limit)
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

func (ats Attrs) JSON() string {
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

func (s TrainClause) JSON() string {
	fmter := `{
"Estimator": "%s",
"Attrs": %s,
"columns": %s,
"Save": "%s"
}`
	return fmt.Sprintf(fmter, s.Estimator, s.Attrs.JSON(), s.columns.JSON(), s.Save)
}

func (s InferClause) JSON() string {
	fmter := `{
"Model":"%s"
}`
	return fmt.Sprintf(fmter, "\"" + s.Model + "\"")
}

func (s extendedSelect) JSON() string {
	bf := `{
"Extended": %t,
"Train": %t,
"StandardSelect": "%s"
}`
	tf := `{
"Extended": %t,
"Train": %t,
"StandardSelect": "%s",
"TrainClause": %s
}`
	nf := `{
"Extended": %t,
"Train": %t,
"StandardSelect": "%s",
"InferClause": %s
}`
	if s.Extended {
		if s.Train {
			return fmt.Sprintf(tf, s.Extended, s.Train,
				jsonString(s.StandardSelect.String()), s.TrainClause.JSON())
		} else {
			return fmt.Sprintf(nf, s.Extended, s.Train,
				jsonString(s.StandardSelect.String()), s.InferClause.JSON())
		}
	}
	return fmt.Sprintf(bf, s.Extended, s.Train, jsonString(s.StandardSelect.String()))
}

func Parse(s string) extendedSelect {
	defer func() {
		if e := recover(); e != nil {
			log.Fatal(e)
		}
	}()
	sqlParse(newLexer(s))
    return parseResult
}
