package ast

// Source: src/ast/fpa_decl_plugin.h:96-108; src/util/mpf.cpp:23-59.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Source: src/ast/fpa_decl_plugin.cpp:52-62. No rollback is performed if
// reserve, SET, insertion, recycling, or NUM destruction throws.
func (p *astFloatingPointPluginOwner) mkValueID(value *unumeral.ArbitraryFloat) (idResult uint32, err error) {
	defer func() {
		if err != nil && p != nil && p.self == p && p.bodyCause == nil {
			p.bodyCause = err
		}
	}()
	if p == nil || p.self != p || p.owner == nil || p.owner.Self != p.owner || p.owner.floatingPointPlugin != p || !p.published || p.sourceEnded {
		return 0, ErrASTHandle
	}
	var id uint32
	if n := len(p.freeIDs); n != 0 {
		id = p.freeIDs[n-1]
		p.freeIDs = p.freeIDs[:n-1]
	} else {
		id = p.nextID
		if id == ^uint32(0) {
			return 0, ErrASTDomain
		}
		p.nextID++
	}
	if e := p.values.reserve(id + 1); e != nil {
		return 0, e
	}
	if e := p.values.set(id, value); e != nil {
		return 0, e
	}
	canonical, e := p.internValueID(id)
	if e != nil {
		return 0, e
	}
	if canonical != id {
		if !p.outOfMemory() {
			p.freeIDs = append(p.freeIDs, id)
		}
		if e = p.values.del(id); e != nil {
			return 0, e
		}
	}
	return canonical, nil
}
