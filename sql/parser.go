//line sql.y:2
package sql

import __yyfmt__ "fmt"

//line sql.y:3
import "fmt"

type expr struct {
	typ  int    /* NUMBER, IDENT, STRING, or operator */
	oprd []expr /* if typ is an operator */
	val  string /* if typ is not an operator */
}

type selectStmt struct {
	fields []string
	tables []string
	where  expr
	limit  string
}

//line sql.y:22
type sqlSymType struct {
	yys  int
	val  string /* NUMBER, IDENT, STRING, and keywords */
	flds []string
	tbls []string
	expr expr
	slct selectStmt
}

const SELECT = 57346
const FROM = 57347
const WHERE = 57348
const LIMIT = 57349
const TRAIN = 57350
const COLUMN = 57351
const IDENT = 57352
const NUMBER = 57353
const GE = 57354
const LE = 57355
const POWER = 57356
const UMINUS = 57357

var sqlToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"SELECT",
	"FROM",
	"WHERE",
	"LIMIT",
	"TRAIN",
	"COLUMN",
	"IDENT",
	"NUMBER",
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
	"UMINUS",
	"';'",
	"','",
	"'('",
	"')'",
}
var sqlStatenames = [...]string{}

const sqlEofCode = 1
const sqlErrCode = 2
const sqlInitialStackSize = 16

//line sql.y:82

//line yacctab:1
var sqlExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const sqlPrivate = 57344

const sqlLast = 65

var sqlAct = [...]int{

	14, 28, 27, 26, 30, 29, 20, 21, 22, 23,
	24, 25, 19, 23, 24, 25, 45, 13, 31, 32,
	34, 33, 35, 36, 37, 38, 39, 40, 41, 42,
	43, 44, 28, 27, 26, 30, 29, 12, 21, 22,
	23, 24, 25, 16, 15, 5, 7, 6, 10, 3,
	11, 8, 18, 21, 22, 23, 24, 25, 9, 17,
	1, 2, 0, 0, 4,
}
var sqlPact = [...]int{

	45, -1000, 40, 38, -1000, 27, 6, 33, -13, -1000,
	-1000, -19, -1000, -1000, 20, -1000, -1000, 33, 33, 11,
	10, 33, 33, 33, 33, 33, 33, 33, 33, 33,
	33, -11, -1000, -1000, -1000, -7, -7, -1000, -1000, -1000,
	35, 35, 35, 35, 35, -1000,
}
var sqlPgo = [...]int{

	0, 61, 60, 51, 50, 0,
}
var sqlR1 = [...]int{

	0, 2, 1, 1, 1, 1, 3, 3, 3, 4,
	4, 5, 5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5,
}
var sqlR2 = [...]int{

	0, 2, 2, 3, 3, 3, 1, 1, 3, 1,
	3, 1, 1, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 2,
}
var sqlChk = [...]int{

	-1000, -2, -1, 4, 24, 5, 7, 6, -3, 20,
	10, -4, 10, 11, -5, 11, 10, 26, 19, 25,
	25, 18, 19, 20, 21, 22, 14, 13, 12, 16,
	15, -5, -5, 10, 10, -5, -5, -5, -5, -5,
	-5, -5, -5, -5, -5, 27,
}
var sqlDef = [...]int{

	0, -2, 0, 0, 1, 0, 0, 0, 2, 6,
	7, 3, 9, 4, 5, 11, 12, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 24, 8, 10, 14, 15, 16, 17, 18,
	19, 20, 21, 22, 23, 13,
}
var sqlTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 22, 3, 3,
	26, 27, 20, 18, 25, 19, 3, 21, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 24,
	13, 14, 12,
}
var sqlTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	15, 16, 17, 23,
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
		//line sql.y:46
		{
			fmt.Printf("%qq\n", sqlDollar[1].slct)
		}
	case 2:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:49
		{
			sqlVAL.slct.fields = sqlDollar[2].flds
		}
	case 3:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:50
		{
			sqlVAL.slct.tables = sqlDollar[3].tbls
		}
	case 4:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:51
		{
			sqlVAL.slct.limit = sqlDollar[3].val
		}
	case 5:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:52
		{
			sqlVAL.slct.where = sqlDollar[3].expr
		}
	case 6:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:56
		{
			sqlVAL.flds = sqlVAL.flds[:0]
		}
	case 7:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:57
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[1].val)
		}
	case 8:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:58
		{
			sqlVAL.flds = append(sqlVAL.flds, sqlDollar[3].val)
		}
	case 9:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:62
		{
			sqlVAL.tbls = append(sqlVAL.tbls, sqlDollar[1].val)
		}
	case 10:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:63
		{
			sqlVAL.tbls = append(sqlVAL.tbls, sqlDollar[3].val)
		}
	case 11:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:67
		{
			sqlVAL.expr = expr{typ: NUMBER, val: sqlDollar[1].val}
		}
	case 12:
		sqlDollar = sqlS[sqlpt-1 : sqlpt+1]
		//line sql.y:68
		{
			sqlVAL.expr = expr{typ: IDENT, val: sqlDollar[1].val}
		}
	case 13:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:69
		{
			sqlVAL.expr = sqlDollar[2].expr
		}
	case 14:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:70
		{
			sqlVAL.expr = expr{typ: '+', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 15:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:71
		{
			sqlVAL.expr = expr{typ: '-', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 16:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:72
		{
			sqlVAL.expr = expr{typ: '*', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 17:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:73
		{
			sqlVAL.expr = expr{typ: '/', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 18:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:74
		{
			sqlVAL.expr = expr{typ: '%', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 19:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:75
		{
			sqlVAL.expr = expr{typ: '=', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 20:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:76
		{
			sqlVAL.expr = expr{typ: '<', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 21:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:77
		{
			sqlVAL.expr = expr{typ: '>', oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 22:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:78
		{
			sqlVAL.expr = expr{typ: LE, oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 23:
		sqlDollar = sqlS[sqlpt-3 : sqlpt+1]
		//line sql.y:79
		{
			sqlVAL.expr = expr{typ: GE, oprd: []expr{sqlDollar[1].expr, sqlDollar[3].expr}}
		}
	case 24:
		sqlDollar = sqlS[sqlpt-2 : sqlpt+1]
		//line sql.y:80
		{
			sqlVAL.expr = expr{typ: '-', oprd: []expr{sqlDollar[2].expr}}
		}
	}
	goto sqlstack /* stack new state and value */
}
