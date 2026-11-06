package rel

// Source: src/muz/rel/dl_instruction.cpp instr_io load/store operations.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type relationIOInstruction struct {
	accounting    *base.AccountedObject
	manager       *ast.ASTManager
	relations     *RelationManager
	predicate     ast.ASTHandle
	register      uint32
	store         bool
	explanation   bool
	predicateHeld bool
	closing       bool
	closed        bool
}

func newRelationIOInstruction(relations *RelationManager, predicate ast.ASTHandle, register uint32, store bool) (Instruction, error) {
	if err := relations.open(); err != nil {
		return nil, err
	}
	explanation, err := relations.predicateUsesExplanationColumns(predicate)
	if err != nil {
		return nil, err
	}
	if explanation {
		if _, _, err = relations.explanationPredicateSignature(predicate); err != nil {
			return nil, err
		}
	} else if _, _, err = relations.PredicateSignature(predicate); err != nil {
		return nil, err
	}
	if err := relations.manager.IncRef(predicate); err != nil {
		return nil, err
	}
	return &relationIOInstruction{
		accounting:    base.NewAccountedObject(),
		manager:       relations.manager,
		relations:     relations,
		predicate:     predicate,
		register:      register,
		store:         store,
		explanation:   explanation,
		predicateHeld: true,
	}, nil
}

func NewLoadInstruction(relations *RelationManager, predicate ast.ASTHandle, target uint32) (Instruction, error) {
	return newRelationIOInstruction(relations, predicate, target, false)
}

func NewStoreInstruction(relations *RelationManager, predicate ast.ASTHandle, source uint32) (Instruction, error) {
	return newRelationIOInstruction(relations, predicate, source, true)
}

func (i *relationIOInstruction) open() error {
	if i == nil || i.closed || i.closing || i.accounting == nil || i.manager == nil || i.relations == nil || !i.predicateHeld {
		return ErrInstruction
	}
	return i.relations.open()
}

func (i *relationIOInstruction) contextRelations(context *ExecutionContext) (*RelationManager, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	if context.relations != i.relations {
		return nil, ErrInstruction
	}
	if err := i.relations.open(); err != nil {
		return nil, err
	}
	return i.relations, nil
}

func (i *relationIOInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	relations, err := i.contextRelations(context)
	if err != nil {
		return false, err
	}
	if i.explanation {
		return i.performExplanation(context, relations)
	}
	if !i.store {
		table, x := relations.GetFiniteRelation(i.predicate)
		if x != nil {
			return false, x
		}
		clone, x := table.CloneFinite()
		if x != nil {
			return false, x
		}
		return true, context.SetFiniteRegister(i.register, clone)
	}
	table, err := context.FiniteRegister(i.register)
	if err != nil {
		return false, err
	}
	transferred := table != nil
	if transferred {
		table, err = context.ReleaseFiniteRegister(i.register)
		if err != nil {
			return false, err
		}
	}
	if table == nil {
		selected, x := relations.GetFiniteRelation(i.predicate)
		if x != nil {
			return false, x
		}
		table, x = emptyFiniteLike(selected)
		if x != nil {
			return false, x
		}
	}
	if err = relations.StoreFiniteRelation(i.predicate, table); err != nil {
		if transferred {
			_ = context.SetFiniteRegister(i.register, table)
		} else {
			_ = table.Close()
		}
		return false, err
	}
	return true, nil
}

func (i *relationIOInstruction) predicateName() (string, error) {
	name, err := i.manager.GetASTName(i.predicate)
	if err != nil {
		return "", err
	}
	return i.manager.Symbols.String(name)
}

func (i *relationIOInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	name, err := i.predicateName()
	if err != nil {
		return err
	}
	return context.SetRegisterAnnotation(i.register, name)
}

func (i *relationIOInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	name, err := i.predicateName()
	if err != nil {
		return err
	}
	if i.store {
		_, err = fmt.Fprintf(out, "store %d into %s", i.register, name)
	} else {
		_, err = fmt.Fprintf(out, "load %s into %d", name, i.register)
	}
	return err
}

func (i *relationIOInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *relationIOInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *relationIOInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *relationIOInstruction) Close() (result error) {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closing = true
	if i.predicateHeld {
		consumed, err := i.manager.DecRefConsumed(i.predicate)
		result = errors.Join(result, err)
		if consumed {
			i.predicateHeld = false
		}
		if !consumed {
			return result
		}
	}
	if err := i.accounting.Close(); err != nil {
		return errors.Join(result, err)
	}
	i.closed = true
	return result
}
