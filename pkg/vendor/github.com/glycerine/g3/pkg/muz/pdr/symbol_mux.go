package pdr

// Source: src/muz/pdr/pdr_sym_mux.h and the retained tuple, conversion,
// classification, collection and filtering operations in pdr_sym_mux.cpp.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
)

var ErrSymbolMuxOwner = errors.New("muz/pdr: invalid or closed symbol multiplexer")

type SymbolMux struct {
	manager       *ast.ASTManager
	suffixes      []string
	usedSuffixes  map[string]bool
	nextSuffix    uint32
	primaryTuples map[ast.ASTHandle][]ast.ASTHandle
	symbolIndex   map[ast.ASTHandle]uint32
	symbolPrimary map[ast.ASTHandle]ast.ASTHandle
	prefixPrimary map[ast.ASTHandle]ast.ASTHandle
	primaryPrefix map[ast.ASTHandle]ast.ASTHandle
	primary       []ast.ASTHandle
	nonModel      map[ast.ASTHandle]bool
	references    []ast.ASTRef
	closed        bool
}

func NewSymbolMux(manager *ast.ASTManager) (*SymbolMux, error) {
	if manager == nil {
		return nil, ErrSymbolMuxOwner
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &SymbolMux{
		manager:       manager,
		suffixes:      []string{"_n"},
		usedSuffixes:  map[string]bool{"_n": true},
		primaryTuples: make(map[ast.ASTHandle][]ast.ASTHandle),
		symbolIndex:   make(map[ast.ASTHandle]uint32),
		symbolPrimary: make(map[ast.ASTHandle]ast.ASTHandle),
		prefixPrimary: make(map[ast.ASTHandle]ast.ASTHandle),
		primaryPrefix: make(map[ast.ASTHandle]ast.ASTHandle),
		nonModel:      make(map[ast.ASTHandle]bool),
	}, nil
}

func (m *SymbolMux) open() error {
	if m == nil || m.closed || m.manager == nil {
		return ErrSymbolMuxOwner
	}
	_, err := m.manager.ProofsEnabled()
	return err
}

func (m *SymbolMux) suffix(index uint32) string {
	for uint32(len(m.suffixes)) <= index {
		for {
			candidate := "_" + strconv.FormatUint(uint64(m.nextSuffix), 10)
			m.nextSuffix++
			if !m.usedSuffixes[candidate] {
				m.usedSuffixes[candidate] = true
				m.suffixes = append(m.suffixes, candidate)
				break
			}
		}
	}
	return m.suffixes[index]
}

func (m *SymbolMux) declarationName(declaration ast.ASTHandle) (string, error) {
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

func (m *SymbolMux) declarationSignature(declaration ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	arity, err := m.manager.GetArity(declaration)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	domain := make([]ast.ASTHandle, arity)
	for index := range domain {
		domain[index], err = m.manager.GetDomain(declaration, uint32(index))
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
	}
	rangeSort, err := m.manager.GetRange(declaration)
	return domain, rangeSort, err
}

func sameSignature(manager *ast.ASTManager, declaration ast.ASTHandle, domain []ast.ASTHandle, rangeSort ast.ASTHandle) error {
	arity, err := manager.GetArity(declaration)
	if err != nil || arity != uint32(len(domain)) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	actualRange, err := manager.GetRange(declaration)
	if err != nil || actualRange != rangeSort {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	for index, expected := range domain {
		actual, x := manager.GetDomain(declaration, uint32(index))
		if x != nil || actual != expected {
			if x != nil {
				return x
			}
			return ast.ErrASTDomain
		}
	}
	return nil
}

// CreateTuple creates or completes one family of versioned declarations.
// Existing non-null entries are retained; missing entries are named from the
// prefix. The returned handles remain borrowed from the multiplexer.
func (m *SymbolMux) CreateTuple(prefix ast.ASTHandle, domain []ast.ASTHandle, rangeSort ast.ASTHandle, tupleLength uint32, existing []ast.ASTHandle) (_ []ast.ASTHandle, resultErr error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	if tupleLength == 0 || uint32(len(existing)) > tupleLength {
		return nil, ast.ErrASTDomain
	}
	if _, found := m.prefixPrimary[prefix]; found {
		return nil, ast.ErrASTDomain
	}
	if err := sameSignature(m.manager, prefix, domain, rangeSort); err != nil {
		return nil, err
	}
	name, err := m.declarationName(prefix)
	if err != nil {
		return nil, err
	}
	tuple := make([]ast.ASTHandle, tupleLength)
	copy(tuple, existing)
	refs := make([]ast.ASTRef, 0, tupleLength+1)
	committed := false
	defer func() {
		if committed {
			return
		}
		for index := len(refs) - 1; index >= 0; index-- {
			resultErr = errors.Join(resultErr, m.manager.DelRef(&refs[index]))
		}
	}()
	for index := range tuple {
		if tuple[index].IsNull() {
			symbol, x := m.manager.Symbols.Intern(name + m.suffix(uint32(index)))
			if x != nil {
				return nil, x
			}
			tuple[index], x = m.manager.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
			if x != nil {
				return nil, x
			}
		} else if err = sameSignature(m.manager, tuple[index], domain, rangeSort); err != nil {
			return nil, err
		}
		if _, duplicate := m.symbolIndex[tuple[index]]; duplicate {
			return nil, ast.ErrASTDomain
		}
		ref, x := m.manager.NewRef(tuple[index])
		if x != nil {
			return nil, x
		}
		refs = append(refs, ref)
	}
	prefixRef, err := m.manager.NewRef(prefix)
	if err != nil {
		return nil, err
	}
	refs = append(refs, prefixRef)
	primary := tuple[0]
	for index, symbol := range tuple {
		m.symbolIndex[symbol] = uint32(index)
		m.symbolPrimary[symbol] = primary
	}
	m.primaryTuples[primary] = tuple
	m.prefixPrimary[prefix] = primary
	m.primaryPrefix[primary] = prefix
	m.primary = append(m.primary, primary)
	m.references = append(m.references, refs...)
	committed = true
	return append([]ast.ASTHandle(nil), tuple...), nil
}

func (m *SymbolMux) ensureTupleSize(primary ast.ASTHandle, size uint32) (resultErr error) {
	tuple, found := m.primaryTuples[primary]
	if !found || len(tuple) == 0 || tuple[0] != primary {
		return ast.ErrASTDomain
	}
	if size <= uint32(len(tuple)) {
		return nil
	}
	prefix := m.primaryPrefix[primary]
	domain, rangeSort, err := m.declarationSignature(prefix)
	if err != nil {
		return err
	}
	name, err := m.declarationName(prefix)
	if err != nil {
		return err
	}
	added := make([]ast.ASTRef, 0, int(size)-len(tuple))
	committed := false
	defer func() {
		if !committed {
			for index := len(added) - 1; index >= 0; index-- {
				resultErr = errors.Join(resultErr, m.manager.DelRef(&added[index]))
			}
		}
	}()
	created := make([]ast.ASTHandle, 0, cap(added))
	for index := uint32(len(tuple)); index < size; index++ {
		symbol, x := m.manager.Symbols.Intern(name + m.suffix(index))
		if x != nil {
			return x
		}
		declaration, x := m.manager.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
		if x != nil {
			return x
		}
		ref, x := m.manager.NewRef(declaration)
		if x != nil {
			return x
		}
		added = append(added, ref)
		created = append(created, declaration)
	}
	for offset, declaration := range created {
		index := uint32(len(tuple) + offset)
		tuple = append(tuple, declaration)
		m.symbolIndex[declaration] = index
		m.symbolPrimary[declaration] = primary
	}
	m.primaryTuples[primary] = tuple
	m.references = append(m.references, added...)
	committed = true
	return nil
}

func (m *SymbolMux) TryIndex(symbol ast.ASTHandle) (uint32, bool, error) {
	if err := m.open(); err != nil {
		return 0, false, err
	}
	index, found := m.symbolIndex[symbol]
	return index, found, nil
}

func (m *SymbolMux) HasIndex(symbol ast.ASTHandle, expected uint32) (bool, error) {
	index, found, err := m.TryIndex(symbol)
	return found && index == expected, err
}

func (m *SymbolMux) Primary(symbol ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	primary, found := m.symbolPrimary[symbol]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return primary, nil
}

func (m *SymbolMux) TryPrimaryByPrefix(prefix ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	primary, found := m.prefixPrimary[prefix]
	return primary, found, nil
}

func (m *SymbolMux) TryByPrefix(prefix ast.ASTHandle, index uint32) (ast.ASTHandle, bool, error) {
	primary, found, err := m.TryPrimaryByPrefix(prefix)
	if err != nil || !found {
		return ast.ASTHandle{}, false, err
	}
	result, err := m.Convert(primary, 0, index)
	return result, err == nil, err
}

func (m *SymbolMux) Convert(symbol ast.ASTHandle, source, target uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	actual, found := m.symbolIndex[symbol]
	if !found || actual != source {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if source == target {
		return symbol, nil
	}
	primary := symbol
	if source != 0 {
		primary = m.symbolPrimary[symbol]
	}
	if target > source {
		if err := m.ensureTupleSize(primary, target+1); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	tuple := m.primaryTuples[primary]
	if target >= uint32(len(tuple)) || tuple[source] != symbol {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return tuple[target], nil
}

func (m *SymbolMux) GetOrCreateByPrefix(prefix ast.ASTHandle, index uint32, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if primary, found := m.prefixPrimary[prefix]; found {
		if err := sameSignature(m.manager, primary, domain, rangeSort); err != nil {
			return ast.ASTHandle{}, err
		}
		return m.Convert(primary, 0, index)
	}
	tuple, err := m.CreateTuple(prefix, domain, rangeSort, index+1, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return tuple[index], nil
}

func (m *SymbolMux) IsMuxed(symbol ast.ASTHandle) (bool, error) {
	_, found, err := m.TryIndex(symbol)
	return found, err
}

// IsMuxedLiteral preserves the source predicate: index is accepted for API
// compatibility but the native implementation tests only whether the literal
// declaration belongs to any multiplexed tuple.
func (m *SymbolMux) IsMuxedLiteral(expression ast.ASTHandle, _ uint32) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	kind, err := m.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	if isNot, child, x := m.manager.IsNot(expression); x != nil {
		return false, x
	} else if isNot {
		expression = child
		if kind, err = m.manager.GetKind(expression); err != nil || kind != ast.ASTApp {
			return false, err
		}
	}
	declaration, err := m.manager.GetDecl(expression)
	if err != nil {
		return false, err
	}
	_, found := m.symbolIndex[declaration]
	return found, nil
}

func (m *SymbolMux) walkApplications(root ast.ASTHandle, visit func(ast.ASTHandle, ast.ASTHandle) (bool, error)) error {
	if err := m.open(); err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		index := len(stack) - 1
		node := stack[index]
		stack = stack[:index]
		if visited[node] {
			continue
		}
		visited[node] = true
		kind, err := m.manager.GetKind(node)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, x := m.manager.GetDecl(node)
			if x != nil {
				return x
			}
			stop, x := visit(node, declaration)
			if x != nil {
				return x
			}
			if stop {
				return nil
			}
			count, x := m.manager.GetNumArgs(node)
			if x != nil {
				return x
			}
			for child := count; child != 0; child-- {
				argument, x := m.manager.GetArg(node, child-1)
				if x != nil {
					return x
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, x := m.manager.GetQuantifierBody(node)
			if x != nil {
				return x
			}
			stack = append(stack, body)
			patterns, x := m.manager.GetNumPatterns(node)
			if x != nil {
				return x
			}
			for child := patterns; child != 0; child-- {
				pattern, x := m.manager.GetPattern(node, child-1)
				if x != nil {
					return x
				}
				stack = append(stack, pattern)
			}
			noPatterns, x := m.manager.GetNumNoPatterns(node)
			if x != nil {
				return x
			}
			for child := noPatterns; child != 0; child-- {
				pattern, x := m.manager.GetNoPattern(node, child-1)
				if x != nil {
					return x
				}
				stack = append(stack, pattern)
			}
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

func (m *SymbolMux) Contains(root ast.ASTHandle, index uint32) (bool, error) {
	found := false
	err := m.walkApplications(root, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		actual, muxed := m.symbolIndex[declaration]
		found = muxed && actual == index
		return found, nil
	})
	return found, err
}

func (m *SymbolMux) IsHomogeneousAt(root ast.ASTHandle, index uint32) (bool, error) {
	homogeneous := true
	err := m.walkApplications(root, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		actual, muxed := m.symbolIndex[declaration]
		homogeneous = !muxed || actual == index
		return !homogeneous, nil
	})
	return homogeneous, err
}

func (m *SymbolMux) IsHomogeneous(root ast.ASTHandle) (bool, error) {
	var first uint32
	found := false
	homogeneous := true
	err := m.walkApplications(root, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		actual, muxed := m.symbolIndex[declaration]
		if !muxed {
			return false, nil
		}
		if !found {
			first, found = actual, true
		} else if first != actual {
			homogeneous = false
		}
		return !homogeneous, nil
	})
	return homogeneous, err
}

func (m *SymbolMux) IsHomogeneousAll(formulas []ast.ASTHandle, index uint32) (bool, error) {
	for _, formula := range formulas {
		homogeneous, err := m.IsHomogeneousAt(formula, index)
		if err != nil || !homogeneous {
			return false, err
		}
	}
	return true, nil
}

func (m *SymbolMux) CollectIndices(root ast.ASTHandle) ([]uint32, error) {
	seen := make(map[uint32]bool)
	maximum := uint32(0)
	err := m.walkApplications(root, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolIndex[declaration]
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
	indices := make([]uint32, 0, len(seen))
	for index := uint32(0); len(indices) != len(seen) && index <= maximum; index++ {
		if seen[index] {
			indices = append(indices, index)
		}
	}
	return indices, nil
}

func (m *SymbolMux) CollectVariables(root ast.ASTHandle) ([][]ast.ASTHandle, error) {
	variables := make([][]ast.ASTHandle, 0)
	err := m.walkApplications(root, func(application, declaration ast.ASTHandle) (bool, error) {
		index, muxed := m.symbolIndex[declaration]
		if !muxed {
			return false, nil
		}
		if index == 0 {
			return false, ast.ErrASTDomain
		}
		index--
		for uint32(len(variables)) <= index {
			variables = append(variables, nil)
		}
		variables[index] = append(variables[index], application)
		return false, nil
	})
	return variables, err
}

type symbolMuxRewriteConfiguration struct {
	rewriter.DefaultConfiguration
	mux         *SymbolMux
	source      uint32
	target      uint32
	shift       int64
	shifting    bool
	homogeneous bool
}

func (c *symbolMuxRewriteConfiguration) GetSubst(term ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := c.mux.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	declaration, err := c.mux.manager.GetDecl(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	index, muxed := c.mux.symbolIndex[declaration]
	if !muxed {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	target := c.target
	if c.shifting {
		value := int64(index) + c.shift
		if value < 0 || value > int64(^uint32(0)) {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		target = uint32(value)
	} else if index != c.source {
		if c.homogeneous {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
		}
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	replacement, err := c.mux.Convert(declaration, index, target)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	count, err := c.mux.manager.GetNumArgs(term)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	arguments := make([]ast.ASTHandle, count)
	for argument := range arguments {
		arguments[argument], err = c.mux.manager.GetArg(term, uint32(argument))
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false, err
		}
	}
	result, err := c.mux.manager.MkApp(replacement, arguments)
	return result, ast.ASTHandle{}, err == nil, err
}

func (m *SymbolMux) rewrite(root ast.ASTHandle, config *symbolMuxRewriteConfiguration) (result ast.ASTHandle, resultErr error) {
	traversal, err := rewriter.NewTraversal(m.manager, config)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, traversal.Close()) }()
	out, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&out)) }()
	if err = traversal.Rewrite(root, out); err != nil {
		return result, err
	}
	result, err = m.manager.RefGet(out)
	if err != nil {
		return result, err
	}
	retained, err := m.manager.NewRef(result)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	m.references = append(m.references, retained)
	return result, nil
}

func (m *SymbolMux) ConvertFormula(root ast.ASTHandle, source, target uint32, homogeneous bool) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if source == target {
		return root, nil
	}
	config := &symbolMuxRewriteConfiguration{mux: m, source: source, target: target, homogeneous: homogeneous}
	return m.rewrite(root, config)
}

func (m *SymbolMux) ShiftFormula(root ast.ASTHandle, distance int32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if distance == 0 {
		return root, nil
	}
	config := &symbolMuxRewriteConfiguration{mux: m, shift: int64(distance), shifting: true}
	return m.rewrite(root, config)
}

func (m *SymbolMux) ConvertFormulas(formulas []ast.ASTHandle, source, target uint32) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, len(formulas))
	for index, formula := range formulas {
		converted, err := m.ConvertFormula(formula, source, target, true)
		if err != nil {
			return nil, err
		}
		result[index] = converted
	}
	return result, nil
}

func (m *SymbolMux) FilterIndex(formulas []ast.ASTHandle, index uint32) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), formulas...)
	for position := 0; position < len(result); {
		contains, err := m.Contains(result[position], index)
		if err != nil {
			return nil, err
		}
		homogeneous, err := m.IsHomogeneousAt(result[position], index)
		if err != nil {
			return nil, err
		}
		if contains && homogeneous {
			position++
		} else {
			result[position] = result[len(result)-1]
			result = result[:len(result)-1]
		}
	}
	return result, nil
}

func (m *SymbolMux) PartitionIndex(formulas []ast.ASTHandle, index uint32) (matching, other []ast.ASTHandle, resultErr error) {
	for _, formula := range formulas {
		contains, err := m.Contains(formula, index)
		if err != nil {
			return nil, nil, err
		}
		homogeneous, err := m.IsHomogeneousAt(formula, index)
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

func (m *SymbolMux) MarkNonModel(symbol ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	primary, found := m.symbolPrimary[symbol]
	if !found {
		return ast.ErrASTDomain
	}
	m.nonModel[primary] = true
	return nil
}

func (m *SymbolMux) IsNonModel(symbol ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	primary, found := m.symbolPrimary[symbol]
	return found && m.nonModel[primary], nil
}

func (m *SymbolMux) HasNonModelSymbol(root ast.ASTHandle) (bool, error) {
	found := false
	err := m.walkApplications(root, func(_ ast.ASTHandle, declaration ast.ASTHandle) (bool, error) {
		primary, muxed := m.symbolPrimary[declaration]
		found = muxed && m.nonModel[primary]
		return found, nil
	})
	return found, err
}

func (m *SymbolMux) FilterNonModel(formulas []ast.ASTHandle) ([]ast.ASTHandle, error) {
	result := append([]ast.ASTHandle(nil), formulas...)
	for position := 0; position < len(result); {
		remove, err := m.HasNonModelSymbol(result[position])
		if err != nil {
			return nil, err
		}
		if !remove {
			position++
		} else {
			result[position] = result[len(result)-1]
			result = result[:len(result)-1]
		}
	}
	return result, nil
}

func (m *SymbolMux) PrimaryPredicates() ([]ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), m.primary...), nil
}

// WriteModel writes constant interpretations grouped by mux index and ordered
// by declaration name, matching the source PDR diagnostic view.
func (m *SymbolMux) WriteModel(out io.Writer, modelCore *model.ModelCore) error {
	if err := m.open(); err != nil {
		return err
	}
	if out == nil || modelCore == nil {
		return ErrSymbolMuxOwner
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
		if value, found := m.symbolIndex[declaration]; found {
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
		arrow := " -> "
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

func (m *SymbolMux) ModelString(modelCore *model.ModelCore) (string, error) {
	var out bytes.Buffer
	if err := m.WriteModel(&out, modelCore); err != nil {
		return "", err
	}
	return out.String(), nil
}

func (m *SymbolMux) Close() (result error) {
	if m == nil || m.manager == nil {
		return ErrSymbolMuxOwner
	}
	if m.closed {
		return nil
	}
	for index := len(m.references) - 1; index >= 0; index-- {
		result = errors.Join(result, m.manager.DelRef(&m.references[index]))
	}
	if result != nil {
		return fmt.Errorf("%w: %v", ErrSymbolMuxOwner, result)
	}
	m.references = nil
	m.primaryTuples = nil
	m.symbolIndex = nil
	m.symbolPrimary = nil
	m.prefixPrimary = nil
	m.primaryPrefix = nil
	m.primary = nil
	m.nonModel = nil
	m.manager = nil
	m.closed = true
	return nil
}
