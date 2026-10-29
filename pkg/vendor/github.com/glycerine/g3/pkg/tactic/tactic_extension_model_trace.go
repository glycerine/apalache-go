package tactic

// Source: src/tactic/extension_model_converter.cpp:29-49,59,71; src/util/trace.h:51.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// A TRACE ostream keeps its failed state but does not make an insertion a
// computation error. The outer TraceTags writer still records the refusal.
type extensionTraceOstream struct{ out io.Writer }

func (w extensionTraceOstream) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func (c *tacticExtensionModelConverter) extensionTraceTags() *diagnostic.TraceTags {
	if c.services.Trace != nil {
		return c.services.Trace
	}
	return c.manager.BorrowMemoryTraceTags()
}

func (c *tacticExtensionModelConverter) writeExtensionSMT2(w io.Writer, h ast.ASTHandle) (result error) {
	return c.writeExtensionSMT2Indented(w, h, 0)
}

func (c *tacticExtensionModelConverter) writeExtensionSMT2Indented(w io.Writer, h ast.ASTHandle, indent uint32) (result error) {
	format, err := c.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return err
	}
	if indent != 0 {
		format, err = c.manager.FormatIndent(indent, format)
		if err != nil {
			return err
		}
	}
	return c.writeExtensionFormat(w, format)
}

func (c *tacticExtensionModelConverter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil || len(c.vars) != len(c.defs) {
		return ast.ErrASTHandle
	}
	if _, err := io.WriteString(out, "(extension-model-converter"); err != nil {
		return err
	}
	for i, declaration := range c.vars {
		name, err := c.manager.GetASTName(declaration)
		if err != nil {
			return err
		}
		spelling, err := c.manager.Symbols.String(name)
		if err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n  ("+spelling+" "); err != nil {
			return err
		}
		if err = c.writeExtensionSMT2Indented(out, c.defs[i], uint32(len(spelling))+4); err != nil {
			return err
		}
		if _, err = io.WriteString(out, ")"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, ")\n")
	return err
}

func (c *tacticExtensionModelConverter) writeExtensionFormat(w io.Writer, format ast.ASTHandle) (result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: c.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	empty := c.services.Context.EmptyValue()
	if err = c.manager.PrintFormat(out, format, c.services.Context, &empty, c.services.Globals); err != nil {
		return err
	}
	body, err := out.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, body)
	return err
}

func (c *tacticExtensionModelConverter) writeExtensionSort(w io.Writer, sort ast.ASTHandle) error {
	format, err := c.manager.SMT2PPSort(sort)
	if err != nil {
		return err
	}
	return c.writeExtensionFormat(w, format)
}

func (c *tacticExtensionModelConverter) traceExtensionModel(md *model.ModelCore, line int) error {
	tags := c.extensionTraceTags()
	if !tags.ShouldTrace("extension_mc") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("extension_mc", "operator()", "src/tactic/extension_model_converter.cpp", line,
		func() bool { return true }, func(w io.Writer) (bodyErr error) {
			defer func() { operandErr = bodyErr }()
			w = extensionTraceOstream{w}
			if err := md.WriteV2(w, c.services.Context, c.services.Globals, false); err != nil {
				return err
			}
			n, err := md.NumDeclarations()
			if err != nil {
				return err
			}
			for i := uint32(0); i < n; i++ {
				d, err := md.Declaration(i)
				if err != nil {
					return err
				}
				name, err := c.manager.GetASTName(d)
				if err != nil {
					return err
				}
				spelling, err := c.manager.Symbols.String(name)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, spelling+" ("); err != nil {
					return err
				}
				arity, err := c.manager.GetArity(d)
				if err != nil {
					return err
				}
				for j := uint32(0); j < arity; j++ {
					sort, err := c.manager.GetDomain(d, j)
					if err != nil {
						return err
					}
					if err = c.writeExtensionSort(w, sort); err != nil {
						return err
					}
				}
				sort, err := c.manager.GetRange(d)
				if err != nil {
					return err
				}
				if err = c.writeExtensionSort(w, sort); err != nil {
					return err
				}
				if _, err = io.WriteString(w, ") "); err != nil {
					return err
				}
				info, present, err := c.manager.FuncDeclInfoText(d)
				if err != nil {
					return err
				}
				if present {
					if _, err = io.WriteString(w, info); err != nil {
						return err
					}
				}
				id, err := c.manager.GetASTID(d)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, " :id "+strconv.FormatUint(uint64(id), 10)+"\n"); err != nil {
					return err
				}
			}
			return nil
		})
	return operandErr
}

func (c *tacticExtensionModelConverter) traceExtensionDefinition(decl, definition, value ast.ASTHandle) error {
	tags := c.extensionTraceTags()
	if !tags.ShouldTrace("extension_mc") {
		return nil
	}
	name, err := c.manager.GetASTName(decl)
	if err != nil {
		return err
	}
	spelling, err := c.manager.Symbols.String(name)
	if err != nil {
		return err
	}
	var operandErr error
	_ = tags.EmitConditional("extension_mc", "operator()", "src/tactic/extension_model_converter.cpp", 59,
		func() bool { return true }, func(w io.Writer) (bodyErr error) {
			defer func() { operandErr = bodyErr }()
			w = extensionTraceOstream{w}
			if _, err := io.WriteString(w, spelling+" ->\n"); err != nil {
				return err
			}
			if err := c.writeExtensionSMT2(w, definition); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n==>\n"); err != nil {
				return err
			}
			if err := c.writeExtensionSMT2(w, value); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
	return operandErr
}
