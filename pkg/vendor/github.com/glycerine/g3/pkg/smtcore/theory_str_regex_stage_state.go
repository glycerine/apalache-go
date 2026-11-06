package smtcore

// Source: src/smt/theory_str.h regex caches and retained positive unbounded
// regex_automaton_under_assumptions instantiations; theory_str.cpp persistence.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Every retained cache producer constructs (regex, automaton, true) without
// bounds. The bound setters/unsetters have no retained callers. Represent
// that actual specialization instead of adding a general assumption adapter.
type smtStringRegexStageState struct {
	automata         map[ast.ASTHandle]*rewriter.StringRegexAutomaton
	owned            []*rewriter.StringRegexAutomaton
	products         []*rewriter.StringRegexAutomaton
	lengthDone       map[ast.ASTHandle]uint32
	pathDone         map[ast.ASTHandle]uint32
	lengthFormulas   map[ast.ASTHandle]ast.ASTHandle
	lengthVariables  map[ast.ASTHandle][]ast.ASTHandle
	failures         map[ast.ASTHandle]uint32
	lengthAttempts   map[ast.ASTHandle]uint32
	intersectionFail map[ast.ASTHandle]uint32
	lastUpper        map[ast.ASTHandle]*big.Int
	persisted        []ast.ASTRef
	persistedTodo    []ast.ASTRef
}

func newStringRegexStageState() smtStringRegexStageState {
	return smtStringRegexStageState{
		automata:   make(map[ast.ASTHandle]*rewriter.StringRegexAutomaton),
		lengthDone: make(map[ast.ASTHandle]uint32), pathDone: make(map[ast.ASTHandle]uint32),
		lengthFormulas: make(map[ast.ASTHandle]ast.ASTHandle), lengthVariables: make(map[ast.ASTHandle][]ast.ASTHandle),
		failures: make(map[ast.ASTHandle]uint32), lengthAttempts: make(map[ast.ASTHandle]uint32),
		intersectionFail: make(map[ast.ASTHandle]uint32), lastUpper: make(map[ast.ASTHandle]*big.Int),
	}
}

func (t *SMTStringTheory) addPersistedStringAxiom(expression ast.ASTHandle) error {
	root, err := t.state.manager.NewRef(expression)
	if err != nil {
		return err
	}
	t.regexStage.persisted = append(t.regexStage.persisted, root)
	return nil
}

func (t *SMTStringTheory) popStringRegexStage() error {
	for _, root := range t.regexStage.persisted {
		expression, err := t.state.manager.RefGet(root)
		if err != nil {
			return err
		}
		replayRoot, err := t.state.manager.NewRef(expression)
		if err != nil {
			return err
		}
		t.regexStage.persistedTodo = append(t.regexStage.persistedTodo, replayRoot)
	}
	for term, level := range t.regexStage.lengthDone {
		if level > t.level {
			delete(t.regexStage.lengthDone, term)
		}
	}
	for term, level := range t.regexStage.pathDone {
		if level > t.level {
			delete(t.regexStage.pathDone, term)
		}
	}
	return nil
}

func (t *SMTStringTheory) propagatePersistedStringAxioms() error {
	for _, root := range t.regexStage.persistedTodo {
		expression, err := t.state.manager.RefGet(root)
		if err != nil {
			return err
		}
		if err := t.assertAxiom(expression); err != nil {
			return err
		}
	}
	for i := range t.regexStage.persistedTodo {
		if err := t.state.manager.DelRef(&t.regexStage.persistedTodo[i]); err != nil {
			return err
		}
	}
	t.regexStage.persistedTodo = nil
	return nil
}

func (s *smtStringRegexStageState) closeAutomata() (result error) {
	for _, owner := range s.owned {
		result = errors.Join(result, owner.Close())
	}
	s.owned, s.automata = nil, nil
	return result
}

func (s *smtStringRegexStageState) closeReferences(m *ast.ASTManager) (result error) {
	// Native later-declared m_persisted_axiom_todo closes before its source
	// vector; each expr_ref_vector releases its elements forward.
	for i := range s.persistedTodo {
		result = errors.Join(result, m.DelRef(&s.persistedTodo[i]))
	}
	for i := range s.persisted {
		result = errors.Join(result, m.DelRef(&s.persisted[i]))
	}
	s.persistedTodo, s.persisted = nil, nil
	return result
}

func (s *smtStringRegexStageState) closeProducts() (result error) {
	// The later-declared scoped_ptr_vector<eautomaton> closes after conversion
	// terms but before contains_map, separately from regex_automata's body loop.
	for _, owner := range s.products {
		result = errors.Join(result, owner.Close())
	}
	s.products = nil
	return result
}
