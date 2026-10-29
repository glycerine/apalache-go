package rel

// Source: src/muz/rel/dl_instruction.cpp concrete finite-table join, filter,
// union, project/rename, join-project, selection, and negation instructions.

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/statistics"
)

const VoidRegister = ^uint32(0)

type tableInstruction struct {
	accounting *base.AccountedObject
	kind       string
	first      uint32
	second     uint32
	result     uint32
	left       []int
	right      []int
	removed    []int
	value      Element
	column     int
	closed     bool
}

func newTableInstruction(kind string) *tableInstruction {
	return &tableInstruction{accounting: base.NewAccountedObject(), kind: kind, second: VoidRegister, result: VoidRegister}
}

func NewJoinInstruction(first, second uint32, left, right []int, result uint32) Instruction {
	i := newTableInstruction("join")
	i.first, i.second, i.result = first, second, result
	i.left, i.right = append([]int(nil), left...), append([]int(nil), right...)
	return i
}

func NewFilterEqualInstruction(register uint32, value Element, column int) Instruction {
	i := newTableInstruction("filter_equal")
	i.first, i.value, i.column = register, value, column
	return i
}

func NewFilterIdenticalInstruction(register uint32, columns []int) Instruction {
	i := newTableInstruction("filter_identical")
	i.first, i.left = register, append([]int(nil), columns...)
	return i
}

func NewUnionInstruction(source, target, delta uint32) Instruction {
	i := newTableInstruction("union")
	i.first, i.second, i.result = source, target, delta
	return i
}

func NewWidenInstruction(source, target, delta uint32) Instruction {
	i := newTableInstruction("widen")
	i.first, i.second, i.result = source, target, delta
	return i
}

func NewProjectionInstruction(source uint32, removed []int, target uint32) Instruction {
	i := newTableInstruction("project")
	i.first, i.result, i.removed = source, target, append([]int(nil), removed...)
	return i
}

func NewRenameInstruction(source uint32, cycle []int, target uint32) Instruction {
	i := newTableInstruction("rename")
	i.first, i.result, i.left = source, target, append([]int(nil), cycle...)
	return i
}

func NewJoinProjectInstruction(first, second uint32, left, right, removed []int, result uint32) Instruction {
	i := newTableInstruction("join_project")
	i.first, i.second, i.result = first, second, result
	i.left, i.right, i.removed = append([]int(nil), left...), append([]int(nil), right...), append([]int(nil), removed...)
	return i
}

func NewSelectEqualProjectInstruction(source uint32, value Element, column int, result uint32) Instruction {
	i := newTableInstruction("select_equal_and_project")
	i.first, i.result, i.value, i.column = source, result, value, column
	return i
}

func NewFilterByNegationInstruction(target, negated uint32, targetColumns, negatedColumns []int) Instruction {
	i := newTableInstruction("filter_by_negation")
	i.first, i.second = target, negated
	i.left, i.right = append([]int(nil), targetColumns...), append([]int(nil), negatedColumns...)
	return i
}

func (i *tableInstruction) open() error {
	if i == nil || i.closed || i.accounting == nil {
		return ErrInstruction
	}
	return nil
}

func instructionRegister(context *ExecutionContext, index uint32) (*HashTable, error) {
	if index == VoidRegister {
		return nil, nil
	}
	return context.Register(index)
}

func setNonemptyRegister(context *ExecutionContext, index uint32, table *HashTable) error {
	if table == nil {
		return context.MakeEmpty(index)
	}
	count, err := table.Len()
	if err != nil {
		return err
	}
	if count == 0 {
		return context.MakeEmpty(index)
	}
	return context.SetRegister(index, table)
}

func emptyLike(table *HashTable) (*HashTable, error) {
	signature, err := table.Signature()
	if err != nil {
		return nil, err
	}
	return NewHashTable(signature), nil
}

func (i *tableInstruction) Perform(context *ExecutionContext) (bool, error) {
	if err := i.open(); err != nil {
		return false, err
	}
	first, err := instructionRegister(context, i.first)
	if err != nil {
		return false, err
	}
	second, err := instructionRegister(context, i.second)
	if err != nil {
		return false, err
	}
	switch i.kind {
	case "join", "join_project":
		if i.kind == "join" {
			context.stats.Join++
		}
		if first == nil || second == nil {
			return true, context.MakeEmpty(i.result)
		}
		if i.kind == "join_project" {
			context.stats.JoinProject++
		}
		value, x := first.Join(second, i.left, i.right)
		if x == nil && i.kind == "join_project" {
			value, x = value.Project(i.removed)
		}
		if x != nil {
			return false, x
		}
		return true, setNonemptyRegister(context, i.result, value)
	case "filter_equal":
		context.stats.FilterEqual++
		if first == nil {
			return true, nil
		}
		if err = first.FilterEqual(i.value, i.column); err != nil {
			return false, err
		}
		count, x := first.Len()
		if x != nil {
			return false, x
		}
		if count == 0 {
			return true, context.MakeEmpty(i.first)
		}
		return true, nil
	case "filter_identical":
		context.stats.FilterIdentical++
		if first == nil {
			return true, nil
		}
		if err = first.FilterIdentical(i.left); err != nil {
			return false, err
		}
		count, x := first.Len()
		if x != nil {
			return false, x
		}
		if count == 0 {
			return true, context.MakeEmpty(i.first)
		}
		return true, nil
	case "union", "widen":
		if first == nil {
			return true, nil
		}
		context.stats.Union++
		target := second
		if target == nil {
			target, err = emptyLike(first)
			if err != nil {
				return false, err
			}
			if err = context.SetRegister(i.second, target); err != nil {
				return false, err
			}
		}
		var delta *HashTable
		if i.result != VoidRegister {
			delta, err = context.Register(i.result)
			if err != nil {
				return false, err
			}
			if delta == nil {
				delta, err = emptyLike(target)
				if err != nil {
					return false, err
				}
				if err = context.SetRegister(i.result, delta); err != nil {
					return false, err
				}
			}
		}
		if err = target.Union(first, delta); err != nil {
			return false, err
		}
		if delta != nil {
			count, x := delta.Len()
			if x != nil {
				return false, x
			}
			if count == 0 {
				return true, context.MakeEmpty(i.result)
			}
		}
		return true, nil
	case "project", "rename":
		if first == nil {
			return true, context.MakeEmpty(i.result)
		}
		context.stats.ProjectRename++
		var value *HashTable
		if i.kind == "project" {
			value, err = first.Project(i.removed)
		} else {
			value, err = first.RenameCycle(i.left)
		}
		if err != nil {
			return false, err
		}
		return true, context.SetRegister(i.result, value)
	case "select_equal_and_project":
		if first == nil {
			return true, context.MakeEmpty(i.result)
		}
		context.stats.SelectEqualProject++
		value, x := first.SelectEqualAndProject(i.value, i.column)
		if x != nil {
			return false, x
		}
		return true, setNonemptyRegister(context, i.result, value)
	case "filter_by_negation":
		if first == nil || second == nil {
			return true, nil
		}
		context.stats.FilterByNegation++
		if err = first.FilterByNegation(second, i.left, i.right); err != nil {
			return false, err
		}
		count, x := first.Len()
		if x != nil {
			return false, x
		}
		if count == 0 {
			return true, context.MakeEmpty(i.first)
		}
		return true, nil
	default:
		return false, ErrInstruction
	}
}

func annotationOr(context *ExecutionContext, index uint32, fallback string) (string, error) {
	value, found, err := context.RegisterAnnotation(index)
	if err != nil {
		return "", err
	}
	if !found {
		return fallback, nil
	}
	return value, nil
}

func (i *tableInstruction) MakeAnnotations(context *ExecutionContext) error {
	if err := i.open(); err != nil {
		return err
	}
	switch i.kind {
	case "join":
		left, err := annotationOr(context, i.first, "rel1")
		if err != nil {
			return err
		}
		// Preserve the source's second lookup typo: a2 remains its fallback.
		right := "rel2"
		return context.SetRegisterAnnotation(i.result, "join "+left+" "+right)
	case "filter_equal":
		return context.SetRegisterAnnotation(i.first, fmt.Sprintf("filter_equal %d val: %d", i.column, i.value))
	case "filter_identical":
		return context.SetRegisterAnnotation(i.first, "filter_identical")
	case "union", "widen":
		value, err := annotationOr(context, i.second, "union")
		if err != nil {
			return err
		}
		if _, found, x := context.RegisterAnnotation(i.second); x != nil {
			return x
		} else if !found {
			if x = context.SetRegisterAnnotation(i.second, "union"); x != nil {
				return x
			}
		}
		if i.result != VoidRegister {
			value = "delta of " + value
		}
		return context.SetRegisterAnnotation(i.result, value)
	case "project", "rename":
		value, err := annotationOr(context, i.first, "rel_src")
		if err != nil {
			return err
		}
		return context.SetRegisterAnnotation(i.result, i.kind+" "+value)
	case "join_project":
		left, err := annotationOr(context, i.first, "rel1")
		if err != nil {
			return err
		}
		right, err := annotationOr(context, i.second, "rel2")
		if err != nil {
			return err
		}
		return context.SetRegisterAnnotation(i.result, "join project "+left+" "+right)
	case "select_equal_and_project":
		value, err := annotationOr(context, i.first, "src")
		if err != nil {
			return err
		}
		return context.SetRegisterAnnotation(i.result, fmt.Sprintf("select equal project col %d val: %d %s", i.column, i.value, value))
	case "filter_by_negation":
		value, err := annotationOr(context, i.second, "negated relation")
		if err != nil {
			return err
		}
		return context.SetRegisterAnnotation(i.first, "filter by negation "+value)
	default:
		return ErrInstruction
	}
}

func columnsText(columns []int) string {
	parts := make([]string, len(columns))
	for index, column := range columns {
		parts[index] = fmt.Sprint(column)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (i *tableInstruction) DisplayHead(out io.Writer) error {
	if err := i.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	var text string
	switch i.kind {
	case "join":
		text = fmt.Sprintf("join %d%s and %d%s into %d", i.first, columnsText(i.left), i.second, columnsText(i.right), i.result)
	case "filter_equal":
		text = fmt.Sprintf("filter_equal %d col: %d val: %d", i.first, i.column, i.value)
	case "filter_identical":
		text = fmt.Sprintf("filter_identical %d %s", i.first, columnsText(i.left))
	case "union", "widen":
		text = fmt.Sprintf("%s %d into %d", i.kind, i.first, i.second)
		if i.result != VoidRegister {
			text += fmt.Sprintf(" with delta %d", i.result)
		}
	case "project":
		text = fmt.Sprintf("project %d into %d deleting columns %s", i.first, i.result, columnsText(i.removed))
	case "rename":
		text = fmt.Sprintf("rename %d into %d with cycle %s", i.first, i.result, columnsText(i.left))
	case "join_project":
		text = fmt.Sprintf("join_project %d%s and %d%s into %d removing columns %s", i.first, columnsText(i.left), i.second, columnsText(i.right), i.result, columnsText(i.removed))
	case "select_equal_and_project":
		text = fmt.Sprintf("select_equal_and_project %d into %d col: %d val: %d", i.first, i.result, i.column, i.value)
	case "filter_by_negation":
		text = fmt.Sprintf("filter_by_negation on %d%s with %d%s as the negated table", i.first, columnsText(i.left), i.second, columnsText(i.right))
	default:
		return ErrInstruction
	}
	_, err := io.WriteString(out, text)
	return err
}

func (i *tableInstruction) Accounting() *base.AccountedObject { return i.accounting }

func (i *tableInstruction) ProcessAllCosts() error {
	if err := i.open(); err != nil {
		return err
	}
	return i.accounting.ProcessCosts()
}

func (i *tableInstruction) CollectStatistics(out *statistics.Statistics) error {
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

func (i *tableInstruction) Close() error {
	if i == nil {
		return ErrInstruction
	}
	if i.closed {
		return nil
	}
	i.closed = true
	return i.accounting.Close()
}
