package smtcore

// Source: src/smt/theory_array_full.cpp instantiate_select_const_axiom,
// instantiate_select_map_axiom, instantiate_select_as_array_axiom,
// try_assign_eq, and pop_scope_eh.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type smtArrayExpressionPair struct {
	left, right ast.ASTHandle
}

// tryAssignArrayEquality caches ordered expression pairs, independently of
// the canonical ordering used to construct the equality's Boolean atom.
func (t *SMTArrayTheory) tryAssignArrayEquality(left, right ast.ASTHandle) (bool, error) {
	if err := t.emitFullArrayTrace("array", "try_assign_eq", 787, nil, func(out io.Writer) error {
		if err := displayArrayTraceExpression(t.state.manager, out, left); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n==\n"); err != nil {
			return err
		}
		if err := displayArrayTraceExpression(t.state.manager, out, right); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	}); err != nil {
		return false, err
	}
	pair := smtArrayExpressionPair{left: left, right: right}
	if t.equalityAxioms[pair] {
		return false, nil
	}
	t.equalityAxioms[pair] = true
	literal, err := t.MkEqualityLiteral(left, right, true)
	if err != nil {
		return false, err
	}
	if err = t.markArrayEqualityRelevant(literal); err != nil {
		return false, err
	}
	if err = t.assertArrayAxiom([]SMTLiteral{literal}); err != nil {
		return false, err
	}
	return true, nil
}

func (t *SMTArrayTheory) instantiateConstructorRead(observer, constructor ast.ASTHandle) (added bool, err error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	op, array, err := t.arrayOperation(constructor)
	if err != nil || !array {
		return false, err
	}
	if op != ast.ASTArrayOpConst && op != ast.ASTArrayOpMap && op != ast.ASTArrayOpAsArray {
		return false, nil
	}
	read := s.context.enodes[observer]
	if read == nil || len(read.arguments) < 2 {
		return false, ast.ErrASTDomain
	}
	readOp, readArray, err := t.arrayOperation(observer)
	if err != nil || !readArray || readOp != ast.ASTArrayOpSelect {
		if err != nil {
			return false, err
		}
		return false, ast.ErrASTDomain
	}
	constructorNode := s.context.enodes[constructor]
	if constructorNode == nil {
		return false, ast.ErrASTDomain
	}
	constructorArgs, err := s.manager.GetNumArgs(constructor)
	if err != nil {
		return false, err
	}
	if op == ast.ASTArrayOpConst && constructorArgs != 1 || op == ast.ASTArrayOpMap && constructorArgs == 0 || op == ast.ASTArrayOpAsArray && constructorArgs != 0 {
		return false, ast.ErrASTDomain
	}
	if op == ast.ASTArrayOpMap {
		if err = t.emitFullArrayTrace("array_map_bug", "instantiate_select_map_axiom", 481, nil, func(out io.Writer) error {
			readID, err := read.OwnerID()
			if err != nil {
				return err
			}
			mapID, err := constructorNode.OwnerID()
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(out, "invoked instantiate_select_map_axiom\n%d %d\n", readID, mapID); err != nil {
				return err
			}
			if err = displayArrayTraceExpression(s.manager, out, observer); err != nil {
				return err
			}
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
			if err = displayArrayTraceExpression(s.manager, out, constructor); err != nil {
				return err
			}
			_, err = io.WriteString(out, "\n")
			return err
		}); err != nil {
			return false, err
		}
	}
	if !t.insertArrayReadFingerprint(constructor, read.arguments[1:]) {
		return false, nil
	}
	if op == ast.ASTArrayOpMap {
		if err = t.emitFullArrayTrace("array_map_bug", "instantiate_select_map_axiom", 489, nil, func(out io.Writer) error {
			_, err := io.WriteString(out, "new axiom\n")
			return err
		}); err != nil {
			return false, err
		}
		if err = t.emitFullArrayTrace("array", "instantiate_select_map_axiom", 492, nil, func(out io.Writer) error {
			if err := displayArrayTraceExpression(s.manager, out, constructor); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, observer); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return false, err
		}
	}
	arguments := []ast.ASTHandle{constructor}
	for _, index := range read.arguments[1:] {
		arguments = append(arguments, index.owner)
	}
	selected, err := t.makeArraySelect(arguments)
	if err != nil {
		return false, err
	}
	var value ast.ASTHandle
	switch op {
	case ast.ASTArrayOpConst:
		t.statistics.selectConst++
		value, err = s.manager.GetArg(constructor, 0)
	case ast.ASTArrayOpAsArray:
		t.statistics.selectAsArray++
		var function ast.ASTHandle
		function, err = s.manager.AsArrayFunction(constructor)
		if err == nil {
			value, err = s.manager.MkApp(function, arguments[1:])
		}
	case ast.ASTArrayOpMap:
		t.statistics.mapAxiom++
		value, err = t.mappedReadValue(constructor, arguments[1:])
	}
	if err != nil {
		return false, err
	}
	// The full SMT context owns a rewriter even when assertion preprocessing
	// is disabled. Standalone internalizer fixtures can attach one explicitly.
	if op == ast.ASTArrayOpMap && s.theoryRewriter != nil {
		ref, refErr := s.manager.NewRef(value)
		if refErr != nil {
			return false, refErr
		}
		defer func() { err = errors.Join(err, s.manager.DelRef(&ref)) }()
		if err = s.theoryRewriter.Rewrite(value, ref); err != nil {
			return false, err
		}
		value, err = s.manager.RefGet(ref)
		if err != nil {
			return false, err
		}
	}
	switch op {
	case ast.ASTArrayOpMap:
		if err = t.emitFullArrayTrace("array_map_bug", "instantiate_select_map_axiom", 524, nil, func(out io.Writer) error {
			if _, err := io.WriteString(out, "select-map axiom\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, selected); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\n=\n"); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, value); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return false, err
		}
	case ast.ASTArrayOpConst, ast.ASTArrayOpAsArray:
		line, kind, function := 642, "const", "instantiate_select_const_axiom"
		if op == ast.ASTArrayOpAsArray {
			line, kind, function = 678, "as-array", "instantiate_select_as_array_axiom"
		}
		if err = t.emitFullArrayTrace("array", function, line, nil, func(out io.Writer) error {
			if _, err := fmt.Fprintf(out, "new select-%s axiom...\n%s: ", kind, kind); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, constructor); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\nselect: "); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, observer); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\nselected: "); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, selected); err != nil {
				return err
			}
			if _, err := io.WriteString(out, "\nvalue: "); err != nil {
				return err
			}
			if err := displayArrayTraceExpression(s.manager, out, value); err != nil {
				return err
			}
			selectedID, err := s.manager.GetASTID(selected)
			if err != nil {
				return err
			}
			valueID, err := s.manager.GetASTID(value)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(out, "\n#%d = #%d\n", selectedID, valueID)
			return err
		}); err != nil {
			return false, err
		}
	}
	if err = s.Internalize(selected, false); err != nil {
		return false, err
	}
	if err = s.Internalize(value, false); err != nil {
		return false, err
	}
	return t.tryAssignArrayEquality(selected, value)
}

func (t *SMTArrayTheory) mappedReadValue(constructor ast.ASTHandle, indices []ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	declaration, err := m.GetDecl(constructor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	function, err := m.ArrayMapFunction(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := m.GetNumArgs(constructor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arity, err := m.GetArity(function)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if count == 0 || count != arity {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	values := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		input, err := m.GetArg(constructor, i)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		arguments := append([]ast.ASTHandle{input}, indices...)
		value, err := t.makeArraySelect(arguments)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		values = append(values, value)
	}
	return m.MkApp(function, values)
}
