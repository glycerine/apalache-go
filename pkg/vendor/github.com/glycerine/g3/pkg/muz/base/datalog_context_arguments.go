package base

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) SetArgumentNames(predicate ast.ASTHandle, names []usymbol.Symbol) error {
	if err := c.open(); err != nil {
		return err
	}
	if _, exists := c.argumentNames[predicate]; exists {
		return fmt.Errorf("muz/base: predicate argument names already set")
	}
	arity, err := c.manager.GetArity(predicate)
	if err != nil {
		return err
	}
	if uint32(len(names)) != arity {
		return fmt.Errorf("muz/base: predicate argument name count mismatch")
	}
	c.argumentNames[predicate] = append([]usymbol.Symbol(nil), names...)
	return nil
}
func (c *DatalogContext) ArgumentName(predicate ast.ASTHandle, index uint32) (usymbol.Symbol, error) {
	if err := c.open(); err != nil {
		return usymbol.Symbol{}, err
	}
	arity, err := c.manager.GetArity(predicate)
	if err != nil {
		return usymbol.Symbol{}, err
	}
	if index >= arity {
		return usymbol.Symbol{}, fmt.Errorf("muz/base: predicate argument index out of bounds")
	}
	if names, ok := c.argumentNames[predicate]; ok {
		return names[index], nil
	}
	return c.manager.Symbols.Intern(fmt.Sprintf("#%d", index))
}
