package fp

// Source: horn_tactic::cleanup plus tactic lifecycle hooks.

import (
	"github.com/glycerine/g3/pkg/tactic"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func tacticOwnerError(message string) error {
	return tactic.NewTacticException("horn tactic: " + message)
}

func (t *HornTactic) Cleanup() error { return t.open() }
func (t *HornTactic) Reset() error   { return t.open() }

func (t *HornTactic) SetLogic(logic usymbol.Symbol) error {
	if err := t.open(); err != nil {
		return err
	}
	t.logic = logic
	return nil
}

func (t *HornTactic) SetProgressCallback(callback func(uint32)) error {
	if err := t.open(); err != nil {
		return err
	}
	t.progress = callback
	return nil
}
