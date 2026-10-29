package rel

// Source: src/muz/rel/dl_instruction.cpp instr_filter_interpreted.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type interpretedFilterInstruction struct {
	accounting      *base.AccountedObject
	manager         *ast.ASTManager
	relations       *RelationManager
	rewriter        *astrewriter.TheoryRewriter
	register        uint32
	conditions      []ast.ASTRef
	variableColumns map[uint32]int
	variableSorts   map[uint32]ast.ASTHandle
	closed          bool
}

// NewFilterInterpretedInstruction retains conditions and copies the concrete
// variable-to-column binding. The manager, relation manager, and rewriter are
// borrowed until the instruction is closed.
func NewFilterInterpretedInstruction(manager *ast.ASTManager, relations *RelationManager, rewriter *astrewriter.TheoryRewriter, register uint32, conditions []ast.ASTHandle, columns map[uint32]int, sorts map[uint32]ast.ASTHandle) (Instruction, error) {
	if manager == nil || relations == nil || rewriter == nil || len(conditions) == 0 {
		return nil, ErrInstruction
	}
	i := &interpretedFilterInstruction{
		accounting: base.NewAccountedObject(), manager: manager, relations: relations,
		rewriter: rewriter, register: register, variableColumns: make(map[uint32]int, len(columns)),
		variableSorts: make(map[uint32]ast.ASTHandle, len(sorts)),
	}
	for variable, column := range columns {
		i.variableColumns[variable] = column
	}
	for variable, sort := range sorts {
		i.variableSorts[variable] = sort
	}
	for _, condition := range conditions {
		reference, err := manager.NewRef(condition)
		if err != nil {
			return nil, errors.Join(err, i.Close())
		}
		i.conditions = append(i.conditions, reference)
	}
	return i, nil
}

func (i *interpretedFilterInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil || i.manager == nil || i.relations == nil || i.rewriter == nil {
		return ErrInstruction
	}
	return nil
}

func (i *interpretedFilterInstruction) accepts(fact Fact) (accepted bool, resultErr error) {
	var maximum uint32
	for variable := range i.variableSorts {
		if variable > maximum {
			maximum = variable
		}
	}
	bindings := make([]ast.ASTHandle, 0)
	if len(i.variableSorts) != 0 {
		bindings = make([]ast.ASTHandle, int(maximum)+1)
	}
	references := make([]ast.ASTRef, 0, len(i.variableSorts)+1)
	defer func() {
		for index := len(references) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, i.manager.DelRef(&references[index]))
		}
	}()
	for variable, sort := range i.variableSorts {
		column, found := i.variableColumns[variable]
		if !found || column < 0 || column >= len(fact) {
			return false, ErrInstruction
		}
		reference, err := i.relations.DecodeValue(sort, fact[column])
		if err != nil {
			return false, err
		}
		references = append(references, reference)
		handle, err := i.manager.RefGet(reference)
		if err != nil {
			return false, err
		}
		bindings[len(bindings)-int(variable)-1] = handle
	}
	for _, condition := range i.conditions {
		handle, err := i.manager.RefGet(condition)
		if err != nil {
			return false, err
		}
		out, err := i.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return false, err
		}
		references = append(references, out)
		if err = i.rewriter.RewriteWithBindings(handle, bindings, out); err != nil {
			return false, err
		}
		rewritten, err := i.manager.RefGet(out)
		if err != nil {
			return false, err
		}
		if i.manager.IsFalse(rewritten) {
			return false, nil
		}
		if !i.manager.IsTrue(rewritten) {
			return false, ErrInstruction
		}
	}
	return true, nil
}

func (i *interpretedFilterInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	table, err := context.Register(i.register)
	if err != nil || table == nil {
		return true, err
	}
	context.stats.Filter++
	for key, fact := range table.rows.All() {
		accepted, x := i.accepts(fact)
		if x != nil {
			return false, x
		}
		if !accepted {
			table.rows.Del(key)
		}
	}
	if table.rows.Len() == 0 {
		return true, context.MakeEmpty(i.register)
	}
	return true, nil
}

func (i *interpretedFilterInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	return context.SetRegisterAnnotation(i.register, "filter_interpreted")
}

func (i *interpretedFilterInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil || out == nil {
		return errors.Join(ErrInstruction, err)
	}
	_, err := fmt.Fprintf(out, "filter_interpreted %d", i.register)
	return err
}

func (i *interpretedFilterInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *interpretedFilterInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *interpretedFilterInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *interpretedFilterInstruction) Close() (result error) {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	for index := len(i.conditions) - 1; index >= 0; index-- {
		result = errors.Join(result, i.manager.DelRef(&i.conditions[index]))
	}
	i.conditions = nil
	result = errors.Join(result, i.accounting.Close())
	return result
}
