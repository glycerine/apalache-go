package smtcore

// Source: src/smt/theory_array.cpp display, display_var, display_ids;
// src/smt/theory_array_full.cpp display_var; and
// src/smt/smt_context_pp.cpp display_theories.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *SMTInternalizerState) DisplayTheories(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	for _, theory := range s.theoryOrder {
		if printer, ok := theory.(interface{ Display(io.Writer) error }); ok {
			if err := printer.Display(out); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *SMTArrayTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if t.NumVariables() == 0 {
		return nil
	}
	if _, err := io.WriteString(out, "Theory array:\n"); err != nil {
		return err
	}
	for variable := range t.variables {
		if err := t.DisplayVariable(out, SMTTheoryVar(variable)); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTArrayTheory) displayArrayIDs(out io.Writer, nodes []*SMTEnode) error {
	for i, node := range nodes {
		if i != 0 {
			if _, err := io.WriteString(out, " "); err != nil {
				return err
			}
		}
		id, err := node.OwnerID()
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintf(out, "#%d", id); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTArrayTheory) DisplayVariable(out io.Writer, variable SMTTheoryVar) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	root, err := t.Enode(SMTTheoryVar(t.arrayUnion.Find(uint32(variable))))
	if err != nil {
		return err
	}
	id, err := node.OwnerID()
	if err != nil {
		return err
	}
	rootID, err := root.OwnerID()
	if err != nil {
		return err
	}
	sort, err := s.manager.GetSort(node.owner)
	if err != nil {
		return err
	}
	family, err := s.manager.GetFamilyID(sort)
	if err != nil {
		return err
	}
	op, arrayOperation, err := t.arrayOperation(node.owner)
	if err != nil {
		return err
	}
	var isArray, isSelect, upward int
	if family == t.FamilyID() {
		isArray = 1
	}
	if arrayOperation && op == ast.ASTArrayOpSelect {
		isSelect = 1
	}
	data := t.arrayData[variable]
	if data.propUpward {
		upward = 1
	}
	if _, err = fmt.Fprintf(out, "v%-4d #%-4d -> #%-4d is_array: %d is_select: %d upward: %d stores: {", variable, id, rootID, isArray, isSelect, upward); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.stores); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "} p_stores: {"); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.parentStores); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "} p_selects: {"); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.parentSelects); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "}\n"); err != nil || t.simple {
		return err
	}
	if _, err = io.WriteString(out, " maps: {"); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.maps); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "} p_parent_maps: {"); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.parentMaps); err != nil {
		return err
	}
	if _, err = io.WriteString(out, "} p_const: {"); err != nil {
		return err
	}
	if err = t.displayArrayIDs(out, data.consts); err != nil {
		return err
	}
	_, err = io.WriteString(out, "}\n")
	return err
}
