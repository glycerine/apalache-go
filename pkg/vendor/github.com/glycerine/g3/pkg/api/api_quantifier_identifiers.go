package api

// Source: src/ast/ast.h quantifier qid/skid accessors, consumed by the
// retained quantifier construction and inspection boundary.

import "github.com/glycerine/g3/pkg/ast"

// QuantifierIdentifiers returns the optional quantifier and skolem identifiers.
// Empty source symbols are returned as empty strings.
func (c *SMTContext) QuantifierIdentifiers(value ast.ASTHandle) (qid, skolemID string, err error) {
	manager, err := c.Manager()
	if err != nil {
		return "", "", err
	}
	q, s, err := manager.GetQuantifierIDs(value)
	if err != nil {
		return "", "", err
	}
	if !q.IsNull() {
		qid, err = manager.Symbols.String(q)
		if err != nil {
			return "", "", err
		}
	}
	if !s.IsNull() {
		skolemID, err = manager.Symbols.String(s)
		if err != nil {
			return "", "", err
		}
	}
	return qid, skolemID, nil
}
