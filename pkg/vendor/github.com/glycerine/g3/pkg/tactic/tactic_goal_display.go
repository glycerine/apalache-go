package tactic

// Source: src/tactic/goal.cpp:402-410.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func goalDisplayWrite(out io.Writer, value string) error {
	n, err := io.WriteString(out, value)
	if err != nil {
		return err
	}
	if n != len(value) {
		return io.ErrShortWrite
	}
	return nil
}

func (g *Goal) displaySMT2Formula(h ast.ASTHandle) (body string, result error) {
	global := uparams.BorrowProcessGlobalParamsService()
	if global == nil || global.Context == nil {
		return "", uparams.ErrGlobalParamsOwner
	}
	format, err := g.manager.SMT2PPExprTree(h, nil)
	if err != nil {
		return "", err
	}
	formats, err := g.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	ref, err := formats.NewRef(format)
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, formats.DelRef(&ref)) }()
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: g.manager.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	empty := global.Context.EmptyValue()
	if err = g.manager.PrintFormat(stream, format, global.Context, &empty, global); err != nil {
		return "", err
	}
	return stream.String()
}

// Display writes the source default goal printer's formula-only SMT2 form.
// The process parameter owner is borrowed only when a formula is printed.
func (g *Goal) Display(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	for _, form := range g.forms {
		if err := goalDisplayWrite(out, "\n  "); err != nil {
			return err
		}
		body, err := g.displaySMT2Formula(form)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, body); err != nil {
			return err
		}
	}
	return goalDisplayWrite(out, ")\n")
}

func goalPrecisionText(value GoalPrecision) (string, error) {
	switch value {
	case GoalPrecise:
		return "precise", nil
	case GoalUnder:
		return "under", nil
	case GoalOver:
		return "over", nil
	case GoalUnderOver:
		return "under-over", nil
	default:
		return "", ast.ErrASTDomain
	}
}

// DisplayWithAttributes is the source ast_printer display profile.
func (g *Goal) DisplayWithAttributes(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	for _, form := range g.forms {
		body, err := g.displaySMT2Formula(form)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, "\n  "+body); err != nil {
			return err
		}
	}
	precision, err := goalPrecisionText(g.precision)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, "\n  :precision "+precision+" :depth "+strconv.FormatUint(uint64(g.depth), 10)+")\n")
}

func (g *Goal) dependencyIsUninterpretedConstant(h ast.ASTHandle) (bool, error) {
	kind, err := g.manager.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	n, err := g.manager.GetNumArgs(h)
	if err != nil || n != 0 {
		return false, err
	}
	declaration, err := g.manager.GetDecl(h)
	if err != nil {
		return false, err
	}
	family, err := g.manager.GetFamilyID(declaration)
	return family == ast.NullFamilyID, err
}

func (g *Goal) DisplayWithDependencies(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	if err := goalDisplayWrite(out, "(goal"); err != nil {
		return err
	}
	for i, form := range g.forms {
		if err := goalDisplayWrite(out, "\n  |-"); err != nil {
			return err
		}
		dependency := ast.ASTDependency{}
		if g.coreEnabled {
			dependency = g.dependencies[i]
		}
		leaves, err := g.manager.DependencyLinearize(dependency, nil)
		if err != nil {
			return err
		}
		for _, leaf := range leaves {
			constant, err := g.dependencyIsUninterpretedConstant(leaf)
			if err != nil {
				return err
			}
			if constant {
				body, err := g.displaySMT2Formula(leaf)
				if err != nil {
					return err
				}
				if err = goalDisplayWrite(out, " "+body); err != nil {
					return err
				}
			} else {
				id, err := g.manager.GetASTID(leaf)
				if err != nil {
					return err
				}
				if err = goalDisplayWrite(out, " #"+strconv.FormatUint(uint64(id), 10)); err != nil {
					return err
				}
			}
		}
		body, err := g.displaySMT2Formula(form)
		if err != nil {
			return err
		}
		if err = goalDisplayWrite(out, "\n  "+body); err != nil {
			return err
		}
	}
	precision, err := goalPrecisionText(g.precision)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, "\n  :precision "+precision+" :depth "+strconv.FormatUint(uint64(g.depth), 10)+")\n")
}

func (g *Goal) DisplayAsAnd(out io.Writer) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	conjunction, err := g.manager.MkFamilyApp(g.manager.BasicFamilyID(), ast.ASTOpAnd, nil, g.forms)
	if err != nil {
		return err
	}
	body, err := g.displaySMT2Formula(conjunction)
	if err != nil {
		return err
	}
	return goalDisplayWrite(out, body+"\n")
}

func (g *Goal) DisplayLowLevel(out io.Writer) (result error) {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	for _, form := range g.forms {
		stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: g.manager.Budget()})
		if err != nil {
			return err
		}
		if err = g.manager.DisplayLowLevel(stream, form, true, true); err != nil {
			return errors.Join(err, stream.Close())
		}
		body, err := stream.String()
		if err != nil {
			return errors.Join(err, stream.Close())
		}
		if err = stream.Close(); err != nil {
			return err
		}
		if err = goalDisplayWrite(out, body+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func (g *Goal) DisplayDIMACS(out io.Writer) error {
	return g.DisplayDIMACSWithNames(out, false)
}

// DisplayDIMACSWithNames is src/ast/display_dimacs.cpp's retained goal
// printer. Unlike the API wrapper, this low-level method assumes CNF.
func (g *Goal) DisplayDIMACSWithNames(out io.Writer, includeNames bool) error {
	if err := g.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrInvalidSlot
	}
	variables := make(map[ast.ASTHandle]uint32)
	atoms := make([]ast.ASTHandle, 1)
	next := uint32(0)
	clauseLiterals := func(form ast.ASTHandle) ([]ast.ASTHandle, error) {
		isOr, err := g.isOp(form, ast.ASTOpOr)
		if err != nil {
			return nil, err
		}
		if !isOr {
			return []ast.ASTHandle{form}, nil
		}
		n, err := g.manager.GetNumArgs(form)
		if err != nil {
			return nil, err
		}
		result := make([]ast.ASTHandle, n)
		for i := uint32(0); i < n; i++ {
			result[i], err = g.manager.GetArg(form, i)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	numericID := func(literal ast.ASTHandle) (uint32, bool, error) {
		isFalse, err := g.isOp(literal, ast.ASTOpFalse)
		if err != nil || isFalse {
			return 0, isFalse, err
		}
		atom, negated, err := g.notArgument(literal)
		if err != nil {
			return 0, false, err
		}
		if !negated {
			atom = literal
		}
		kind, err := g.manager.GetKind(atom)
		if err != nil || kind != ast.ASTApp {
			return 0, false, err
		}
		family, err := g.manager.GetFamilyID(atom)
		if err != nil || family != ast.NullFamilyID {
			return 0, false, err
		}
		arity, err := g.manager.GetNumArgs(atom)
		if err != nil || arity != 0 {
			return 0, false, err
		}
		declaration, err := g.manager.GetDecl(atom)
		if err != nil {
			return 0, false, err
		}
		name, err := g.manager.GetASTName(declaration)
		if err != nil || !name.IsNumerical() {
			return 0, false, err
		}
		id, err := name.Number()
		return id, id > 0, err
	}
	fromNumericDIMACS := true
	for _, form := range g.forms {
		literals, err := clauseLiterals(form)
		if err != nil {
			return err
		}
		for _, literal := range literals {
			_, numeric, err := numericID(literal)
			if err != nil {
				return err
			}
			if !numeric {
				fromNumericDIMACS = false
				break
			}
		}
		if !fromNumericDIMACS {
			break
		}
	}
	for _, form := range g.forms {
		literals, err := clauseLiterals(form)
		if err != nil {
			return err
		}
		for _, literal := range literals {
			isFalse, err := g.isOp(literal, ast.ASTOpFalse)
			if err != nil {
				return err
			}
			if isFalse {
				continue
			}
			atom, negated, err := g.notArgument(literal)
			if err != nil {
				return err
			}
			if !negated {
				atom = literal
			}
			if variables[atom] == 0 {
				next++
				if fromNumericDIMACS {
					id, _, idErr := numericID(literal)
					if idErr != nil {
						return idErr
					}
					variables[atom] = id
				} else {
					variables[atom] = next
					atoms = append(atoms, atom)
				}
			}
		}
	}
	if err := goalDisplayWrite(out, "p cnf "+strconv.FormatUint(uint64(next), 10)+" "+strconv.Itoa(len(g.forms))+"\n"); err != nil {
		return err
	}
	for _, form := range g.forms {
		literals, err := clauseLiterals(form)
		if err != nil {
			return err
		}
		for _, literal := range literals {
			isFalse, err := g.isOp(literal, ast.ASTOpFalse)
			if err != nil {
				return err
			}
			if isFalse {
				continue
			}
			isTrue, err := g.isOp(literal, ast.ASTOpTrue)
			if err != nil {
				return err
			}
			if isTrue {
				if err = goalDisplayWrite(out, "1 -1 "); err != nil {
					return err
				}
				continue
			}
			atom, negated, err := g.notArgument(literal)
			if err != nil {
				return err
			}
			if !negated {
				atom = literal
			} else if err = goalDisplayWrite(out, "-"); err != nil {
				return err
			}
			id := variables[atom]
			if id == 0 {
				return ast.ErrASTDomain
			}
			if err = goalDisplayWrite(out, strconv.FormatUint(uint64(id), 10)+" "); err != nil {
				return err
			}
		}
		if err = goalDisplayWrite(out, "0\n"); err != nil {
			return err
		}
	}
	if includeNames && !fromNumericDIMACS {
		for id := uint32(1); id <= next; id++ {
			declaration, err := g.manager.GetDecl(atoms[id])
			if err != nil {
				return err
			}
			name, err := g.manager.GetASTName(declaration)
			if err != nil {
				return err
			}
			text, err := g.manager.Symbols.String(name)
			if err != nil {
				return err
			}
			if err = goalDisplayWrite(out, "c "+strconv.FormatUint(uint64(id), 10)+" "+text+"\n"); err != nil {
				return err
			}
		}
	}
	return nil
}
