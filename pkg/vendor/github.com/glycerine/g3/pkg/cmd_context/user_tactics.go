package cmd_context

// Source: src/cmd_context/cmd_context.cpp insert_user_tactic,
// find_user_tactic and reset_user_tactics; tactic_cmds.cpp.

import (
	"fmt"

	"github.com/glycerine/insdict"
)

// UserTacticDeclaration is an insertion-ordered name and SMT-LIB tactic body.
type UserTacticDeclaration struct {
	Name, Body string
}

// DeclareUserTactic stores a validated tactic body, replacing a declaration
// with the same name while preserving its dictionary position.
func (c *Context) DeclareUserTactic(name, body string) error {
	if c == nil || c.closed || name == "" || body == "" {
		return fmt.Errorf("invalid user tactic declaration")
	}
	if c.userTactics == nil {
		c.userTactics = insdict.NewDict[string, string]()
	}
	c.userTactics.Set(name, body)
	return nil
}

// UserTacticBody returns the stored body for a previously declared tactic.
func (c *Context) UserTacticBody(name string) (string, bool) {
	if c == nil || c.closed || c.userTactics == nil {
		return "", false
	}
	return c.userTactics.Get2(name)
}

// UserTacticDeclarations returns a snapshot in source dictionary iteration
// order. Strings are immutable Go values and need no separate refcount owner.
func (c *Context) UserTacticDeclarations() []UserTacticDeclaration {
	if c == nil || c.closed || c.userTactics == nil {
		return nil
	}
	declarations := make([]UserTacticDeclaration, 0, c.userTactics.Len())
	for name, body := range c.userTactics.All() {
		declarations = append(declarations, UserTacticDeclaration{Name: name, Body: body})
	}
	return declarations
}
