//line sql.y:2
package sql

import __yyfmt__ "fmt"

//line sql.y:2
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
	typ  int
	val  string
	sexp exprlist
}

type exprlist []*expr

/* construct an atomic expr */
func atomic(typ int, val string) *expr {
	return &expr{
		typ: typ,
		val: val,
	}
}

/* construct a funcall expr */
func funcall(name string, oprd exprlist) *expr {
	return &expr{
		sexp: append(exprlist{atomic(IDENT, name)}, oprd...),
	}
}

/* construct a unary expr */
func unary(typ int, op string, od1 *expr) *expr {
	return &expr{
		sexp: append(exprlist{atomic(typ, op)}, od1),
	}
}

/* construct a binary expr */
func binary(typ int, od1 *expr, op string, od2 *expr) *expr {
	return &expr{
		sexp: append(exprlist{atomic(typ, op)}, od1, od2),
	}
}

/* construct a variadic expr */
func variadic(typ int, op string, ods exprlist) *expr {
	return &expr{
		sexp: append(exprlist{atomic(typ, op)}, ods...),
	}
}

type extendedSelect struct {
	extended bool
	train    bool
	standardSelect
	trainClause
	inferClause
}

type standardSelect struct {
	fields []string
	tables []string
	where  *expr
	limit  string
}

type trainClause struct {
	estimator string
	attrs     attrs
	columns   exprlist
	label     string
	save      string
}

type attrs map[string]*expr

type inferClause struct {
	model string
}

var parseResult extendedSelect

func attrsUnion(as1, as2 attrs) attrs {
	for k, v := range as2 {
		if _, ok := as1[k]; ok {
			log.Panicf("attr %q already specified", as2)
		}
		as1[k] = v
	}
	return as1
}

//line sql.y:101
type sqlSymType struct {
	yys  int
	val  string /* NUMBER, IDENT, STRING, and keywords */
	flds []string
	tbls []string
	expr *expr
	expl exprlist
	atrs attrs
	eslt extendedSelect
	slct standardSelect
	tran trainClause
	infr inferClause
}

const SELECT = 57346
const FROM = 57347
const WHERE = 57348
const LIMIT = 57349
const TRAIN = 57350
const INFER = 57351
const WITH = 57352
const COLUMN = 57353
const LABEL = 57354
const INTO = 57355
const IDENT = 57356
const NUMBER = 57357
const STRING = 57358
const AND = 57359
const OR = 57360
const GE = 57361
const LE = 57362
const NOT = 57363
const POWER = 57364
const UMINUS = 57365

var sqlToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"SELECT",
	"FROM",
	"WHERE",
	"LIMIT",
	"TRAIN",
	"INFER",
	"WITH",
	"COLUMN",
	"LABEL",
	"INTO",
	"IDENT",
	"NUMBER",
	"STRING",
	"AND",
	"OR",
	"'>'",
	"'<'",
	"'='",
	"GE",
	"LE",
	"'+'",
	"'-'",
	"'*'",
	"'/'",
	"'%'",
	"NOT",
	"POWER",
	"UMINUS",
	"';'",
	"','",
	"'('",
	"')'",
	"'['",
	"']'",
}
var sqlStatenames = [...]string{}

const sqlEofCode = 1
const sqlErrCode = 2
const sqlInitialStackSize = 16

//line sql.y:247

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
	r := "SELECT " + strings.Join(s.fields, ", ") +
		"\n FROM" + strings.Join(s.tables, ", ")
	if s.where != nil {
		r += fmt.Sprintf("\n WHERE %s", s.where)
	}
	if len(s.limit) > 0 {
		r += fmt.Sprintf("\n LIMIT %s", s.limit)
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
		ks = append(ks, "\""+jsonString(e.String())+"\"")
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

func (s inferClause) JSON() string {
	fmter := `{
"model":"%s"
}`
	return fmt.Sprintf(fmter, "\""+s.model+"\"")
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
"inferClause": %s
}`
	if s.extended {
		if s.train {
			return fmt.Sprintf(tf, s.extended, s.train,
				jsonString(s.standardSelect.String()), s.trainClause.JSON())
		} else {
			return fmt.Sprintf(nf, s.extended, s.train,
				jsonString(s.standardSelect.String()), s.inferClause.JSON())
		}
	}
	return fmt.Sprintf(bf, s.extended, s.train, jsonString(s.standardSelect.String()))
}

func Parse(s string) string {
	defer func() {
		if e := recover(); e != nil {
			log.Fatal(e)
		}
	}()

	sqlParse(newLexer(s))
	return parseResult.JSON()
}

//line yacctab:1
var sqlExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const sqlPrivate = 57344

const sqlLast = 157

var sqlAct = [...]int{

	52, 26, 82, 74, 51, 71, 71, 33, 76, 72,
	20, 46, 44, 45, 41, 40, 39, 43, 42, 34,
	35, 36, 37, 38, 32, 16, 47, 80, 48, 49,
	70, 88, 15, 19, 78, 56, 57, 58, 59, 60,
	61, 62, 63, 64, 65, 66, 67, 22, 21, 23,
	93, 69, 89, 22, 21, 23, 79, 90, 28, 84,
	75, 55, 27, 54, 28, 31, 30, 25, 27, 29,
	50, 83, 77, 25, 68, 29, 36, 37, 38, 18,
	85, 87, 14, 86, 22, 21, 23, 92, 53, 3,
	73, 85, 91, 81, 13, 28, 24, 17, 12, 27,
	6, 5, 2, 1, 25, 0, 29, 44, 45, 41,
	40, 39, 43, 42, 34, 35, 36, 37, 38, 41,
	40, 39, 43, 42, 34, 35, 36, 37, 38, 7,
	9, 8, 10, 11, 34, 35, 36, 37, 38, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 4,
}
var sqlPact = [...]int{

	85, -1000, 124, 68, -1000, 0, -7, 65, 18, 70,
	52, 51, -9, -1000, -1000, -1000, -1000, -26, -1000, -1000,
	90, -1000, -23, -1000, -1000, 70, -1000, 70, 70, 33,
	78, -1000, 49, 47, 70, 70, 70, 70, 70, 70,
	70, 70, 70, 70, 70, 70, 39, -5, -1000, -1000,
	-1000, -28, 90, 46, -1000, -1000, 50, 50, -1000, -1000,
	-1000, 110, 110, 110, 110, 110, 100, 100, -1000, -27,
	-1000, 70, -1000, 23, -1000, 6, -1000, 90, 45, 46,
	70, 19, -1000, -1000, -23, -1000, -1000, 90, 43, 45,
	74, -1000, 36, -1000,
}
var sqlPgo = [...]int{

	0, 103, 102, 101, 100, 98, 97, 0, 1, 2,
	4, 96, 93, 3, 90,
}
var sqlR1 = [...]int{

	0, 1, 1, 1, 2, 2, 2, 2, 3, 4,
	5, 5, 5, 9, 9, 9, 12, 12, 6, 6,
	13, 14, 14, 8, 8, 10, 10, 11, 11, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7,
	7, 7, 7, 7, 7, 7, 7, 7, 7,
}
var sqlR2 = [...]int{

	0, 2, 3, 3, 2, 3, 3, 3, 10, 2,
	1, 1, 3, 1, 1, 1, 1, 3, 1, 3,
	3, 1, 3, 3, 4, 1, 3, 2, 3, 1,
	1, 1, 1, 3, 1, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 2, 2,
}
var sqlChk = [...]int{

	-1000, -1, -2, 4, 32, -3, -4, 5, 7, 6,
	8, 9, -5, 26, 14, 32, 32, -6, 14, 15,
	-7, 15, 14, 16, -11, 34, -8, 29, 25, 36,
	14, 14, 33, 33, 24, 25, 26, 27, 28, 21,
	20, 19, 23, 22, 17, 18, 34, -7, -7, -7,
	37, -10, -7, 10, 14, 14, -7, -7, -7, -7,
	-7, -7, -7, -7, -7, -7, -7, -7, 35, -10,
	35, 33, 37, -14, -13, 14, 35, -7, 11, 33,
	21, -12, -9, 26, 14, -8, -13, -7, 12, 33,
	14, -9, 13, 14,
}
var sqlDef = [...]int{

	0, -2, 0, 0, 1, 0, 0, 0, 0, 0,
	0, 0, 4, 10, 11, 2, 3, 5, 18, 6,
	7, 29, 30, 31, 32, 0, 34, 0, 0, 0,
	0, 9, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 47, 48,
	27, 0, 25, 0, 12, 19, 35, 36, 37, 38,
	39, 40, 41, 42, 43, 44, 45, 46, 23, 0,
	33, 0, 28, 0, 21, 0, 24, 26, 0, 0,
	0, 0, 16, 13, 14, 15, 22, 20, 0, 0,
	0, 17, 0, 8,
}
var sqlTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 28, 3, 3,
	34, 35, 26, 24, 33, 25, 3, 27, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 32,
	20, 21, 19, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 36, 3, 37,
}
var sqlTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 22, 23, 29,
	30, 31,
}
var sqlTok3 = [...]int{
	0,
}

var sqlErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

//line yaccpar:1

/*	parser for yacc output	*/

var (
	sqlDebug        = 0
	sqlErrorVerbose = false
)

type sqlLexer interface {
	Lex(lval *sqlSymType) int
	Error(s string)
}

type sqlParser interface {
	Parse(sqlLexer) int
	Lookahead() int
}

type sqlParserImpl struct {
	lval  sqlSymType
	stack [sqlInitialStackSize]sqlSymType
	char  int
}

func (p *sqlParserImpl) Lookahead() int {
	return p.char
}

func sqlNewParser() sqlParser {
	return &sqlParserImpl{}
}

const sqlFlag = -1000

func sqlTokname(c int) string {
	if c >= 1 && c-1 < len(sqlToknames) {
		if sqlToknames[c-1] != "" {
			return sqlToknames[c-1]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func sqlStatname(s int) string {
	if s >= 0 && s < len(sqlStatenames) {
		if sqlStatenames[s] != "" {
			return sqlStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func sqlErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !sqlErrorVerbose {
		return "syntax error"
	}

	for _, e := range sqlErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + sqlTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := sqlPact[state]
	for tok := TOKSTART; tok-1 < len(sqlToknames); tok++ {
		if n := base + tok; n >= 0 && n < sqlLast && sqlChk[sqlAct[n]] == tok {
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}
	}

	if sqlDef[state] == -2 {
		i := 0
		for sqlExca[i] != -1 || sqlExca[i+1] != state {
			i += 2
		}

		// Look for tokens that we accept or reduce.
		for i += 2; sqlExca[i] >= 0; i += 2 {
			tok := sqlExca[i]
			if tok < TOKSTART || sqlExca[i+1] == 0 {
				continue
			}
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if sqlExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += sqlTokname(tok)
	}
	return res
}

func sqllex1(lex sqlLexer, lval *sqlSymType) (char, token int) {
	token = 0
	char = lex.Lex(lval)
	if char <= 0 {
		token = sqlTok1[0]
		goto out
	}
	if char < len(sqlTok1) {
		token = sqlTok1[char]
		goto out
	}
	if char >= sqlPrivate {
		if char < sqlPrivate+len(sqlTok2) {
			token = sqlTok2[char-sqlPrivate]
			goto out
		}
	}
	for i := 0; i < len(sqlTok3); i += 2 {
		token = sqlTok3[i+0]
		if token == char {
			token = sqlTok3[i+1]
			goto out
		}
	}

out:
	if token == 0 {
		token = sqlTok2[1] /* unknown char */
	}
	if sqlDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", sqlTokname(token), uint(char))
	}
	return char, token
}

func sqlParse(sqllex sqlLexer) int {
	return sqlNewParser().Parse(sqllex)
}

func (sqlrcvr *sqlParserImpl) Parse(sqllex sqlLexer) int {
	var sqln int
	var sqlVAL sqlSymType
	var sqlDollar []sqlSymType
	_ = sqlDollar // silence set and not used
	sqlS := sqlrcvr.stack[:]

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	sqlstate := 0
	sqlrcvr.char = -1
	sqltoken := -1 // sqlrcvr.char translated into internal numbering
	defer func() {
		// Make sure we report no lookahead when not parsing.
		sqlstate = -1
		sqlrcvr.char = -1
		sqltoken = -1
	}()
	sqlp := -1
	goto sqlstack

ret0:
	return 0

ret1:
	return 1

sqlstack:
	/* put a state and value onto the stack */
	if sqlDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", sqlTokname(sqltoken), sqlStatname(sqlstate))
	}

	sqlp++
	if sqlp >= len(sqlS) {
		nyys := make([]sqlSymType, len(sqlS)*2)
		copy(nyys, sqlS)
		sqlS = nyys
	}
	sqlS[sqlp] = sqlVAL
	sqlS[sqlp].yys = sqlstate

sqlnewstate:
	sqln = sqlPact[sqlstate]
	if sqln <= sqlFlag {
		goto sqldefault /* simple state */
	}
	if sqlrcvr.char < 0 {
		sqlrcvr.char, sqltoken = sqllex1(sqllex, &sqlrcvr.lval)
	}
	sqln += sqltoken
	if sqln < 0 || sqln >= sqlLast {
		goto sqldefault
	}
	sqln = sqlAct[sqln]
	if sqlChk[sqln] == sqltoken { /* valid shift */
		sqlrcvr.char = -1
		sqltoken = -1
		sqlVAL = sqlrcvr.lval
		sqlstate = sqln
		if Errflag > 0 {
			Errflag--
		}
		goto sqlstack
	}

sqldefault:
	/* default state action */
	sqln = sqlDef[sqlstate]
	if sqln == -2 {
		if sqlrcvr.char < 0 {
			sqlrcvr.char, sqltoken = sqllex1(sqllex, &sqlrcvr.lval)
		}

		/* look through exception table */
		xi := 0
		for {
			if sqlExca[xi+0] == -1 && sqlExca[xi+1] == sqlstate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			sqln = sqlExca[xi+0]
			if sqln < 0 || sqln == sqltoken {
				break
			}
		}
		sqln = sqlExca[xi+1]
		if sqln < 0 {
			goto ret0
		}
	}
	if sqln == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			sqllex.Error(sqlErrorMessage(sqlstate, sqltoken))
			Nerrs++
			if sqlDebug >= 1 {
				__yyfmt__.Printf("%s", sqlStatname(sqlstate))
				__yyfmt__.Printf(" saw %s\n", sqlTokname(sqltoken))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for sqlp >= 0 {
				sqln = sqlPact[sqlS[sqlp].yys] + sqlErrCode
				if sqln >= 0 && sqln < sqlLast {
					sqlstate = sqlAct[sqln] /* simulate a shift of "error" */
					if sqlChk[sqlstate] == sqlErrCode {
						goto sqlstack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if sqlDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", sqlS[sqlp].yys)
				}
				sqlp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if sqlDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", sqlTokname(sqltoken))
			}
			if sqltoken == sqlEofCode {
				goto ret1
			}
			sqlrcvr.char = -1
			sqltoken = -1
			goto sqlnewstate /* try again in the same state */
		}
	}

	/* reduction by production sqln */
	if sqlDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", sqln, sqlStatname(sqlstate))
	}

	sqlnt := sqln
	sqlpt := sqlp
	_ = sqlpt // guard against "declared and not used"

	sqlp -= sqlR2[sqln]
	// sqlp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
	if sqlp+1 >= len(sqlS) {
		nyys := make([]sqlSymType, len(sqlS)*2)
		copy(nyys, sqlS)
		sqlS = nyys
	}
	sqlVAL = sqlS[sqlp+1]

	/* consult goto table to find next state */
	sqln = sqlR1[sqln]
	sqlg := sqlPgo[sqln]
	sqlj := sqlg + sqlS[sqlp].yys + 1

	if sqlj >= sqlLast {
		sqlstate = sqlAct[sqlg]
	} else {
		sqlstate = sqlAct[sqlj]
		if sqlChk[sqlstate] != -sqln {
			sqlstate = sqlAct[sqlg]
		}
	}
	// dummy call; replaced with literal code
	switch sqlnt {

	case 1:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:139
		{
			parseResult.extended = false
			parseResult.standardSelect = sqlDollar[1].slct
		}
	case 2:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:143
		{
			parseResult.extended = true
			parseResult.train = true
			parseResult.standardSelect = sqlDollar[1].slct
			parseResult.trainClause = sqlDollar[2].tran
		}
	case 3:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:149
		{
			parseResult.extended = true
			parseResult.train = false
			parseResult.standardSelect = sqlDollar[1].slct
			parseResult.inferClause = sqlDollar[2].infr
		}
	case 4:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:158
		{
			sqlVAL.slct.fields = sqlDollar[2].flds
		}
	case 5:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:159
		{
			sqlVAL.slct.tables = sqlDollar[3].tbls
		}
	case 6:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:160
		{
			sqlVAL.slct.limit = sqlDollar[3].val
		}
	case 7:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:161
		{
			sqlVAL.slct.where = sqlDollar[3].expr
		}
	case 8:
		sqlDollar = sqlS[sqlpt-10 : sqlpt+1]
		//line sql.y:165
		{
			sqlVAL.tran.estimator = sqlDollar[2].val
			sqlVAL.tran.attrs = sqlDollar[4].atrs
			sqlVAL.tran.columns = sqlDollar[6].expl
			sqlVAL.tran.label = sqlDollar[8].val
			sqlVAL.tran.save = sqlDollar[10].val
		}
	case 9:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:175
		{
			sqlVAL.infr.model = sqlDollar[2].val
		}
	case 10:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:179
		{
			sqlVAL.flds = sqlVAL.flds[:0]
		}
	case 11:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:180
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[1].val)
		}
	case 12:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:181
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[3].val)
		}
	case 13:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:185
		{
			sqlVAL.expr = atomic(IDENT, "*")
		}
	case 14:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:186
		{
			sqlVAL.expr = atomic(IDENT, sqlDollar[1].val)
		}
	case 15:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:187
		{
			sqlVAL.expr = sqlDollar[1].expr
		}
	case 16:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:191
		{
			sqlVAL.expl = exprlist{sqlDollar[1].expr}
		}
	case 17:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:192
		{
			sqlVAL.expl = append(sqlDollar[1].expl, sqlDollar[3].expr)
		}
	case 18:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:196
		{
			sqlVAL.tbls = []string{sqlDollar[1].val}
		}
	case 19:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:197
		{
			sqlVAL.tbls = append(sqlDollar[1].tbls, sqlDollar[3].val)
		}
	case 20:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:201
		{
			sqlVAL.atrs = attrs{sqlDollar[1].val: sqlDollar[3].expr}
		}
	case 21:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:205
		{
			sqlVAL.atrs = sqlDollar[1].atrs
		}
	case 22:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:206
		{
			sqlVAL.atrs = attrsUnion(sqlDollar[1].atrs, sqlDollar[3].atrs)
		}
	case 23:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:210
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, nil)
		}
	case 24:
		sqlDollar = sqlS[sqlpt-4 : sqlpt+1]
		//line sql.y:211
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, sqlDollar[3].expl)
		}
	case 25:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:215
		{
			sqlVAL.expl = exprlist{sqlDollar[1].expr}
		}
	case 26:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:216
		{
			sqlVAL.expl = append(sqlDollar[1].expl, sqlDollar[3].expr)
		}
	case 27:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:220
		{
			sqlVAL.expl = nil
		}
	case 28:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:221
		{
			sqlVAL.expl = sqlDollar[2].expl
		}
	case 29:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:225
		{
			sqlVAL.expr = atomic(NUMBER, sqlDollar[1].val)
		}
	case 30:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:226
		{
			sqlVAL.expr = atomic(IDENT, sqlDollar[1].val)
		}
	case 31:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:227
		{
			sqlVAL.expr = atomic(STRING, sqlDollar[1].val)
		}
	case 32:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:228
		{
			sqlVAL.expr = variadic('[', "square", sqlDollar[1].expl)
		}
	case 33:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:229
		{
			sqlVAL.expr = unary('(', "paren", sqlDollar[2].expr)
		}
	case 34:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:230
		{
			sqlVAL.expr = sqlDollar[1].expr
		}
	case 35:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:231
		{
			sqlVAL.expr = binary('+', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 36:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:232
		{
			sqlVAL.expr = binary('-', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 37:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:233
		{
			sqlVAL.expr = binary('*', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 38:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:234
		{
			sqlVAL.expr = binary('/', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 39:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:235
		{
			sqlVAL.expr = binary('%', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 40:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:236
		{
			sqlVAL.expr = binary('=', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 41:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:237
		{
			sqlVAL.expr = binary('<', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 42:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:238
		{
			sqlVAL.expr = binary('>', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 43:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:239
		{
			sqlVAL.expr = binary(LE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 44:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:240
		{
			sqlVAL.expr = binary(GE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 45:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:241
		{
			sqlVAL.expr = binary(AND, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 46:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:242
		{
			sqlVAL.expr = binary(OR, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 47:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:243
		{
			sqlVAL.expr = unary(NOT, sqlDollar[1].val, sqlDollar[2].expr)
		}
	case 48:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:244
		{
			sqlVAL.expr = unary('-', sqlDollar[1].val, sqlDollar[2].expr)
		}
	}
	goto sqlstack /* stack new state and value */
}
