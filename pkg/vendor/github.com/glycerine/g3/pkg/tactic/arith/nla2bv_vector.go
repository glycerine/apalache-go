package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:60-62,132,259-262,277-284;
// src/util/ref_vector.h:39-59,90-94; src/util/vector.h:59-99.

import "github.com/glycerine/g3/pkg/ast"

func (c *nla2BVInvocation) appendOwned(dst *[]ast.ASTHandle, charged *uint64, h ast.ASTHandle) error {
	if c.local.err != nil {
		return c.local.err
	}
	return c.real.appendVectorRef(dst, charged, h)
}
