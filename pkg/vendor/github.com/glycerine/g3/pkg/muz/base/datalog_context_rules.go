package base

import "github.com/glycerine/g3/pkg/ast"

func (c *DatalogContext) AddRule(rule *Rule) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.rules.Add(rule)
}
func (c *DatalogContext) SetOutputPredicate(predicate ast.ASTHandle) error {
	if err := c.open(); err != nil {
		return err
	}
	return c.rules.SetOutputPredicate(predicate)
}
func (c *DatalogContext) Finalize() (bool, error) {
	if err := c.open(); err != nil {
		return false, err
	}
	return c.rules.Finalize()
}
func (c *DatalogContext) Reopen() error {
	if err := c.open(); err != nil {
		return err
	}
	return c.rules.Reopen()
}
func (c *DatalogContext) RuleCount() (int, error) {
	if err := c.open(); err != nil {
		return 0, err
	}
	return c.rules.Len()
}
