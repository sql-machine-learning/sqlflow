package sql

import (
	"log"
	"strconv"
)

type parser struct {
	l   chan item
	sel *selectStmt
}

type parseState func(p *parser) parseState

type selectStmt struct {
	fields    []string // empty denotes SELECT *
	from      []string
	where     *expr
	limit     int
	estimator string
	column    *expr
}

type expr struct {
	op       itemType // itemPlus, itemMinus, itemAnd, itemOr, ...
	operands *expr
}

func newParser(l chan item) *parser {
	return &parser{l: l, sel: &selectStmt{}}
}

func (p *parser) parse() {
	for state := parseSelect; state != nil; {
		state = state(p)
	}
}

func parseSelect(p *parser) parseState {
	expectItemType(itemSelect, <-p.l)

	n := <-p.l
	switch n.typ {
	case itemTimes:
		n = <-p.l
		return selectClause(n)
	case itemIdent:
		for n.typ == itemIdent {
			p.sel.fields = append(p.sel.fields, n.val)
			n = <-p.l
			if n.typ == itemComma {
				n = <-p.l
			}
		}
		return selectClause(n)
	default:
		log.Panicf("Unexpected token %q", n)
	}
	return nil // stop parsing.
}

func selectClause(n item) parseState {
	switch n.typ {
	case itemFrom:
		return parseFrom
	case itemWhere:
		return parseWhere
	case itemLimit:
		return parseLimit
	case itemTrain:
		return parseTrain
	case itemColumn:
		return parseColumn
	case itemSemiColon:
		return nil
	default:
		log.Panicf("Unknown clause in select statement: %q", n)
	}
	return nil
}

func parseFrom(p *parser) parseState {
	for {
		n := <-p.l
		expectItemType(itemIdent, n)
		p.sel.from = append(p.sel.from, n.val)
		n = <-p.l
		if n.typ != itemComma {
			return selectClause(n)
		}
	}
}

func parseWhere(p *parser) parseState {
	log.Panicf("parseWhere to be implemented")
	return nil
}

func parseLimit(p *parser) parseState {
	n := <-p.l
	expectItemType(itemNumber, n)
	if limit, e := strconv.Atoi(n.val); e != nil {
		log.Panicf("parseLimit: Cannot convert limit (%s) into int: %s", n.val, e)
	} else {
		p.sel.limit = limit
	}

	n = <-p.l
	return selectClause(n)
}

func parseTrain(p *parser) parseState {
	n := <-p.l
	expectItemType(itemIdent, n)
	p.sel.estimator = n.val

	n = <-p.l
	return selectClause(n)
}

func parseColumn(p *parser) parseState {
	log.Panicf("parseColumn to be implemented")
	return nil
}

func expectItemType(expect itemType, real item) {
	if expect != real.typ {
		log.Panicf("Expecting itemType %q, got %q", expect, real)
	}
}
