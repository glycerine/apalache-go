package model

// Source: src/model/model_implicant.cpp:480,506,515,519,526,529.

import (
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (i *ModelImplicant) traceArrayExtraction(line int, prefix string, terms ...ast.ASTHandle) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "extract_array_func_interp", "src/model/model_implicant.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			_, _ = io.WriteString(out, prefix)
			for _, term := range terms {
				if term.IsNull() {
					_, _ = io.WriteString(out, "null\n")
					continue
				}
				if err := i.evaluator.writeSMT2(out, term); err != nil {
					operandErr = err
					return err
				}
				_, _ = io.WriteString(out, "\n")
			}
			return nil
		})
	return operandErr
}

// Source: model_implicant::eval_array_eq, model_implicant.cpp:538-609.
func (i *ModelImplicant) traceArrayEquality(line int, prefix string, terms ...ast.ASTHandle) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "eval_array_eq", "src/model/model_implicant.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			_, _ = io.WriteString(out, prefix)
			for n, term := range terms {
				if err := i.evaluator.writeSMT2(out, term); err != nil {
					operandErr = err
					return err
				}
				separator := "\n"
				if line == 565 && n+1 < len(terms) {
					separator = " "
				}
				if line == 597 && (n == 1 || n == 3) {
					separator = " |-> "
				}
				_, _ = io.WriteString(out, separator)
			}
			return nil
		})
	return operandErr
}

// Source: model_implicant::assign_value, model_implicant.cpp:65-66.
func (i *ModelImplicant) reportUntrackedValue(expression, value ast.ASTHandle) error {
	enabled, err := diagnostic.ProcessVerboseEnabled(3)
	if err != nil {
		return err
	}
	if enabled {
		out, err := diagnostic.BorrowProcessVerboseStream()
		if err != nil {
			return err
		}
		print := func(h ast.ASTHandle) error {
			if h.IsNull() {
				return out.AppendCString("null")
			}
			format, err := i.manager.SMT2PPExprTree(h, nil)
			if err != nil {
				return err
			}
			empty := i.services.Context.EmptyValue()
			return i.manager.PrintFormat(out, format, i.services.Context, &empty, i.services.Globals)
		}
		if err = out.AppendCString("Not evaluated "); err != nil {
			return err
		}
		if err = print(expression); err != nil {
			return err
		}
		if err = out.AppendCString(" := "); err != nil {
			return err
		}
		if err = print(value); err != nil {
			return err
		}
		if err = out.AppendCString("\n"); err != nil {
			return err
		}
	}
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr", "assign_value", "src/model/model_implicant.cpp", 66, func() bool { return true }, func(out io.Writer) error {
		_, _ = io.WriteString(out, "Variable is not tracked: ")
		if err := i.evaluator.writeSMT2(out, expression); err != nil {
			operandErr = err
			return err
		}
		_, _ = io.WriteString(out, " := ")
		if value.IsNull() {
			_, _ = io.WriteString(out, "null")
		} else {
			if err := i.evaluator.writeSMT2(out, value); err != nil {
				operandErr = err
				return err
			}
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
	return operandErr
}

// Source: model_implicant::set_number, model_implicant.h:77.
func (i *ModelImplicant) traceNumber(expression ast.ASTHandle, value *big.Rat) error {
	tags := i.services.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pdr_verbose") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("pdr_verbose", "set_number", "src/model/model_implicant.h", 77, func() bool { return true }, func(out io.Writer) error {
		operandErr = i.evaluator.writeSMT2(out, expression)
		if operandErr != nil {
			return operandErr
		}
		_, _ = io.WriteString(out, " "+value.RatString()+"\n")
		return nil
	})
	return operandErr
}
