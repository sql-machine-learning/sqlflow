//line sql.y:2
package sql

import __yyfmt__ "fmt"

//line sql.y:2
import (
	"fmt"
	"log"
	"strings"
)

/* expr defines an expression as a Lisp list.  If len(val)>0,
   it is an atomic expression, in particular, NUMBER, IDENT,
   or STRING, defined by typ and val; otherwise, it is a
   Lisp S-expression. */
type expr struct {
	typ  int
	val  string
	sexp []expr
}

/* construct an atomic expr */
func atomic(typ int, val string) expr {
	return expr{
		typ: typ,
		val: val,
	}
}

/* construct a funcall expr */
func funcall(name string, oprd []expr) expr {
	return expr{
		sexp: append([]expr{atomic(IDENT, name)}, oprd...),
	}
}

/* construct a unary expr */
func unary(typ int, op string, od1 expr) expr {
	return expr{
		sexp: append([]expr{atomic(typ, op)}, od1),
	}
}

/* construct a binary expr */
func binary(typ int, od1 expr, op string, od2 expr) expr {
	return expr{
		sexp: append([]expr{atomic(typ, op)}, od1, od2),
	}
}

/* construct a variadic expr */
func variadic(typ int, op string, ods []expr) expr {
	return expr{
		sexp: append([]expr{atomic(typ, op)}, ods...),
	}
}

type selectStmt struct {
	fields    []string
	tables    []string
	where     expr
	limit     string
	estimator string
	attrs     map[string]expr
	columns   []expr
	into      string
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

//line sql.y:80
type sqlSymType struct {
	yys  int
	val  string /* NUMBER, IDENT, STRING, and keywords */
	flds []string
	tbls []string
	expr expr
	expl []expr
	atrs map[string]expr
	slct selectStmt
}

const SELECT = 57346
const FROM = 57347
const WHERE = 57348
const LIMIT = 57349
const TRAIN = 57350
const WITH = 57351
const COLUMN = 57352
const INTO = 57353
const IDENT = 57354
const NUMBER = 57355
const STRING = 57356
const AND = 57357
const OR = 57358
const GE = 57359
const LE = 57360
const NOT = 57361
const POWER = 57362
const UMINUS = 57363

var sqlToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"SELECT",
	"FROM",
	"WHERE",
	"LIMIT",
	"TRAIN",
	"WITH",
	"COLUMN",
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

//line sql.y:194

/* Like Lisp's builtin function cdr. */
func (e expr) cdr() (r []string) {
	for i := 1; i < len(e.sexp); i++ {
		r = append(r, e.sexp[i].String())
	}
	return r
}

func (e expr) String() string {
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

//line yacctab:1
var sqlExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const sqlPrivate = 57344

const sqlLast = 150

var sqlAct = [...]int{

	58, 24, 33, 57, 52, 30, 79, 79, 18, 84,
	80, 61, 36, 59, 20, 19, 21, 39, 38, 40,
	41, 42, 43, 44, 53, 26, 54, 55, 35, 25,
	42, 43, 44, 60, 23, 17, 27, 56, 31, 63,
	34, 64, 65, 66, 67, 68, 69, 70, 71, 72,
	73, 74, 75, 62, 37, 28, 77, 16, 3, 29,
	32, 82, 22, 36, 83, 81, 50, 51, 47, 46,
	45, 49, 48, 40, 41, 42, 43, 44, 15, 12,
	85, 20, 19, 21, 78, 14, 1, 2, 0, 20,
	19, 21, 26, 0, 0, 0, 25, 13, 0, 0,
	26, 23, 76, 27, 25, 0, 0, 0, 0, 23,
	0, 27, 50, 51, 47, 46, 45, 49, 48, 40,
	41, 42, 43, 44, 5, 7, 6, 8, 9, 10,
	11, 47, 46, 45, 49, 48, 40, 41, 42, 43,
	44, 0, 0, 0, 0, 0, 0, 0, 0, 4,
}
var sqlPact = [...]int{

	54, -1000, 119, 73, -1000, 45, 22, 77, 43, 26,
	16, 42, -13, -1000, -1000, -14, -1000, -1000, 97, -1000,
	-28, -1000, -1000, 77, -1000, 77, 77, 2, -1000, -18,
	-1000, 14, -20, -1000, -1000, -28, -1000, -1000, 41, 27,
	77, 77, 77, 77, 77, 77, 77, 77, 77, 77,
	77, 77, 69, 51, -1000, -1000, -1000, -25, 97, 26,
	77, 16, -1000, -1000, 6, 6, -1000, -1000, -1000, -3,
	-3, -3, -3, -3, 114, 114, -1000, -24, -1000, 77,
	-1000, -1000, 97, -1000, -1000, 97,
}
var sqlPgo = [...]int{

	0, 87, 86, 79, 78, 0, 1, 2, 3, 62,
	60, 5, 59,
}
var sqlR1 = [...]int{

	0, 2, 1, 1, 1, 1, 1, 1, 1, 1,
	3, 3, 3, 7, 7, 7, 10, 10, 4, 4,
	11, 12, 12, 6, 6, 8, 8, 9, 9, 5,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5, 5, 5,
}
var sqlR2 = [...]int{

	0, 2, 2, 3, 3, 3, 3, 3, 3, 3,
	1, 1, 3, 1, 1, 1, 1, 3, 1, 3,
	3, 1, 3, 3, 4, 1, 3, 2, 3, 1,
	1, 1, 1, 3, 1, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 2, 2,
}
var sqlChk = [...]int{

	-1000, -2, -1, 4, 30, 5, 7, 6, 8, 9,
	10, 11, -3, 24, 12, -4, 12, 13, -5, 13,
	12, 14, -9, 32, -6, 27, 23, 34, 12, -12,
	-11, 12, -10, -7, 24, 12, -6, 12, 31, 31,
	22, 23, 24, 25, 26, 19, 18, 17, 21, 20,
	15, 16, 32, -5, -5, -5, 35, -8, -5, 31,
	19, 31, 12, 12, -5, -5, -5, -5, -5, -5,
	-5, -5, -5, -5, -5, -5, 33, -8, 33, 31,
	35, -11, -5, -7, 33, -5,
}
var sqlDef = [...]int{

	0, -2, 0, 0, 1, 0, 0, 0, 0, 0,
	0, 0, 2, 10, 11, 3, 18, 4, 5, 29,
	30, 31, 32, 0, 34, 0, 0, 0, 6, 7,
	21, 0, 8, 16, 13, 14, 15, 9, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 47, 48, 27, 0, 25, 0,
	0, 0, 12, 19, 35, 36, 37, 38, 39, 40,
	41, 42, 43, 44, 45, 46, 23, 0, 33, 0,
	28, 22, 20, 17, 24, 26,
}
var sqlTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 26, 3, 3,
	32, 33, 24, 22, 31, 23, 3, 25, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 30,
	18, 19, 17, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 34, 3, 35,
}
var sqlTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 20, 21, 27, 28, 29,
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
		//line sql.y:112
		{
			parseResult = sqlDollar[1].slct
		}
	case 2:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:115
		{
			sqlVAL.slct.fields = sqlDollar[2].flds
		}
	case 3:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:116
		{
			sqlVAL.slct.tables = sqlDollar[3].tbls
		}
	case 4:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:117
		{
			sqlVAL.slct.limit = sqlDollar[3].val
		}
	case 5:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:118
		{
			sqlVAL.slct.where = sqlDollar[3].expr
		}
	case 6:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:119
		{
			sqlVAL.slct.estimator = sqlDollar[3].val
		}
	case 7:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:120
		{
			sqlVAL.slct.attrs = sqlDollar[3].atrs
		}
	case 8:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:121
		{
			sqlVAL.slct.columns = sqlDollar[3].expl
		}
	case 9:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:122
		{
			sqlVAL.slct.into = sqlDollar[3].val
		}
	case 10:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:126
		{
			sqlVAL.flds = sqlVAL.flds[:0]
		}
	case 11:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:127
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[1].val)
		}
	case 12:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:128
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[3].val)
		}
	case 13:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:132
		{
			sqlVAL.expr = atomic(IDENT, "*")
		}
	case 14:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:133
		{
			sqlVAL.expr = atomic(IDENT, sqlDollar[1].val)
		}
	case 15:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:134
		{
			sqlVAL.expr = sqlDollar[1].expr
		}
	case 16:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:138
		{
			sqlVAL.expl = []expr{sqlDollar[1].expr}
		}
	case 17:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:139
		{
			sqlVAL.expl = append(sqlDollar[1].expl, sqlDollar[3].expr)
		}
	case 18:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:143
		{
			sqlVAL.tbls = []string{sqlDollar[1].val}
		}
	case 19:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:144
		{
			sqlVAL.tbls = append(sqlDollar[1].tbls, sqlDollar[3].val)
		}
	case 20:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:148
		{
			sqlVAL.atrs = map[string]expr{sqlDollar[1].val: sqlDollar[3].expr}
		}
	case 21:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:152
		{
			sqlVAL.atrs = sqlDollar[1].atrs
		}
	case 22:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:153
		{
			sqlVAL.atrs = attrsUnion(sqlDollar[1].atrs, sqlDollar[3].atrs)
		}
	case 23:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:157
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, nil)
		}
	case 24:
		sqlDollar = sqlS[sqlpt-4 : sqlpt+1]
		//line sql.y:158
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, sqlDollar[3].expl)
		}
	case 25:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:162
		{
			sqlVAL.expl = []expr{sqlDollar[1].expr}
		}
	case 26:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:163
		{
			sqlVAL.expl = append(sqlDollar[1].expl, sqlDollar[3].expr)
		}
	case 27:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:167
		{
			sqlVAL.expl = nil
		}
	case 28:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:168
		{
			sqlVAL.expl = sqlDollar[2].expl
		}
	case 29:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:172
		{
			sqlVAL.expr = atomic(NUMBER, sqlDollar[1].val)
		}
	case 30:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:173
		{
			sqlVAL.expr = atomic(IDENT, sqlDollar[1].val)
		}
	case 31:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:174
		{
			sqlVAL.expr = atomic(STRING, sqlDollar[1].val)
		}
	case 32:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:175
		{
			sqlVAL.expr = variadic('[', "square", sqlDollar[1].expl)
		}
	case 33:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:176
		{
			sqlVAL.expr = unary('(', "paren", sqlDollar[2].expr)
		}
	case 34:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:177
		{
			sqlVAL.expr = sqlDollar[1].expr
		}
	case 35:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:178
		{
			sqlVAL.expr = binary('+', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 36:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:179
		{
			sqlVAL.expr = binary('-', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 37:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:180
		{
			sqlVAL.expr = binary('*', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 38:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:181
		{
			sqlVAL.expr = binary('/', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 39:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:182
		{
			sqlVAL.expr = binary('%', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 40:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:183
		{
			sqlVAL.expr = binary('=', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 41:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:184
		{
			sqlVAL.expr = binary('<', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 42:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:185
		{
			sqlVAL.expr = binary('>', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 43:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:186
		{
			sqlVAL.expr = binary(LE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 44:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:187
		{
			sqlVAL.expr = binary(GE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 45:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:188
		{
			sqlVAL.expr = binary(AND, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 46:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:189
		{
			sqlVAL.expr = binary(OR, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 47:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:190
		{
			sqlVAL.expr = unary(NOT, sqlDollar[1].val, sqlDollar[2].expr)
		}
	case 48:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:191
		{
			sqlVAL.expr = unary('-', sqlDollar[1].val, sqlDollar[2].expr)
		}
	}
	goto sqlstack /* stack new state and value */
}
