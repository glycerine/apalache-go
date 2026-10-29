package smtcore

// Source: theory_datatype::mk_split and datatype::util constructor preferences.

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"io"
)

func (t *SMTDatatypeTheory) split(value, sort ast.ASTHandle) error {
	s, err := t.State()
	if err != nil {
		return err
	}
	node := s.context.enodes[value]
	if node == nil {
		return ast.ErrASTDomain
	}
	root, data, err := t.rootData(node.TheoryVar(t.FamilyID()))
	if err != nil {
		return err
	}
	if data.constructor != nil {
		return ast.ErrASTDomain
	}
	node = t.variables[root]
	if t.utility == nil {
		t.utility, err = ast.NewDatatypeUtil(s.manager)
		if err != nil {
			return err
		}
	}
	preferred, found, err := t.utility.NonRecursiveConstructor(sort)
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTDomain
	}
	if err = t.emitDatatype("datatype_bug", "mk_split", 829, func(out io.Writer) error {
		_, _ = io.WriteString(out, "non_rec_c: ")
		name, err := s.manager.GetASTName(preferred)
		if err != nil {
			return err
		}
		text, err := s.manager.Symbols.Display(name)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, text)
		return err
	}); err != nil {
		return err
	}
	index, found, err := s.manager.DatatypeConstructorIndex(preferred)
	if err != nil {
		return err
	}
	if !found {
		return ast.ErrASTDomain
	}
	t.statistics.splits++
	activate := func(recognizer *SMTEnode) (bool, error) {
		if !s.isRelevant(recognizer.owner) {
			return true, s.markRelevant(recognizer.owner)
		}
		literal, err := s.Literal(recognizer.owner)
		if err != nil {
			return false, err
		}
		assignment, err := s.Assignment(literal)
		return assignment != lbool.False, err
	}
	constructor := preferred
	if len(data.recognizers) > 0 && data.recognizers[index] != nil {
		stop, err := activate(data.recognizers[index])
		if err != nil || stop {
			return err
		}
		constructors, err := s.manager.DatatypeConstructors(sort)
		if err != nil {
			return err
		}
		constructor = ast.ASTHandle{}
		for i, recognizer := range data.recognizers {
			if recognizer == nil {
				constructor = constructors[i]
				break
			}
			stop, err := activate(recognizer)
			if err != nil || stop {
				return err
			}
		}
		if constructor.IsNull() {
			return nil
		}
	}
	expression, err := t.recognizerApplication(constructor, node.owner)
	if err != nil {
		return err
	}
	if err = t.traceDatatypeExpression("mk_split", 879, "creating split: ", expression); err != nil {
		return err
	}
	if err = s.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := s.Literal(expression)
	if err != nil {
		return err
	}
	if _, err = s.setScopedTrueFirst(literal.Var()); err != nil {
		return err
	}
	return s.markRelevant(expression)
}

func (t *SMTDatatypeTheory) Close() error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if t.utility != nil {
		if err := t.utility.Close(); err != nil {
			return err
		}
		t.utility = nil
	}
	if err := t.findTrail.Reset(); err != nil {
		return err
	}
	t.data = nil
	t.scopes = nil
	t.mergeError = nil
	t.trace = nil
	t.variables = nil
	t.limits = nil
	return nil
}
