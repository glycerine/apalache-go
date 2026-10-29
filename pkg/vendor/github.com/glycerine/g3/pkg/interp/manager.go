package interp

// Source checkpoint: src/interp/iz3mgr.cpp; src/interp/iz3mgr.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Operator is the interpolation wrapper's family-independent AST vocabulary.
type Operator uint8

const (
	OperatorTrue Operator = iota
	OperatorFalse
	OperatorAnd
	OperatorOr
	OperatorNot
	OperatorIff
	OperatorIte
	OperatorEqual
	OperatorImplies
	OperatorDistinct
	OperatorXor
	OperatorOEq
	OperatorInterp
	OperatorLE
	OperatorGE
	OperatorLT
	OperatorGT
	OperatorPlus
	OperatorSub
	OperatorUnaryMinus
	OperatorTimes
	OperatorDiv
	OperatorIDiv
	OperatorRem
	OperatorMod
	OperatorPower
	OperatorToReal
	OperatorToInt
	OperatorIsInt
	OperatorSelect
	OperatorStore
	OperatorConstArray
	OperatorArrayDefault
	OperatorArrayMap
	OperatorSetUnion
	OperatorSetIntersect
	OperatorSetDifference
	OperatorSetComplement
	OperatorSetSubset
	OperatorAsArray
	OperatorNumeral
	OperatorForall
	OperatorExists
	OperatorVariable
	OperatorUninterpreted
	OperatorOther
)

// Manager is the retained typed convenience layer over ASTManager. It borrows
// the AST manager; callers pin handles that outlive an immediate operation.
type Manager struct {
	self       *Manager
	ast        *ast.ASTManager
	arithmetic ast.FamilyID
	array      ast.FamilyID
	simplifier *astrewriter.TheoryRewriter
	strong     *astrewriter.TheoryRewriter
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	closed     bool
}

func NewManager(m *ast.ASTManager) (*Manager, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	arithmetic, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	array, err := m.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	r := &Manager{ast: m, arithmetic: arithmetic, array: array}
	r.self = r
	return r, nil
}

// NewManagerWithParams equips the wrapper with the two source theory-rewriter
// configurations used by z3_simplify and z3_really_simplify.
func NewManagerWithParams(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (result *Manager, resultErr error) {
	result, err := NewManager(m)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	result.simplifier, err = astrewriter.NewTheoryRewriter(m, context, globals, nil)
	if err != nil {
		return nil, err
	}
	result.context, result.globals = context, globals
	parameters, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, context.Del(&parameters)) }()
	som, sortSums := "som", "sort_sums"
	if err = context.SetBoolName(&parameters, &som, true); err != nil {
		return nil, err
	}
	if err = context.SetBoolName(&parameters, &sortSums, true); err != nil {
		return nil, err
	}
	result.strong, err = astrewriter.NewTheoryRewriter(m, context, globals, &parameters)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *Manager) open() error {
	if m == nil || m.self != m || m.closed || m.ast == nil {
		return ast.ErrASTHandle
	}
	_, err := m.ast.ProofsEnabled()
	return err
}

func (m *Manager) Close() (result error) {
	if m == nil || m.self != m {
		return ast.ErrASTHandle
	}
	if m.closed {
		return nil
	}
	if m.strong != nil {
		result = errors.Join(result, m.strong.Close())
		m.strong = nil
	}
	if m.simplifier != nil {
		result = errors.Join(result, m.simplifier.Close())
		m.simplifier = nil
	}
	m.closed = true
	return result
}

func (m *Manager) ASTManager() (*ast.ASTManager, error) {
	if err := m.open(); err != nil {
		return nil, err
	}
	return m.ast, nil
}

var basicOperators = map[int32]Operator{
	ast.ASTOpTrue: OperatorTrue, ast.ASTOpFalse: OperatorFalse,
	ast.ASTOpEq: OperatorEqual, ast.ASTOpDistinct: OperatorDistinct,
	ast.ASTOpIte: OperatorIte, ast.ASTOpAnd: OperatorAnd,
	ast.ASTOpOr: OperatorOr, ast.ASTOpIff: OperatorIff,
	ast.ASTOpXor: OperatorXor, ast.ASTOpNot: OperatorNot,
	ast.ASTOpImplies: OperatorImplies, ast.ASTOpOEq: OperatorOEq,
	ast.ASTOpInterp: OperatorInterp,
}

var arithmeticOperators = map[int32]Operator{
	2: OperatorLE, 3: OperatorGE, 4: OperatorLT, 5: OperatorGT,
	6: OperatorPlus, 7: OperatorSub, 8: OperatorUnaryMinus, 9: OperatorTimes,
	10: OperatorDiv, 11: OperatorIDiv, 12: OperatorRem, 13: OperatorMod,
	14: OperatorToReal, 15: OperatorToInt, 16: OperatorIsInt, 18: OperatorPower,
}

var arrayOperators = map[int32]Operator{
	ast.ASTArrayOpStore: OperatorStore, ast.ASTArrayOpSelect: OperatorSelect,
	ast.ASTArrayOpConst: OperatorConstArray, ast.ASTArrayOpDefault: OperatorArrayDefault,
	ast.ASTArrayOpMap: OperatorArrayMap, ast.ASTArrayOpSetUnion: OperatorSetUnion,
	ast.ASTArrayOpSetIntersect: OperatorSetIntersect, ast.ASTArrayOpSetDifference: OperatorSetDifference,
	ast.ASTArrayOpSetComplement: OperatorSetComplement, ast.ASTArrayOpSetSubset: OperatorSetSubset,
	ast.ASTArrayOpAsArray: OperatorAsArray,
}

func (m *Manager) Operator(h ast.ASTHandle) (Operator, error) {
	if err := m.open(); err != nil {
		return OperatorOther, err
	}
	kind, err := m.ast.GetKind(h)
	if err != nil {
		return OperatorOther, err
	}
	switch kind {
	case ast.ASTQuantifier:
		forall, e := m.ast.IsForall(h)
		if e != nil {
			return OperatorOther, e
		}
		if forall {
			return OperatorForall, nil
		}
		return OperatorExists, nil
	case ast.ASTVar:
		return OperatorVariable, nil
	case ast.ASTApp:
		family, e := m.ast.GetFamilyID(h)
		if e != nil {
			return OperatorOther, e
		}
		if family == ast.NullFamilyID {
			return OperatorUninterpreted, nil
		}
		declKind, e := m.ast.GetDeclKind(h)
		if e != nil {
			return OperatorOther, e
		}
		if family == m.ast.BasicFamilyID() {
			if op, found := basicOperators[declKind]; found {
				return op, nil
			}
			return OperatorOther, nil
		}
		if family == m.arithmetic {
			if op, found := arithmeticOperators[declKind]; found {
				return op, nil
			}
			if _, _, numeral, e := m.ast.ArithmeticNumeralText(h); e != nil {
				return OperatorOther, e
			} else if numeral {
				return OperatorNumeral, nil
			}
			return OperatorOther, nil
		}
		if family == m.array {
			if op, found := arrayOperators[declKind]; found {
				return op, nil
			}
		}
	}
	return OperatorOther, nil
}

func (m *Manager) NumArgs(h ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	kind, err := m.ast.GetKind(h)
	if err != nil {
		return 0, err
	}
	switch kind {
	case ast.ASTApp:
		return m.ast.GetNumArgs(h)
	case ast.ASTQuantifier:
		return 1, nil
	case ast.ASTVar:
		return 0, nil
	default:
		return 0, ast.ErrASTDomain
	}
}

func (m *Manager) Arg(h ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := m.ast.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind == ast.ASTQuantifier {
		if index != 0 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return m.ast.GetQuantifierBody(h)
	}
	return m.ast.GetArg(h, index)
}

func (m *Manager) Args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := m.NumArgs(h)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := range result {
		result[i], err = m.Arg(h, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *Manager) Symbol(h ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if kind, err := m.ast.GetKind(h); err != nil {
		return ast.ASTHandle{}, err
	} else if kind != ast.ASTApp {
		return ast.ASTHandle{}, nil
	}
	return m.ast.GetDecl(h)
}

func (m *Manager) Type(h ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetSort(h)
}

func (m *Manager) ASTID(h ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	return m.ast.GetASTID(h)
}

func (m *Manager) StringOfSymbol(declaration ast.ASTHandle) (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}
	name, err := m.ast.GetASTName(declaration)
	if err != nil {
		return "", err
	}
	text, present, err := m.ast.Symbols.BareString(name)
	if err != nil {
		return "", err
	}
	if present {
		return text, nil
	}
	return m.ast.Symbols.String(name)
}

func (m *Manager) Numeral(h ast.ASTHandle) (*big.Rat, bool, error) {
	if err := m.open(); err != nil {
		return nil, false, err
	}
	text, _, matched, err := m.ast.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return nil, matched, err
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, NewError("invalid arithmetic numeral text")
	}
	return value, true, nil
}

func (m *Manager) StringOfNumeral(h ast.ASTHandle) (string, error) {
	value, matched, err := m.Numeral(h)
	if err != nil {
		return "", err
	}
	if !matched {
		return "", ast.ErrASTDomain
	}
	return value.RatString(), nil
}

func (m *Manager) Coefficient(h ast.ASTHandle) (*big.Rat, error) {
	op, err := m.Operator(h)
	if err != nil {
		return nil, err
	}
	if op == OperatorTimes {
		first, e := m.Arg(h, 0)
		if e != nil {
			return nil, e
		}
		if value, matched, e := m.Numeral(first); e != nil {
			return nil, e
		} else if matched {
			return value, nil
		}
	}
	return new(big.Rat).SetInt64(1), nil
}

func (m *Manager) LinearVariable(h ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := m.Operator(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if op == OperatorTimes {
		first, e := m.Arg(h, 0)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if _, matched, e := m.Numeral(first); e != nil {
			return ast.ASTHandle{}, e
		} else if matched {
			return m.Arg(h, 1)
		}
	}
	return h, nil
}

func (m *Manager) QuantifierNumBound(h ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	return m.ast.GetNumDeclarations(h)
}

func (m *Manager) QuantifierBoundName(h ast.ASTHandle, index uint32) (string, error) {
	if err := m.open(); err != nil {
		return "", err
	}
	name, err := m.ast.GetDeclarationName(h, index)
	if err != nil {
		return "", err
	}
	text, present, err := m.ast.Symbols.BareString(name)
	if err != nil {
		return "", err
	}
	if present {
		return text, nil
	}
	return m.ast.Symbols.String(name)
}

func (m *Manager) QuantifierBoundType(h ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetDeclarationSort(h, index)
}

func (m *Manager) QuantifierBody(h ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetQuantifierBody(h)
}

func (m *Manager) VariableIndex(h ast.ASTHandle) (uint32, error) {
	if err := m.open(); err != nil {
		return 0, err
	}
	return m.ast.GetVarIndex(h)
}

func (m *Manager) IsBoolType(sort ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	boolean, err := m.ast.MkBoolSort()
	return sort == boolean, err
}

func (m *Manager) IsArrayType(sort ast.ASTHandle) (bool, error) {
	if err := m.open(); err != nil {
		return false, err
	}
	family, err := m.ast.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	kind, err := m.ast.GetDeclKind(sort)
	return err == nil && family == m.array && kind == 0, err
}

func (m *Manager) RangeType(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.GetRange(declaration)
}

func (m *Manager) MakeVariable(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := m.ast.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkConst(symbol, sort)
}

func (m *Manager) MakeDeclaration(declaration ast.ASTHandle, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkApp(declaration, args)
}

func (m *Manager) MakeQuantifier(op Operator, bound []ast.ASTHandle, body ast.ASTHandle) (result ast.ASTHandle, resultErr error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if op != OperatorForall && op != OperatorExists {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(bound) == 0 {
		return body, nil
	}
	// Native make_quant receives an owning ast wrapper for body.
	// expr_abstract requires that input root to remain referenced.
	input, err := m.ast.NewRef(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&input)) }()
	out, err := m.ast.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.ast.DelRef(&out)) }()
	abstractor, err := ast.NewExprAbstractor(m.ast)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, abstractor.Close()) }()
	if err = abstractor.Abstract(0, bound, body, out); err != nil {
		return ast.ASTHandle{}, err
	}
	abstracted, err := m.ast.RefGet(out)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sorts := make([]ast.ASTHandle, len(bound))
	names := make([]usymbol.Symbol, len(bound))
	for i, value := range bound {
		sorts[i], err = m.ast.GetSort(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		declaration, e := m.ast.GetDecl(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		names[i], e = m.ast.GetASTName(declaration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
	}
	return m.ast.MkQuantifier(ast.ASTQuantifierSpec{Forall: op == OperatorForall, Sorts: sorts, Names: names, Body: abstracted})
}

func (m *Manager) Clone(h ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if len(args) == 0 {
		return h, nil
	}
	kind, err := m.ast.GetKind(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch kind {
	case ast.ASTApp:
		count, e := m.ast.GetNumArgs(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		if int(count) != len(args) {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		declaration, e := m.ast.GetDecl(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return m.ast.MkApp(declaration, args)
	case ast.ASTQuantifier:
		if len(args) != 1 {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		return m.ast.UpdateQuantifierBody(h, args[0])
	default:
		return h, nil
	}
}

func (m *Manager) MakeIntText(text string) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.ArithmeticIntegerNumeralText(text)
}

func (m *Manager) MakeRealText(text string) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.ArithmeticRealNumeralText(text)
}

func (m *Manager) MakeInt(value *big.Rat) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.ArithmeticNumeralRat(value, true)
}

func (m *Manager) MakeReal(value *big.Rat) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.ArithmeticNumeralRat(value, false)
}

func (m *Manager) FreshConstant(prefix string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkFreshConst(prefix, sort)
}

func (m *Manager) BoolType() (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkBoolSort()
}

func (m *Manager) IntType() (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkFamilySort(m.arithmetic, 1, nil)
}

func (m *Manager) RealType() (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkFamilySort(m.arithmetic, 0, nil)
}

func (m *Manager) ArrayType(domain, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkArraySort(ast.SliceArguments([]ast.ASTHandle{domain}), rangeSort)
}

func (m *Manager) Function(name string, domain []ast.ASTHandle, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := m.ast.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.ast.MkFuncDecl(symbol, domain, rangeSort, ast.ASTFuncDeclInfo{})
}

func (m *Manager) familyOperator(op Operator) (ast.FamilyID, int32, bool) {
	for kind, candidate := range basicOperators {
		if candidate == op {
			return m.ast.BasicFamilyID(), kind, true
		}
	}
	for kind, candidate := range arithmeticOperators {
		if candidate == op {
			return m.arithmetic, kind, true
		}
	}
	for kind, candidate := range arrayOperators {
		if candidate == op {
			return m.array, kind, true
		}
	}
	return ast.NullFamilyID, 0, false
}

func (m *Manager) Make(op Operator, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	family, kind, found := m.familyOperator(op)
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return m.ast.MkFamilyApp(family, kind, nil, args)
}

func (m *Manager) IsTrue(h ast.ASTHandle) (bool, error) {
	op, err := m.Operator(h)
	return op == OperatorTrue, err
}
func (m *Manager) IsFalse(h ast.ASTHandle) (bool, error) {
	op, err := m.Operator(h)
	return op == OperatorFalse, err
}
func (m *Manager) IsIff(h ast.ASTHandle) (bool, error) {
	op, err := m.Operator(h)
	return op == OperatorIff, err
}
func (m *Manager) IsOr(h ast.ASTHandle) (bool, error) {
	op, err := m.Operator(h)
	return op == OperatorOr, err
}
func (m *Manager) IsNot(h ast.ASTHandle) (bool, error) {
	op, err := m.Operator(h)
	return op == OperatorNot, err
}

func (m *Manager) MkNot(x ast.ASTHandle) (ast.ASTHandle, error) {
	op, err := m.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch op {
	case OperatorTrue:
		return m.Make(OperatorFalse)
	case OperatorFalse:
		return m.Make(OperatorTrue)
	case OperatorNot:
		return m.Arg(x, 0)
	default:
		return m.Make(OperatorNot, x)
	}
}

func (m *Manager) MkAnd(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	ox, err := m.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oy, err := m.Operator(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch {
	case ox == OperatorTrue:
		return y, nil
	case oy == OperatorTrue:
		return x, nil
	case ox == OperatorFalse:
		return x, nil
	case oy == OperatorFalse, x == y:
		return y, nil
	default:
		return m.Make(OperatorAnd, x, y)
	}
}

func (m *Manager) MkOr(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	ox, err := m.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oy, err := m.Operator(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch {
	case ox == OperatorFalse:
		return y, nil
	case oy == OperatorFalse:
		return x, nil
	case ox == OperatorTrue:
		return x, nil
	case oy == OperatorTrue, x == y:
		return y, nil
	default:
		return m.Make(OperatorOr, x, y)
	}
}

func (m *Manager) MkImplies(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	ox, err := m.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oy, err := m.Operator(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch {
	case ox == OperatorTrue:
		return y, nil
	case oy == OperatorFalse:
		return m.MkNot(x)
	case ox == OperatorFalse, x == y:
		return m.Make(OperatorTrue)
	case oy == OperatorTrue:
		return y, nil
	default:
		return m.Make(OperatorImplies, x, y)
	}
}

func (m *Manager) MkAndSlice(values []ast.ASTHandle) (ast.ASTHandle, error) {
	filtered := make([]ast.ASTHandle, 0, len(values))
	for _, value := range values {
		op, err := m.Operator(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if op == OperatorFalse {
			return m.Make(OperatorFalse)
		}
		if op != OperatorTrue {
			filtered = append(filtered, value)
		}
	}
	switch len(filtered) {
	case 0:
		return m.Make(OperatorTrue)
	case 1:
		return filtered[0], nil
	default:
		return m.Make(OperatorAnd, filtered...)
	}
}

func (m *Manager) MkOrSlice(values []ast.ASTHandle) (ast.ASTHandle, error) {
	result, err := m.Make(OperatorFalse)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for _, value := range values {
		result, err = m.MkOr(result, value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

func (m *Manager) MkEqual(x, y ast.ASTHandle) (ast.ASTHandle, error) {
	if x == y {
		return m.Make(OperatorTrue)
	}
	ox, err := m.Operator(x)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oy, err := m.Operator(y)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	switch {
	case ox == OperatorTrue:
		return y, nil
	case oy == OperatorTrue:
		return x, nil
	case ox == OperatorFalse:
		return m.MkNot(y)
	case oy == OperatorFalse:
		return m.MkNot(x)
	default:
		return m.Make(OperatorEqual, x, y)
	}
}

func (m *Manager) MkIte(condition, thenValue, elseValue ast.ASTHandle) (ast.ASTHandle, error) {
	oc, err := m.Operator(condition)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if oc == OperatorTrue {
		return thenValue, nil
	}
	if oc == OperatorFalse {
		return elseValue, nil
	}
	if thenValue == elseValue {
		return thenValue, nil
	}
	ot, err := m.Operator(thenValue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	oe, err := m.Operator(elseValue)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if ot == OperatorTrue && oe == OperatorFalse {
		return condition, nil
	}
	if oe == OperatorTrue && ot == OperatorFalse {
		return m.MkNot(condition)
	}
	return m.Make(OperatorIte, condition, thenValue, elseValue)
}
