package smtcore

// Source: src/smt/theory_str.cpp cut_vars_map_copy, has_self_cut,
// add_cut_info_one_node/merge, check_and_init_cut_var and pop_scope_eh.

import "github.com/glycerine/g3/pkg/ast"

type smtStringCut struct {
	level int64
	vars  map[ast.ASTHandle]struct{}
}

func (t *SMTStringTheory) hasSelfCut(first, second ast.ASTHandle) bool {
	a, b := t.cutHistory[first], t.cutHistory[second]
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	for variable := range a[len(a)-1].vars {
		if _, found := b[len(b)-1].vars[variable]; found {
			return true
		}
	}
	return false
}

func (t *SMTStringTheory) addStringCut(base ast.ASTHandle, level int64, variables map[ast.ASTHandle]struct{}) error {
	history := t.cutHistory[base]
	if len(history) != 0 && history[len(history)-1].level > level {
		return ast.ErrASTDomain
	}
	if len(history) == 0 || history[len(history)-1].level < level {
		frame := smtStringCut{level: level, vars: make(map[ast.ASTHandle]struct{})}
		if len(history) != 0 {
			for variable := range history[len(history)-1].vars {
				frame.vars[variable] = struct{}{}
			}
		}
		history = append(history, frame)
	}
	for variable := range variables {
		history[len(history)-1].vars[variable] = struct{}{}
	}
	t.cutHistory[base] = history
	return nil
}

func (t *SMTStringTheory) initializeStringCut(variable ast.ASTHandle) error {
	if _, exists := t.cutHistory[variable]; exists {
		return nil
	}
	_, literal, err := t.state.manager.SequenceStringValue(variable)
	if err != nil || literal {
		return err
	}
	// Native check_and_init_cut_var seeds nonliteral variables at -1, which
	// survives every ordinary scope pop, including a return to level zero.
	if err := t.retain(variable); err != nil {
		return err
	}
	if err := t.retain(variable); err != nil {
		return err
	}
	return t.addStringCut(variable, -1, map[ast.ASTHandle]struct{}{variable: {}})
}

func (t *SMTStringTheory) mergeStringCut(destination ast.ASTHandle, level int64, source ast.ASTHandle) error {
	if err := t.retain(destination); err != nil {
		return err
	}
	if err := t.retain(source); err != nil {
		return err
	}
	history := t.cutHistory[source]
	if len(history) == 0 {
		return ast.ErrASTDomain
	}
	// A valid pop deletes every emptied entry. The source's dest.empty()
	// branch nevertheless reads dest.top(); no reachable empty-map adapter
	// or invented empty-history union is introduced here.
	if dest, found := t.cutHistory[destination]; found && len(dest) == 0 {
		return ast.ErrASTDomain
	}
	return t.addStringCut(destination, level, history[len(history)-1].vars)
}

func (t *SMTStringTheory) popStringCuts() {
	for variable, history := range t.cutHistory {
		for len(history) != 0 {
			last := history[len(history)-1]
			if last.level == 0 || last.level < int64(t.level) {
				break
			}
			history[len(history)-1] = smtStringCut{}
			history = history[:len(history)-1]
		}
		if len(history) == 0 {
			delete(t.cutHistory, variable)
		} else {
			t.cutHistory[variable] = history
		}
	}
}
