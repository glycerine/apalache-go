package smtcore

// Source: src/smt/smt_model_finder.cpp:2738-2813,2848-2864.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// hintSearchState is the scoped satisfied/residue portion of hint_solver.
// Quantifiers and function declarations are borrowed from its AST manager.
// The surrounding macro indexes and greedy search remain in the model finder.
type hintSearchState struct {
	residue, satisfied    *scopedQuantifierSet
	definitions           map[ast.ASTHandle]ast.ASTHandle
	infos                 map[ast.ASTHandle]*hintQuantifierInfo
	forbidden, candidates map[ast.ASTHandle]struct{}
	qByFunction           map[ast.ASTHandle]map[ast.ASTHandle]struct{}
	qByDefinition         map[hintFunctionDefinition]map[ast.ASTHandle]struct{}
	defsByFunction        map[ast.ASTHandle]map[ast.ASTHandle]struct{}
	trace                 *diagnostic.TraceTags
}

func newHintSearchState() *hintSearchState {
	s := &hintSearchState{
		residue:        newScopedQuantifierSet(nil),
		definitions:    make(map[ast.ASTHandle]ast.ASTHandle),
		infos:          make(map[ast.ASTHandle]*hintQuantifierInfo),
		forbidden:      make(map[ast.ASTHandle]struct{}),
		candidates:     make(map[ast.ASTHandle]struct{}),
		qByFunction:    make(map[ast.ASTHandle]map[ast.ASTHandle]struct{}),
		qByDefinition:  make(map[hintFunctionDefinition]map[ast.ASTHandle]struct{}),
		defsByFunction: make(map[ast.ASTHandle]map[ast.ASTHandle]struct{}),
	}
	s.satisfied = newScopedQuantifierSet(func(q ast.ASTHandle, _ bool) {
		// hint_solver::ev_handler runs before each satisfying-set trail edit.
		if info := s.infos[q]; info != nil {
			info.theOne = ast.ASTHandle{}
		}
	})
	return s
}

func (s *hintSearchState) pushScope() {
	s.satisfied.pushScope()
	s.residue.pushScope()
}

func (s *hintSearchState) popScope() error {
	// Source greedy pops m_satisfied before m_residue. The first pop clears
	// the chosen function for each undone satisfied-set insertion.
	if e := s.satisfied.popScope(); e != nil {
		return e
	}
	return s.residue.popScope()
}

// updateSatisfiedResidue follows hint_solver::update_satisfied_residue using
// its already-built Q_f and Q_{f=def} indexes. The caller holds the greedy
// search scope, as the source does.
func (s *hintSearchState) updateSatisfiedResidue(f, def ast.ASTHandle) (bool, error) {
	if s == nil || s.residue == nil || s.satisfied == nil || len(s.satisfied.scopes) == 0 || len(s.residue.scopes) == 0 {
		return false, ast.ErrASTDomain
	}
	qF, qFDef := s.qByFunction[f], s.qByDefinition[hintFunctionDefinition{f, def}]
	if qF == nil || qFDef == nil {
		return false, ast.ErrASTDomain
	}
	useful := false
	for q := range qFDef {
		if s.satisfied.contains(q) {
			continue
		}
		info, e := s.info(q)
		if e != nil {
			return false, e
		}
		useful = true
		s.residue.erase(q)
		s.satisfied.insert(q)
		// This is quantifier_info::set_the_one(f); the event handler above
		// clears it when the insertion is undone.
		info.theOne = f
	}
	if !useful {
		return false, nil
	}
	for q := range qF {
		if !s.satisfied.contains(q) {
			s.residue.insert(q)
		}
	}
	return true, nil
}
