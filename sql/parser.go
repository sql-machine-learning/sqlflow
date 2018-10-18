//line sql.y:2
package sql

import __yyfmt__ "fmt"

//line sql.y:2
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

//line sql.y:78
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
const IDENT = 57353
const NUMBER = 57354
const STRING = 57355
const AND = 57356
const OR = 57357
const GE = 57358
const LE = 57359
const POWER = 57360
const NOT = 57361
const UMINUS = 57362

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
	"POWER",
	"'+'",
	"'-'",
	"'*'",
	"'/'",
	"'%'",
	"NOT",
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

//line sql.y:178

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
				if i < len(e.sexp)-1 {
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
				if i < len(e.sexp)-1 {
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

//line yacctab:1
var sqlExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const sqlPrivate = 57344

const sqlLast = 120

var sqlAct = [...]int{

	50, 49, 28, 70, 70, 51, 74, 71, 16, 42,
	43, 39, 38, 37, 41, 40, 44, 32, 33, 34,
	35, 36, 45, 31, 46, 47, 30, 69, 5, 7,
	6, 8, 9, 55, 56, 57, 58, 59, 60, 61,
	62, 63, 64, 65, 66, 52, 68, 18, 17, 19,
	15, 3, 4, 73, 72, 29, 18, 17, 19, 24,
	54, 53, 26, 23, 34, 35, 36, 21, 24, 25,
	48, 75, 23, 18, 17, 19, 21, 67, 25, 32,
	33, 34, 35, 36, 14, 24, 27, 20, 22, 23,
	12, 13, 10, 21, 1, 25, 42, 43, 39, 38,
	37, 41, 40, 11, 32, 33, 34, 35, 36, 39,
	38, 37, 41, 40, 2, 32, 33, 34, 35, 36,
}
var sqlPact = [...]int{

	47, -1000, 23, 79, -1000, 73, 38, 62, 51, 44,
	-4, -1000, -1000, -7, -1000, -1000, 82, -1000, -15, -1000,
	-1000, 62, -1000, 62, 62, 36, -1000, -25, -1000, 27,
	50, 49, 62, 62, 62, 62, 62, 62, 62, 62,
	62, 62, 62, 62, 45, -5, -1000, -1000, -1000, -27,
	82, 44, 62, -1000, -1000, 40, 40, -1000, -1000, -1000,
	57, 57, 57, 57, 57, 93, 93, -1000, -26, -1000,
	62, -1000, -1000, 82, -1000, 82,
}
var sqlPgo = [...]int{

	0, 114, 94, 92, 91, 0, 88, 1, 87, 2,
	86,
}
var sqlR1 = [...]int{

	0, 2, 1, 1, 1, 1, 1, 1, 3, 3,
	3, 4, 4, 9, 10, 10, 6, 6, 7, 7,
	8, 8, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5,
}
var sqlR2 = [...]int{

	0, 2, 2, 3, 3, 3, 3, 3, 1, 1,
	3, 1, 3, 3, 1, 3, 3, 4, 1, 3,
	2, 3, 1, 1, 1, 1, 3, 1, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	2, 2,
}
var sqlChk = [...]int{

	-1000, -2, -1, 4, 29, 5, 7, 6, 8, 9,
	-3, 24, 11, -4, 11, 12, -5, 12, 11, 13,
	-8, 31, -6, 27, 23, 33, 11, -10, -9, 11,
	30, 30, 22, 23, 24, 25, 26, 18, 17, 16,
	20, 19, 14, 15, 31, -5, -5, -5, 34, -7,
	-5, 30, 18, 11, 11, -5, -5, -5, -5, -5,
	-5, -5, -5, -5, -5, -5, -5, 32, -7, 32,
	30, 34, -9, -5, 32, -5,
}
var sqlDef = [...]int{

	0, -2, 0, 0, 1, 0, 0, 0, 0, 0,
	2, 8, 9, 3, 11, 4, 5, 22, 23, 24,
	25, 0, 27, 0, 0, 0, 6, 7, 14, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 40, 41, 20, 0,
	18, 0, 0, 10, 12, 28, 29, 30, 31, 32,
	33, 34, 35, 36, 37, 38, 39, 16, 0, 26,
	0, 21, 15, 13, 17, 19,
}
var sqlTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 26, 3, 3,
	31, 32, 24, 22, 30, 23, 3, 25, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 29,
	17, 18, 16, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 33, 3, 34,
}
var sqlTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 19, 20, 21, 27, 28,
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
		//line sql.y:109
		{
			parseResult = sqlDollar[1].slct
		}
	case 2:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:112
		{
			sqlVAL.slct.fields = sqlDollar[2].flds
		}
	case 3:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:113
		{
			sqlVAL.slct.tables = sqlDollar[3].tbls
		}
	case 4:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:114
		{
			sqlVAL.slct.limit = sqlDollar[3].val
		}
	case 5:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:115
		{
			sqlVAL.slct.where = sqlDollar[3].expr
		}
	case 6:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:116
		{
			sqlVAL.slct.estimator = sqlDollar[3].val
		}
	case 7:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:117
		{
			sqlVAL.slct.attrs = sqlDollar[3].atrs
		}
	case 8:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:121
		{
			sqlVAL.flds = sqlVAL.flds[:0]
		}
	case 9:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:122
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[1].val)
		}
	case 10:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:123
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[3].val)
		}
	case 11:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:127
		{
			sqlVAL.tbls = []string{sqlDollar[1].val}
		}
	case 12:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:128
		{
			sqlVAL.tbls = append(sqlDollar[1].tbls, sqlDollar[3].val)
		}
	case 13:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:132
		{
			sqlVAL.atrs = map[string]expr{sqlDollar[1].val: sqlDollar[3].expr}
		}
	case 14:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:136
		{
			sqlVAL.atrs = sqlDollar[1].atrs
		}
	case 15:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:137
		{
			sqlVAL.atrs = attrsUnion(sqlDollar[1].atrs, sqlDollar[3].atrs)
		}
	case 16:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:141
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, nil)
		}
	case 17:
		sqlDollar = sqlS[sqlpt-4 : sqlpt+1]
		//line sql.y:142
		{
			sqlVAL.expr = funcall(sqlDollar[1].val, sqlDollar[3].expl)
		}
	case 18:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:146
		{
			sqlVAL.expl = []expr{sqlDollar[1].expr}
		}
	case 19:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:147
		{
			sqlVAL.expl = append(sqlDollar[1].expl, sqlDollar[3].expr)
		}
	case 20:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:151
		{
			sqlVAL.expl = nil
		}
	case 21:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:152
		{
			sqlVAL.expl = sqlDollar[2].expl
		}
	case 22:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:156
		{
			sqlVAL.expr = atomic(NUMBER, sqlDollar[1].val)
		}
	case 23:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:157
		{
			sqlVAL.expr = atomic(IDENT, sqlDollar[1].val)
		}
	case 24:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:158
		{
			sqlVAL.expr = atomic(STRING, sqlDollar[1].val)
		}
	case 25:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:159
		{
			sqlVAL.expr = variadic('[', "square", sqlDollar[1].expl)
		}
	case 26:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:160
		{
			sqlVAL.expr = unary('(', "paren", sqlDollar[2].expr)
		}
	case 27:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:161
		{
			sqlVAL.expr = sqlDollar[1].expr
		}
	case 28:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:162
		{
			sqlVAL.expr = binary('+', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 29:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:163
		{
			sqlVAL.expr = binary('-', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 30:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:164
		{
			sqlVAL.expr = binary('*', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 31:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:165
		{
			sqlVAL.expr = binary('/', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 32:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:166
		{
			sqlVAL.expr = binary('%', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 33:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:167
		{
			sqlVAL.expr = binary('=', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 34:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:168
		{
			sqlVAL.expr = binary('<', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 35:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:169
		{
			sqlVAL.expr = binary('>', sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 36:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:170
		{
			sqlVAL.expr = binary(LE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 37:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:171
		{
			sqlVAL.expr = binary(GE, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 38:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:172
		{
			sqlVAL.expr = binary(AND, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 39:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:173
		{
			sqlVAL.expr = binary(OR, sqlDollar[1].expr, sqlDollar[2].val, sqlDollar[3].expr)
		}
	case 40:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:174
		{
			sqlVAL.expr = unary(NOT, sqlDollar[1].val, sqlDollar[2].expr)
		}
	case 41:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:175
		{
			sqlVAL.expr = unary('-', sqlDollar[1].val, sqlDollar[2].expr)
		}
	}
	goto sqlstack /* stack new state and value */
}
