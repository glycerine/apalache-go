package fp

// Source: src/muz/base/dl_context.cpp predicate argument-name and output
// metadata used by the legacy Datalog parser.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// SetArgumentNames records source-level names for a registered predicate.
func (e *PropositionalEngine) SetArgumentNames(declaration ast.ASTHandle, names []usymbol.Symbol) error {
	if err := e.open(); err != nil {
		return err
	}
	state := e.predicates[declaration]
	if state == nil {
		return ErrPropositionalHorn
	}
	if state.argumentNamesSet {
		return fmt.Errorf("muz/fp: predicate argument names already set")
	}
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return err
	}
	if uint32(len(names)) != arity {
		return fmt.Errorf("muz/fp: predicate argument name count mismatch")
	}
	state.argumentNames = append([]usymbol.Symbol(nil), names...)
	state.argumentNamesSet = true
	return nil
}

// ArgumentName returns a declared argument name, or the source fallback #N.
func (e *PropositionalEngine) ArgumentName(declaration ast.ASTHandle, index uint32) (usymbol.Symbol, error) {
	if err := e.open(); err != nil {
		return usymbol.Symbol{}, err
	}
	state := e.predicates[declaration]
	if state == nil {
		return usymbol.Symbol{}, ErrPropositionalHorn
	}
	arity, err := e.manager.GetArity(declaration)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if index >= arity {
		return usymbol.Symbol{}, fmt.Errorf("muz/fp: predicate argument index out of bounds")
	}
	if state.argumentNamesSet {
		return state.argumentNames[index], nil
	}
	return e.manager.Symbols.Intern(fmt.Sprintf("#%d", index))
}

// SetOutputPredicate marks a registered predicate as a program output.
func (e *PropositionalEngine) SetOutputPredicate(declaration ast.ASTHandle) error {
	if err := e.open(); err != nil {
		return err
	}
	if e.predicates[declaration] == nil {
		return ErrPropositionalHorn
	}
	return e.set.SetOutputPredicate(declaration)
}

// IsOutputPredicate reports whether a registered predicate is a program output.
func (e *PropositionalEngine) IsOutputPredicate(declaration ast.ASTHandle) (bool, error) {
	if err := e.open(); err != nil {
		return false, err
	}
	if e.predicates[declaration] == nil {
		return false, ErrPropositionalHorn
	}
	return e.set.IsOutputPredicate(declaration)
}
