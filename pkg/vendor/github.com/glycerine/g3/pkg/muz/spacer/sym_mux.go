package spacer

// Source: the state-symbol tuple and formula-conversion paths in
// src/muz/spacer/spacer_sym_mux.h and spacer_sym_mux.cpp.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

var ErrSymMuxOwner = errors.New("muz/spacer: invalid or closed symbol multiplexer")

type SymMux struct {
	manager       *ast.ASTManager
	suffixes      []string
	usedSuffixes  map[string]bool
	nextSuffix    uint32
	primaryToAll  map[ast.ASTHandle][]ast.ASTHandle
	symbolToIndex map[ast.ASTHandle]uint32
	symbolToPrim  map[ast.ASTHandle]ast.ASTHandle
	prefixToPrim  map[ast.ASTHandle]ast.ASTHandle
	primToPrefix  map[ast.ASTHandle]ast.ASTHandle
	primary       []ast.ASTHandle
	nonModel      map[ast.ASTHandle]bool
	references    []ast.ASTRef
	closed        bool
}

func NewSymMux(manager *ast.ASTManager, suffixes []string) (*SymMux, error) {
	if manager == nil {
		return nil, ErrSymMuxOwner
	}
	result := &SymMux{
		manager: manager, suffixes: append([]string(nil), suffixes...), usedSuffixes: make(map[string]bool),
		primaryToAll: make(map[ast.ASTHandle][]ast.ASTHandle), symbolToIndex: make(map[ast.ASTHandle]uint32),
		symbolToPrim: make(map[ast.ASTHandle]ast.ASTHandle), prefixToPrim: make(map[ast.ASTHandle]ast.ASTHandle),
		primToPrefix: make(map[ast.ASTHandle]ast.ASTHandle), nonModel: make(map[ast.ASTHandle]bool),
	}
	for _, suffix := range suffixes {
		result.usedSuffixes[suffix] = true
	}
	return result, nil
}

func (m *SymMux) open() error {
	if m == nil || m.closed || m.manager == nil {
		return ErrSymMuxOwner
	}
	return nil
}

func (m *SymMux) suffix(index uint32) string {
	for uint32(len(m.suffixes)) <= index {
		var value string
		for {
			value = fmt.Sprintf("_%d", m.nextSuffix)
			m.nextSuffix++
			if !m.usedSuffixes[value] {
				break
			}
		}
		m.usedSuffixes[value] = true
		m.suffixes = append(m.suffixes, value)
	}
	return m.suffixes[index]
}

func (m *SymMux) declarationSignature(declaration ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	arity, err := m.manager.GetArity(declaration)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for index := uint32(0); index < arity; index++ {
		domain[index], err = m.manager.GetDomain(declaration, index)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
	}
	rangeSort, err := m.manager.GetRange(declaration)
	return domain, rangeSort, err
}

func (m *SymMux) declarationName(declaration ast.ASTHandle) (string, error) {
	name, err := m.manager.GetASTName(declaration)
	if err != nil {
		return "", err
	}
	if text, present, x := m.manager.Symbols.BareString(name); x != nil {
		return "", x
	} else if present {
		return text, nil
	}
	return m.manager.Symbols.Display(name)
}

func (m *SymMux) createVariant(prefix ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	name, err := m.manager.GetASTName(prefix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	spelling, present, err := m.manager.Symbols.BareString(name)
	if err != nil || !present {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	variantName, err := m.manager.Symbols.Intern(spelling + m.suffix(index))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	domain, rangeSort, err := m.declarationSignature(prefix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.manager.MkFuncDecl(variantName, domain, rangeSort, ast.ASTFuncDeclInfo{})
}

func (m *SymMux) pin(handle ast.ASTHandle) error {
	reference, err := m.manager.NewRef(handle)
	if err == nil {
		m.references = append(m.references, reference)
	}
	return err
}

func (m *SymMux) CreateTuple(prefix ast.ASTHandle, tupleLength uint32) ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if tupleLength == 0 {
		return nil, ast.ErrASTDomain
	}
	if _, found := m.prefixToPrim[prefix]; found {
		return nil, ast.ErrASTDomain
	}
	tuple := make([]ast.ASTHandle, tupleLength)
	for index := range tuple {
		variant, err := m.createVariant(prefix, uint32(index))
		if err != nil {
			return nil, err
		}
		if err = m.pin(variant); err != nil {
			return nil, err
		}
		tuple[index] = variant
		m.symbolToIndex[variant] = uint32(index)
	}
	primary := tuple[0]
	for _, variant := range tuple {
		m.symbolToPrim[variant] = primary
	}
	if err := m.pin(prefix); err != nil {
		return nil, err
	}
	m.primaryToAll[primary] = tuple
	m.prefixToPrim[prefix] = primary
	m.primToPrefix[primary] = prefix
	m.primary = append(m.primary, primary)
	return append([]ast.ASTHandle(nil), tuple...), nil
}

func (m *SymMux) ensureTupleSize(primary ast.ASTHandle, size uint32) error {
	tuple, found := m.primaryToAll[primary]
	if !found || len(tuple) == 0 || tuple[0] != primary {
		return ast.ErrASTDomain
	}
	prefix := m.primToPrefix[primary]
	for uint32(len(tuple)) < size {
		index := uint32(len(tuple))
		variant, err := m.createVariant(prefix, index)
		if err != nil {
			return err
		}
		if err = m.pin(variant); err != nil {
			return err
		}
		tuple = append(tuple, variant)
		m.symbolToIndex[variant] = index
		m.symbolToPrim[variant] = primary
	}
	m.primaryToAll[primary] = tuple
	return nil
}

func (m *SymMux) IsMuxed(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	_, found := m.symbolToIndex[declaration]
	return found, nil
}

func (m *SymMux) IsMuxedLiteral(expression ast.ASTHandle, _ uint32) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	if negated, argument, err := m.manager.IsNot(expression); err != nil {
		return false, err
	} else if negated {
		expression = argument
	}
	kind, err := m.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	declaration, err := m.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	return m.IsMuxed(declaration)
}

func (m *SymMux) Index(declaration ast.ASTHandle) (uint32, bool, error) {
	if err := m.open(); err != nil {
		return 0, false, err
	}
	index, found := m.symbolToIndex[declaration]
	return index, found, nil
}

func (m *SymMux) Primary(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	primary, found := m.symbolToPrim[declaration]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return primary, nil
}

func (m *SymMux) ByPrefix(prefix ast.ASTHandle, index uint32) (ast.ASTHandle, bool, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	primary, found := m.prefixToPrim[prefix]
	if !found {
		return ast.ASTHandle{}, false, nil
	}
	value, err := m.ConvertDeclaration(primary, 0, index)
	return value, err == nil, err
}

func (m *SymMux) ConvertDeclaration(declaration ast.ASTHandle, source, target uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if source == target {
		return declaration, nil
	}
	actual, found := m.symbolToIndex[declaration]
	if !found || actual != source {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	primary := declaration
	if source != 0 {
		primary = m.symbolToPrim[declaration]
	}
	if err := m.ensureTupleSize(primary, target+1); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.primaryToAll[primary][target], nil
}

func (m *SymMux) rewriteFormula(root ast.ASTHandle, source, target uint32, homogeneous bool, shift *int) (ast.ASTHandle, error) {
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var rewrite func(ast.ASTHandle) (ast.ASTHandle, error)
	rewrite = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if value, found := memo[current]; found {
			return value, nil
		}
		kind, err := m.manager.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result := current
		switch kind {
		case ast.ASTApp:
			declaration, err := m.manager.GetDecl(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			newDeclaration := declaration
			if index, muxed := m.symbolToIndex[declaration]; muxed {
				if shift != nil {
					next := int(index) + *shift
					if next < 0 {
						return ast.ASTHandle{}, ast.ErrASTDomain
					}
					newDeclaration, err = m.ConvertDeclaration(declaration, index, uint32(next))
				} else if index == source {
					newDeclaration, err = m.ConvertDeclaration(declaration, source, target)
				} else if homogeneous {
					return ast.ASTHandle{}, ast.ErrASTDomain
				}
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
			count, err := m.manager.GetNumArgs(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			arguments := make([]ast.ASTHandle, count)
			changed := newDeclaration != declaration
			for index := uint32(0); index < count; index++ {
				argument, err := m.manager.GetArg(current, index)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				arguments[index], err = rewrite(argument)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed = changed || arguments[index] != argument
			}
			if changed {
				result, err = m.manager.MkApp(newDeclaration, arguments)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		case ast.ASTQuantifier:
			body, err := m.manager.GetQuantifierBody(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			newBody, err := rewrite(body)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			patternCount, err := m.manager.GetNumPatterns(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			patterns := make([]ast.ASTHandle, patternCount)
			changed := newBody != body
			for index := uint32(0); index < patternCount; index++ {
				old, err := m.manager.GetPattern(current, index)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				patterns[index], err = rewrite(old)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed = changed || patterns[index] != old
			}
			noPatternCount, err := m.manager.GetNumNoPatterns(current)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			noPatterns := make([]ast.ASTHandle, noPatternCount)
			for index := uint32(0); index < noPatternCount; index++ {
				old, err := m.manager.GetNoPattern(current, index)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				noPatterns[index], err = rewrite(old)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				changed = changed || noPatterns[index] != old
			}
			if changed {
				result, err = m.manager.UpdateQuantifierAnnotations(current, patterns, noPatterns, newBody)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		memo[current] = result
		return result, nil
	}
	return rewrite(root)
}

func (m *SymMux) ConvertFormula(formula ast.ASTHandle, source, target uint32, homogeneous bool) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	converted, err := m.rewriteFormula(formula, source, target, homogeneous, nil)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(converted)
}

func (m *SymMux) ShiftFormula(formula ast.ASTHandle, distance int) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	converted, err := m.rewriteFormula(formula, 0, 0, false, &distance)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.manager.NewRef(converted)
}

func (m *SymMux) walkApplications(root ast.ASTHandle, visit func(ast.ASTHandle, ast.ASTHandle) (bool, error)) error {
	if err := m.open(); err != nil {
		return err
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[current] {
			continue
		}
		seen[current] = true
		kind, err := m.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			declaration, err := m.manager.GetDecl(current)
			if err != nil {
				return err
			}
			stop, err := visit(current, declaration)
			if err != nil || stop {
				return err
			}
		}
		if kind == ast.ASTQuantifier {
			body, err := m.manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patternCount, err := m.manager.GetNumPatterns(current)
			if err != nil {
				return err
			}
			for index := uint32(0); index < patternCount; index++ {
				pattern, err := m.manager.GetPattern(current, index)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			noPatternCount, err := m.manager.GetNumNoPatterns(current)
			if err != nil {
				return err
			}
			for index := uint32(0); index < noPatternCount; index++ {
				pattern, err := m.manager.GetNoPattern(current, index)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		}
		if kind == ast.ASTApp {
			count, err := m.manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			for index := uint32(0); index < count; index++ {
				argument, err := m.manager.GetArg(current, index)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		}
	}
	return nil
}

func (m *SymMux) Contains(formula ast.ASTHandle, wanted uint32) (bool, error) {
	found := false
	err := m.walkApplications(formula, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolToIndex[declaration]
		found = muxed && index == wanted
		return found, nil
	})
	return found, err
}

func (m *SymMux) IsHomogeneousAt(formula ast.ASTHandle, wanted uint32) (bool, error) {
	homogeneous := true
	err := m.walkApplications(formula, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolToIndex[declaration]
		homogeneous = !muxed || index == wanted
		return !homogeneous, nil
	})
	return homogeneous, err
}

func (m *SymMux) IsHomogeneous(formula ast.ASTHandle) (bool, error) {
	var first uint32
	found := false
	homogeneous := true
	err := m.walkApplications(formula, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolToIndex[declaration]
		if !muxed {
			return false, nil
		}
		if !found {
			first, found = index, true
		} else if first != index {
			homogeneous = false
		}
		return !homogeneous, nil
	})
	return homogeneous, err
}

func (m *SymMux) IsHomogeneousAll(formulas []ast.ASTHandle, wanted uint32) (bool, error) {
	for _, formula := range formulas {
		homogeneous, err := m.IsHomogeneousAt(formula, wanted)
		if err != nil || !homogeneous {
			return false, err
		}
	}
	return true, nil
}

func (m *SymMux) CollectIndices(formula ast.ASTHandle) ([]uint32, error) {
	seen := make(map[uint32]bool)
	maximum := uint32(0)
	err := m.walkApplications(formula, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolToIndex[declaration]
		if !muxed {
			return false, nil
		}
		if index == 0 {
			return false, ast.ErrASTDomain
		}
		index--
		seen[index] = true
		if index > maximum {
			maximum = index
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]uint32, 0, len(seen))
	for index := uint32(0); len(result) != len(seen) && index <= maximum; index++ {
		if seen[index] {
			result = append(result, index)
		}
	}
	return result, nil
}

func (m *SymMux) CollectVariables(formula ast.ASTHandle) ([][]ast.ASTHandle, error) {
	var result [][]ast.ASTHandle
	err := m.walkApplications(formula, func(application, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolToIndex[declaration]
		if !muxed {
			return false, nil
		}
		if index == 0 {
			return false, ast.ErrASTDomain
		}
		index--
		for uint32(len(result)) <= index {
			result = append(result, nil)
		}
		result[index] = append(result[index], application)
		return false, nil
	})
	return result, err
}

func (m *SymMux) MarkNonModel(declaration ast.ASTHandle) error {
	primary, err := m.Primary(declaration)
	if err == nil {
		m.nonModel[primary] = true
	}
	return err
}

func (m *SymMux) FilterIndex(formulas []ast.ASTHandle, wanted uint32) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), formulas...)
	for index := 0; index < len(result); {
		contains, err := m.Contains(result[index], wanted)
		if err != nil {
			return nil, err
		}
		homogeneous, err := m.IsHomogeneousAt(result[index], wanted)
		if err != nil {
			return nil, err
		}
		if contains && homogeneous {
			index++
		} else {
			result[index] = result[len(result)-1]
			result = result[:len(result)-1]
		}
	}
	return result, nil
}

func (m *SymMux) PartitionIndex(formulas []ast.ASTHandle, wanted uint32) (matching, other []ast.ASTHandle, resultErr error) {
	for _, formula := range formulas {
		contains, err := m.Contains(formula, wanted)
		if err != nil {
			return nil, nil, err
		}
		homogeneous, err := m.IsHomogeneousAt(formula, wanted)
		if err != nil {
			return nil, nil, err
		}
		if contains && homogeneous {
			matching = append(matching, formula)
		} else {
			other = append(other, formula)
		}
	}
	return matching, other, nil
}

func (m *SymMux) IsNonModel(declaration ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	primary, found := m.symbolToPrim[declaration]
	return found && m.nonModel[primary], nil
}

func (m *SymMux) HasNonModelSymbol(formula ast.ASTHandle) (bool, error) {
	found := false
	err := m.walkApplications(formula, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		primary, muxed := m.symbolToPrim[declaration]
		found = muxed && m.nonModel[primary]
		return found, nil
	})
	return found, err
}

func (m *SymMux) FilterNonModel(formulas []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), formulas...)
	for index := 0; index < len(result); {
		remove, err := m.HasNonModelSymbol(result[index])
		if err != nil {
			return nil, err
		}
		if !remove {
			index++
		} else {
			result[index] = result[len(result)-1]
			result = result[:len(result)-1]
		}
	}
	return result, nil
}

func (m *SymMux) PrimaryPredicates() ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), m.primary...), nil
}

func (m *SymMux) WriteModel(out io.Writer, modelCore *model.ModelCore) error {
	if err := m.open(); err != nil {
		return err
	}
	if out == nil || modelCore == nil {
		return ErrSymMuxOwner
	}
	count, err := modelCore.NumConstants()
	if err != nil {
		return err
	}
	type modelRow struct {
		declaration ast.ASTHandle
		name        string
		index       uint32
	}
	rows := make([]modelRow, count)
	for index := range rows {
		declaration, x := modelCore.Constant(uint32(index))
		if x != nil {
			return x
		}
		name, x := m.declarationName(declaration)
		if x != nil {
			return x
		}
		muxIndex := ^uint32(0)
		if value, found := m.symbolToIndex[declaration]; found {
			muxIndex = value
		}
		rows[index] = modelRow{declaration: declaration, name: name, index: muxIndex}
	}
	sort.SliceStable(rows, func(left, right int) bool {
		if rows[left].index != rows[right].index {
			return rows[left].index < rows[right].index
		}
		return rows[left].name < rows[right].name
	})
	printer, err := ast.NewSimpleASTPrinterContext(m.manager, nil, nil)
	if err != nil {
		return err
	}
	for index, row := range rows {
		const arrow = " -> "
		if _, err = io.WriteString(out, row.name+arrow); err != nil {
			return err
		}
		value, x := modelCore.ConstantInterpretation(row.declaration)
		if x != nil {
			return x
		}
		var rendered bytes.Buffer
		if x = printer.DisplayExpr(&rendered, value, 0); x != nil {
			return x
		}
		indent := strings.Repeat(" ", len(row.name)+len(arrow))
		text := strings.ReplaceAll(rendered.String(), "\n", "\n"+indent)
		if _, err = io.WriteString(out, text+"\n"); err != nil {
			return err
		}
		if index+1 != len(rows) && row.index != rows[index+1].index {
			if _, err = io.WriteString(out, "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *SymMux) ModelString(modelCore *model.ModelCore) (string, error) {
	var out bytes.Buffer
	if err := m.WriteModel(&out, modelCore); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (m *SymMux) Close() (result error) {
	if m == nil || m.manager == nil {
		return ErrSymMuxOwner
	}
	if m.closed {
		return nil
	}
	for index := len(m.references) - 1; index >= 0; index-- {
		result = errors.Join(result, m.manager.DelRef(&m.references[index]))
	}
	if result == nil {
		m.references = nil
		m.primaryToAll = nil
		m.symbolToIndex = nil
		m.symbolToPrim = nil
		m.prefixToPrim = nil
		m.primToPrefix = nil
		m.nonModel = nil
		m.manager = nil
		m.closed = true
	}
	return result
}
