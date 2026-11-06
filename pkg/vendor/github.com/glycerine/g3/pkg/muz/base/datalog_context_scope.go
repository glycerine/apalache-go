package base

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

type datalogContextScope struct {
	rules      *RuleSet
	background int
	restored   bool
	popping    bool
}

func (c *DatalogContext) Push() error {
	if err := c.open(); err != nil {
		return err
	}
	snapshot, err := NewRuleSet(c.ruleManager)
	if err != nil {
		return err
	}
	if err = snapshot.AddRules(c.rules); err != nil {
		cleanup := snapshot.Close()
		if cleanup != nil {
			c.pendingScope = snapshot
		}
		return errors.Join(err, cleanup)
	}
	c.scopes = append(c.scopes, datalogContextScope{rules: snapshot, background: len(c.background)})
	return nil
}

func (c *DatalogContext) Pop(count uint32) error {
	if c == nil || c.scopeRecovery {
		return ErrDatalogContextOwner
	}
	c.scopeRecovery = true
	defer func() { c.scopeRecovery = false }()
	if c.popCount != 0 && count != c.popCount {
		return ErrDatalogContextOwner
	}
	if err := c.openForScopeCleanup(); err != nil {
		return err
	}
	if c.popCount == 0 {
		if uint64(count) > uint64(len(c.scopes)) {
			return fmt.Errorf("muz/base: datalog scope underflow")
		}
		c.popCount = count
		c.popRemaining = count
	}
	for c.popRemaining > 0 {
		last := len(c.scopes) - 1
		target := &c.scopes[last]
		target.popping = true
		for len(c.background) > target.background {
			index := len(c.background) - 1
			err := c.manager.DelRef(&c.background[index])
			if c.background[index] == (ast.ASTRef{}) {
				c.background = c.background[:index]
			}
			if err != nil {
				return err
			}
		}
		if !target.restored {
			replacement, err := NewRuleSet(c.ruleManager)
			if err != nil {
				return err
			}
			if err = replacement.AddRules(target.rules); err != nil {
				cleanup := replacement.Close()
				if cleanup != nil {
					c.pendingScope = replacement
				}
				return errors.Join(err, cleanup)
			}
			c.rules.exchangeContents(replacement)
			target.restored = true
			if err = replacement.Close(); err != nil {
				c.pendingScope = replacement
				return err
			}
		}
		if err := target.rules.Close(); err != nil {
			return err
		}
		c.scopes = c.scopes[:last]
		c.popRemaining--
	}
	c.popCount = 0
	return nil
}
