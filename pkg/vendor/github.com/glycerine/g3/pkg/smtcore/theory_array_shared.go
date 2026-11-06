package smtcore

// Source: theory_array_base::is_shared, collect_shared_vars, mk_interface_eqs,
// context::is_shared, and theory_array::final_check_eh interface scheduling.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Source theory_array::init_search_eh starts the final-check cadence anew
// for each check-sat, independently of user scopes and cumulative statistics.
func (t *SMTArrayTheory) InitializeSearch() error {
	t.finalCheckIndex = 0
	return nil
}

func (t *SMTArrayTheory) sharedArrayNode(node *SMTEnode) (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	shared, consult, err := s.sharedTheoryNode(node)
	if err != nil || !consult {
		return shared, err
	}
	root := node.root
	// Arrays implement multiple sort instances in the same family. A root
	// used in two roles is shared even if every parent has the array family.
	var arrayRole, indexRole, valueRole bool
	for _, parent := range root.parents {
		op, array, err := t.arrayOperation(parent.owner)
		if err != nil {
			return false, err
		}
		if !array {
			continue
		}
		args := parent.arguments
		switch op {
		case ast.ASTArrayOpStore, ast.ASTArrayOpSelect:
			if len(args) == 0 {
				continue
			}
			arrayRole = arrayRole || args[0].root == root
			end := len(args)
			if op == ast.ASTArrayOpStore {
				end--
				valueRole = valueRole || args[end].root == root
			}
			for _, arg := range args[1:end] {
				indexRole = indexRole || arg.root == root
			}
		case ast.ASTArrayOpConst:
			if len(args) != 0 {
				valueRole = valueRole || args[0].root == root
			}
		}
		roles := 0
		for _, role := range []bool{arrayRole, indexRole, valueRole} {
			if role {
				roles++
			}
		}
		if roles > 1 {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTArrayTheory) makeInterfaceEqualities() (bool, error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	if err = t.emitArrayTrace("array_shared", "collect_shared_vars", 422, nil, func(out io.Writer) error {
		_, err := io.WriteString(out, "collecting shared vars...\n")
		return err
	}); err != nil {
		return false, err
	}
	seen := make(map[*SMTEnode]bool)
	var roots []*SMTEnode
	for _, node := range t.variables {
		if node == nil || s.relevancy != nil && !s.relevancy.IsRelevant(node.owner) {
			continue
		}
		root := node.root
		if seen[root] {
			continue
		}
		seen[root] = true
		sort, err := s.manager.GetSort(root.owner)
		if err != nil {
			return false, err
		}
		family, err := s.manager.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		if family != t.FamilyID() {
			continue
		}
		shared, err := t.sharedArrayNode(root)
		if err != nil {
			return false, err
		}
		if shared {
			if err = t.emitArrayTrace("array_shared", "collect_shared_vars", 432, nil, func(out io.Writer) error {
				id, err := root.OwnerID()
				if err != nil {
					return err
				}
				_, err = fmt.Fprintf(out, "new shared var: #%d\n", id)
				return err
			}); err != nil {
				return false, err
			}
			roots = append(roots, root)
		}
	}
	progress := false
	for i, left := range roots {
		if err = t.emitArrayTrace("array_bug", "mk_interface_eqs", 459, nil, func(out io.Writer) error {
			variable := left.TheoryVar(t.FamilyID())
			_, err := fmt.Fprintf(out, "mk_interface_eqs: processing: v%d\n", variable)
			return err
		}); err != nil {
			return false, err
		}
		leftSort, err := s.manager.GetSort(left.owner)
		if err != nil {
			return false, err
		}
		for _, right := range roots[i+1:] {
			rightSort, err := s.manager.GetSort(right.owner)
			if err != nil {
				return false, err
			}
			if leftSort != rightSort {
				continue
			}
			disequal, err := t.hasRelevantDisequality(left, right)
			if err != nil {
				return false, err
			}
			if disequal {
				continue
			}
			expression, err := s.manager.MkEq(left.owner, right.owner)
			if err != nil {
				return false, err
			}
			_, internalized := s.BoolVar(expression)
			relevant := s.relevancy == nil || s.relevancy.IsRelevant(expression)
			if internalized && relevant {
				continue
			}
			if err := s.Internalize(expression, true); err != nil {
				return false, err
			}
			if err := s.markRelevant(expression); err != nil {
				return false, err
			}
			t.statistics.eqSplits++
			s.theoryProgress++
			progress = true
		}
	}
	return progress, nil
}
