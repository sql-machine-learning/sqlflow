package sql

import (
	"log"
)

type parser struct {
	l   chan item
	sel *selectStmt
}

type parseState func(p *parser, it *item) (parseState, *item)

type selectStmt struct {
	fields []string // empty denotes SELECT *
	from   []string
	where  *expr
}

type expr struct {
	op       itemType // itemPlus, itemMinus, itemAnd, itemOr, ...
	operands *expr
}

func newParser(l chan item) *parser {
	return &parser{l: l, sel: &selectStmt{}}
}

func (p *parser) parse() {
	var it *item
	for state := parseSelect; state != nil; {
		state, it = state(p, it)
	}
}

func parseSelect(p *parser, it *item) (parseState, *item) {
	expectItemType(itemSelect, <-p.l)

	n := <-p.l
	switch n.typ {
	case itemTimes:
		n = <-p.l
		return parseClause(n), &n
	case itemIdent:
		for n.typ == itemIdent {
			p.sel.fields = append(p.sel.fields, n.val)
			n = <-p.l
			if n.typ == itemComma {
				n = <-p.l
			}
		}
		return parseClause(n), &n
	default:
		log.Panicf("Unexpected token %q", n)
	}
	return nil, nil
}

func parseClause(n item) parseState {
	switch n.typ {
	case itemFrom:
		return parseFrom
	case itemWhere:
		return parseWhere
	case itemSemiColon:
		return nil
	default:
		log.Panicf("Unknown clause in select statement: %q", n)
	}
	return nil
}

func parseFrom(p *parser, it *item) (parseState, *item) {
	expectItemType(itemFrom, *it)

	for {
		n := <-p.l
		expectItemType(itemIdent, n)
		p.sel.from = append(p.sel.from, n.val)
		n = <-p.l
		if n.typ != itemComma {
			return parseClause(n), &n
		}
	}
}

func parseWhere(p *parser, it *item) (parseState, *item) {
	return nil, nil
}

func expectItemType(expect itemType, real item) {
	if expect != real.typ {
		log.Panicf("Expecting itemType %q, got %q", expect, real)
	}
}
