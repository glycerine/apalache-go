package rel

// Source: src/muz/rel/dl_instruction.cpp instr_mark_saturated.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type saturationInstruction struct {
	accounting *base.AccountedObject
	manager    *ast.ASTManager
	relations  *RelationManager
	predicate  ast.ASTHandle
	held       bool
	closed     bool
}

func NewMarkSaturatedInstruction(relations *RelationManager, predicate ast.ASTHandle) (Instruction, error) {
	if err := relations.open(); err != nil {
		return nil, err
	}
	if _, _, err := relations.PredicateSignature(predicate); err != nil {
		return nil, err
	}
	if err := relations.manager.IncRef(predicate); err != nil {
		return nil, err
	}
	return &saturationInstruction{accounting: base.NewAccountedObject(), manager: relations.manager, relations: relations, predicate: predicate, held: true}, nil
}

func (i *saturationInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil || i.manager == nil || i.relations == nil || !i.held {
		return ErrInstruction
	}
	return i.relations.open()
}

func (i *saturationInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	if context == nil || context.relations != i.relations {
		return false, ErrInstruction
	}
	return true, i.relations.MarkSaturated(i.predicate)
}

func (i *saturationInstruction) MakeAnnotations(*ExecutionContext) error { return i.open() }

func (i *saturationInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	name, err := i.manager.GetASTName(i.predicate)
	if err != nil {
		return err
	}
	text, err := i.manager.Symbols.String(name)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "mark_saturated %s", text)
	return err
}

func (i *saturationInstruction) Accounting() *base.AccountedObject { return i.accounting }
func (i *saturationInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}
func (i *saturationInstruction) CollectStatistics(out *statistics.Statistics) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	cost, err := i.accounting.TotalCosts()
	if err != nil {
		return err
	}
	out.UpdateUint("instruction", cost.Instructions)
	out.UpdateUint("instruction-time", cost.Milliseconds)
	return nil
}

func (i *saturationInstruction) Close() (result error) {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	if i.held {
		consumed, err := i.manager.DecRefConsumed(i.predicate)
		result = errors.Join(result, err)
		if !consumed {
			return result
		}
		i.held = false
	}
	result = errors.Join(result, i.accounting.Close())
	i.closed = true
	return result
}
