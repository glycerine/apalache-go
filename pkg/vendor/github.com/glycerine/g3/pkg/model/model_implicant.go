package model

// Source: src/model/model_implicant.cpp; src/model/model_implicant.h.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

type implicantValue uint8

const (
	implicantUnknown implicantValue = iota
	implicantX
	implicantFalse
	implicantTrue
	implicantConcrete
)

// ModelImplicant extracts a sufficient model cone or a satisfying literal
// subset. It owns only its temporary evaluator and AST references; the model,
// manager, formulas, and process services are borrowed.
type ModelImplicant struct {
	manager           *ast.ASTManager
	services          ModelEvaluatorConfig
	model             *Model
	evaluator         *ModelEvaluator
	states            map[ast.ASTHandle]implicantValue
	numbers           map[ast.ASTHandle]*big.Rat
	values            map[ast.ASTHandle]ast.ASTHandle
	visited           map[ast.ASTHandle]bool
	refs              []ast.ASTRef
	pendingArrays     []*arrayExpansion
	pendingEvaluators []*ModelEvaluator
	pendingResults    [][]ast.ASTRef
	drain             int
	closed            bool
}

func NewModelImplicant(m *ast.ASTManager, services ModelEvaluatorConfig) (*ModelImplicant, error) {
	if m == nil || services.Context == nil || services.Globals == nil || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &ModelImplicant{manager: m, services: services}, nil
}

func (i *ModelImplicant) open() error {
	if i == nil || i.manager == nil {
		return ast.ErrASTHandle
	}
	if i.closed {
		return arena.ErrClosed
	}
	_, err := i.manager.ProofsEnabled()
	return err
}

func (i *ModelImplicant) pin(h ast.ASTHandle) error {
	r, err := i.manager.NewRef(h)
	if err != nil {
		return err
	}
	i.refs = append(i.refs, r)
	return nil
}

func (i *ModelImplicant) reset() error {
	for len(i.pendingArrays) != 0 {
		if err := i.pendingArrays[0].close(i.manager); err != nil {
			return err
		}
		i.pendingArrays[0] = nil
		i.pendingArrays = i.pendingArrays[1:]
	}
	i.pendingArrays = nil
	for len(i.pendingEvaluators) != 0 {
		if err := i.pendingEvaluators[0].Close(); err != nil {
			return err
		}
		i.pendingEvaluators[0] = nil
		i.pendingEvaluators = i.pendingEvaluators[1:]
	}
	i.pendingEvaluators = nil
	for i.drain < len(i.refs) {
		// DelRef can consume and clear its slot before returning an error.
		// On retry, advance past that completed release without replaying it.
		if i.refs[i.drain] != (ast.ASTRef{}) {
			if err := i.manager.DelRef(&i.refs[i.drain]); err != nil {
				return err
			}
		}
		i.drain++
	}
	i.refs = nil
	i.drain = 0
	if i.evaluator != nil {
		if err := i.evaluator.Close(); err != nil {
			return err
		}
		i.evaluator = nil
	}
	for len(i.pendingResults) != 0 {
		if err := closeImplicantResults(i.manager, &i.pendingResults[0]); err != nil {
			return err
		}
		i.pendingResults[0] = nil
		i.pendingResults = i.pendingResults[1:]
	}
	i.pendingResults = nil
	i.model = nil
	i.states = nil
	i.numbers = nil
	i.values = nil
	i.visited = nil
	return nil
}

func (i *ModelImplicant) Close() error {
	if i == nil {
		return ast.ErrASTHandle
	}
	if i.closed {
		return nil
	}
	if err := i.reset(); err != nil {
		return err
	}
	i.closed = true
	return nil
}

func (i *ModelImplicant) setupModel(md *Model) error {
	if err := i.open(); err != nil {
		return err
	}
	if i.model != nil || i.evaluator != nil || len(i.refs) != 0 || len(i.pendingArrays) != 0 || len(i.pendingEvaluators) != 0 || len(i.pendingResults) != 0 {
		return ast.ErrASTPending
	}
	if err := md.open(); err != nil {
		return err
	}
	if md.CoreField.manager != i.manager {
		return ast.ErrASTHandle
	}
	ev, err := NewModelEvaluator(md.Core(), i.services)
	i.model = md
	i.evaluator = ev
	if err != nil {
		return err
	}
	i.states = make(map[ast.ASTHandle]implicantValue)
	i.numbers = make(map[ast.ASTHandle]*big.Rat)
	i.values = make(map[ast.ASTHandle]ast.ASTHandle)
	i.visited = make(map[ast.ASTHandle]bool)
	count, err := md.Core().NumConstants()
	if err != nil {
		return err
	}
	for n := uint32(0); n < count; n++ {
		decl, err := md.Core().Constant(n)
		if err != nil {
			return err
		}
		value, err := md.Core().ConstantInterpretation(decl)
		if err != nil {
			return err
		}
		// Reserve ownership before creating a potentially new constant. If
		// reference-arena growth fails after MkApp, the unpublished node
		// would otherwise have no owner for reset to release.
		slot, err := i.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		i.refs = append(i.refs, slot)
		constant, err := i.manager.MkApp(decl, nil)
		if err != nil {
			return err
		}
		if err = i.manager.SetRef(slot, constant); err != nil {
			return err
		}
		if err = i.assignValue(constant, value); err != nil {
			return err
		}
	}
	return nil
}

func (i *ModelImplicant) state(h ast.ASTHandle) implicantValue { return i.states[h] }
func (i *ModelImplicant) isUnknown(h ast.ASTHandle) bool       { return i.state(h) == implicantUnknown }
func (i *ModelImplicant) isX(h ast.ASTHandle) bool             { return i.state(h) == implicantX }
func (i *ModelImplicant) isFalse(h ast.ASTHandle) bool         { return i.state(h) == implicantFalse }
func (i *ModelImplicant) isTrue(h ast.ASTHandle) bool          { return i.state(h) == implicantTrue }

func (i *ModelImplicant) setState(h ast.ASTHandle, state implicantValue) error {
	if !i.isUnknown(h) {
		return ast.ErrASTPending
	}
	i.states[h] = state
	return nil
}

func (i *ModelImplicant) setBool(h ast.ASTHandle, value bool) error {
	if value {
		return i.setState(h, implicantTrue)
	}
	return i.setState(h, implicantFalse)
}

func (i *ModelImplicant) setNumber(h ast.ASTHandle, value *big.Rat) error {
	if err := i.setState(h, implicantConcrete); err != nil {
		return err
	}
	if err := i.traceNumber(h, value); err != nil {
		return err
	}
	i.numbers[h] = new(big.Rat).Set(value)
	return nil
}

func (i *ModelImplicant) setValue(h, value ast.ASTHandle) error {
	if err := i.setState(h, implicantConcrete); err != nil {
		return err
	}
	if err := i.pin(value); err != nil {
		delete(i.states, h)
		return err
	}
	i.values[h] = value
	return nil
}

func (i *ModelImplicant) assignValue(h, value ast.ASTHandle) error {
	// model::eval leaves its local expr_ref null after a caught exception.
	// Source type/value predicates accept that null and classify it as X.
	if value.IsNull() {
		if err := i.reportUntrackedValue(h, value); err != nil {
			return err
		}
		return i.setState(h, implicantX)
	}
	if i.manager.IsTrue(value) {
		return i.setState(h, implicantTrue)
	}
	if i.manager.IsFalse(value) {
		return i.setState(h, implicantFalse)
	}
	text, _, matched, err := i.manager.ArithmeticNumeralText(value)
	if err != nil {
		return err
	}
	if matched {
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			return ast.ErrASTDomain
		}
		return i.setNumber(h, r)
	}
	isValue, err := i.manager.IsValue(value)
	if err != nil {
		return err
	}
	if isValue {
		return i.setValue(h, value)
	}
	if err = i.reportUntrackedValue(h, value); err != nil {
		return err
	}
	return i.setState(h, implicantX)
}

func (i *ModelImplicant) isBool(h ast.ASTHandle) (bool, error) {
	s, err := i.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := i.manager.MkBoolSort()
	return s == b, err
}

func (i *ModelImplicant) inheritValue(h, from ast.ASTHandle) error {
	if i.isX(from) {
		return i.setState(h, implicantX)
	}
	boolean, err := i.isBool(h)
	if err != nil {
		return err
	}
	if boolean {
		if i.isTrue(from) {
			return i.setState(h, implicantTrue)
		}
		if i.isFalse(from) {
			return i.setState(h, implicantFalse)
		}
		return i.setState(h, implicantX)
	}
	if number := i.numbers[from]; number != nil {
		return i.setNumber(h, number)
	}
	value := from
	isValue, err := i.manager.IsValue(from)
	if err != nil {
		return err
	}
	if !isValue {
		var found bool
		value, found = i.values[from]
		if !found {
			return i.setState(h, implicantX)
		}
	}
	return i.setValue(h, value)
}

func (i *ModelImplicant) appArgs(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	count, err := i.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	args := make([]ast.ASTHandle, count)
	for n := uint32(0); n < count; n++ {
		args[n], err = i.manager.GetArg(h, n)
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func (i *ModelImplicant) evalBasic(h ast.ASTHandle, op int32, args []ast.ASTHandle) error {
	switch op {
	case ast.ASTOpAnd:
		hasX := false
		for _, arg := range args {
			if i.isFalse(arg) {
				return i.setState(h, implicantFalse)
			}
			hasX = hasX || i.isX(arg)
		}
		if hasX {
			return i.setState(h, implicantX)
		}
		return i.setState(h, implicantTrue)
	case ast.ASTOpOr:
		hasX := false
		for _, arg := range args {
			if i.isTrue(arg) {
				return i.setState(h, implicantTrue)
			}
			hasX = hasX || i.isX(arg)
		}
		if hasX {
			return i.setState(h, implicantX)
		}
		return i.setState(h, implicantFalse)
	case ast.ASTOpNot:
		if len(args) != 1 {
			return ast.ErrASTDomain
		}
		if i.isTrue(args[0]) {
			return i.setState(h, implicantFalse)
		}
		if i.isFalse(args[0]) {
			return i.setState(h, implicantTrue)
		}
		return i.setState(h, implicantX)
	case ast.ASTOpImplies:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		if i.isFalse(args[0]) || i.isTrue(args[1]) || args[0] == args[1] {
			return i.setState(h, implicantTrue)
		}
		if i.isTrue(args[0]) && i.isFalse(args[1]) {
			return i.setState(h, implicantFalse)
		}
		return i.setState(h, implicantX)
	case ast.ASTOpIff, ast.ASTOpEq:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		return i.evalEq(h, args[0], args[1])
	case ast.ASTOpIte:
		if len(args) != 3 {
			return ast.ErrASTDomain
		}
		if i.isTrue(args[0]) {
			return i.inheritValue(h, args[1])
		}
		if i.isFalse(args[0]) {
			return i.inheritValue(h, args[2])
		}
		if args[1] == args[2] {
			return i.inheritValue(h, args[1])
		}
		boolean, err := i.isBool(h)
		if err != nil {
			return err
		}
		if boolean && !i.isX(args[1]) && !i.isX(args[2]) && i.isTrue(args[1]) == i.isTrue(args[2]) {
			return i.inheritValue(h, args[1])
		}
		return i.setState(h, implicantX)
	case ast.ASTOpTrue:
		return i.setState(h, implicantTrue)
	case ast.ASTOpFalse:
		return i.setState(h, implicantFalse)
	case ast.ASTOpDistinct:
		values := make([]*big.Rat, len(args))
		for n, arg := range args {
			if i.isX(arg) {
				return i.setState(h, implicantX)
			}
			if i.numbers[arg] == nil {
				return ast.ErrASTDomain
			}
			values[n] = i.numbers[arg]
		}
		sort.Slice(values, func(a, b int) bool { return values[a].Cmp(values[b]) < 0 })
		for n := 0; n+1 < len(values); n++ {
			if values[n].Cmp(values[n+1]) == 0 {
				return i.setState(h, implicantFalse)
			}
		}
		return i.setState(h, implicantTrue)
	default:
		return i.unhandledTerm(h, 808)
	}
}

func (i *ModelImplicant) evalEq(h, left, right ast.ASTHandle) (resultErr error) {
	if left == right {
		return i.setState(h, implicantTrue)
	}
	array, err := i.isArray(left)
	if err != nil {
		return err
	}
	if array {
		return i.evalArrayEq(h, left, right)
	}
	if i.isX(left) || i.isX(right) {
		eqRef, err := i.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, i.manager.DelRef(&eqRef)) }()
		eq, err := i.manager.MkEq(left, right)
		if err != nil {
			return err
		}
		if err = i.manager.SetRef(eqRef, eq); err != nil {
			return err
		}
		out, err := i.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, i.manager.DelRef(&out)) }()
		if err = i.evaluateModelWithoutCompletion(eq, out); err != nil {
			return err
		}
		value, err := i.manager.RefGet(out)
		if err != nil {
			return err
		}
		if i.manager.IsTrue(value) {
			return i.setState(h, implicantTrue)
		}
		if i.manager.IsFalse(value) {
			return i.setState(h, implicantFalse)
		}
		if err = i.traceUnknownEquality(value); err != nil {
			return err
		}
		return i.setState(h, implicantX)
	}
	boolean, err := i.isBool(left)
	if err != nil {
		return err
	}
	if boolean {
		return i.setBool(h, i.isTrue(left) == i.isTrue(right))
	}
	if a, b := i.numbers[left], i.numbers[right]; a != nil && b != nil {
		return i.setBool(h, a.Cmp(b) == 0)
	}
	a, aok := i.values[left]
	b, bok := i.values[right]
	if !aok {
		a = left
		aok, err = i.manager.IsValue(left)
		if err != nil {
			return err
		}
	}
	if !bok {
		b = right
		bok, err = i.manager.IsValue(right)
		if err != nil {
			return err
		}
	}
	if aok && bok || a == b {
		return i.setBool(h, a == b)
	}
	return i.setState(h, implicantX)
}

func floorRat(value *big.Rat) *big.Int {
	q, r := new(big.Int), new(big.Int)
	q.QuoRem(value.Num(), value.Denom(), r)
	if value.Sign() < 0 && r.Sign() != 0 {
		q.Sub(q, big.NewInt(1))
	}
	return q
}

func (i *ModelImplicant) evalArithmetic(h ast.ASTHandle, op int32, args []ast.ASTHandle) error {
	for _, arg := range args {
		if i.isX(arg) {
			return i.setState(h, implicantX)
		}
	}
	if op == 0 {
		text, _, matched, err := i.manager.ArithmeticNumeralText(h)
		if err != nil || !matched {
			if err != nil {
				return err
			}
			return ast.ErrASTDomain
		}
		value, ok := new(big.Rat).SetString(text)
		if !ok {
			return ast.ErrASTDomain
		}
		return i.setNumber(h, value)
	}
	num := func(n int) (*big.Rat, error) {
		if n >= len(args) || i.numbers[args[n]] == nil {
			return nil, ast.ErrASTDomain
		}
		return i.numbers[args[n]], nil
	}
	switch op {
	case 1: // irrational algebraic numeral
		return i.setState(h, implicantX)
	case 2, 3, 4, 5:
		a, err := num(0)
		if err != nil {
			return err
		}
		b, err := num(1)
		if err != nil {
			return err
		}
		cmp := a.Cmp(b)
		return i.setBool(h, op == 2 && cmp <= 0 || op == 3 && cmp >= 0 || op == 4 && cmp < 0 || op == 5 && cmp > 0)
	case 6, 7, 8, 9:
		if len(args) == 0 {
			return ast.ErrASTDomain
		}
		var value *big.Rat
		if op == 6 {
			value = new(big.Rat)
		} else if op == 9 {
			value = new(big.Rat).SetInt64(1)
		} else {
			value = new(big.Rat).Set(i.numbers[args[0]])
		}
		start := 0
		if op == 7 {
			start = 1
		}
		if op == 8 { // Preserve the literal source behavior: UMINUS copies ARG1.
			return i.setNumber(h, value)
		}
		for n := start; n < len(args); n++ {
			a, err := num(n)
			if err != nil {
				return err
			}
			if op == 6 {
				value.Add(value, a)
			} else if op == 7 {
				value.Sub(value, a)
			} else {
				value.Mul(value, a)
			}
		}
		return i.setNumber(h, value)
	case 10, 11, 12, 13:
		a, err := num(0)
		if err != nil {
			return err
		}
		b, err := num(1)
		if err != nil {
			return err
		}
		if b.Sign() == 0 {
			return i.setState(h, implicantX)
		}
		if op == 10 {
			return i.setNumber(h, new(big.Rat).Quo(a, b))
		}
		// Source rational::div uses Euclidean integer division: floor for a
		// positive divisor and ceiling for a negative divisor. This keeps
		// mod nonnegative; rem subsequently follows the divisor's sign.
		ratio := new(big.Rat).Quo(a, b)
		quotient := floorRat(ratio)
		if b.Sign() < 0 && !ratio.IsInt() {
			quotient.Add(quotient, big.NewInt(1))
		}
		if op == 11 {
			return i.setNumber(h, new(big.Rat).SetInt(quotient))
		}
		mod := new(big.Rat).Sub(a, new(big.Rat).Mul(b, new(big.Rat).SetInt(quotient)))
		if op == 12 && b.Sign() < 0 {
			mod.Neg(mod)
		}
		return i.setNumber(h, mod)
	case 14:
		a, err := num(0)
		if err != nil {
			return err
		}
		return i.setNumber(h, a)
	case 15:
		a, err := num(0)
		if err != nil {
			return err
		}
		return i.setNumber(h, new(big.Rat).SetInt(floorRat(a)))
	case 16:
		a, err := num(0)
		if err != nil {
			return err
		}
		return i.setBool(h, a.IsInt())
	case 18:
		return i.setState(h, implicantX)
	default:
		return i.unhandledTerm(h, 432)
	}
}

func (i *ModelImplicant) checkModel(formulas []ast.ASTHandle) (bool, error) {
	todo := append([]ast.ASTHandle(nil), formulas...)
	arithmetic, err := i.manager.ArithmeticFamilyID()
	if err != nil {
		return false, err
	}
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		kind, err := i.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp || !i.isUnknown(current) {
			todo = todo[:len(todo)-1]
			continue
		}
		args, err := i.appArgs(current)
		if err != nil {
			return false, err
		}
		pushed := false
		for _, arg := range args {
			if i.isUnknown(arg) {
				todo = append(todo, arg)
				pushed = true
			}
		}
		if pushed {
			continue
		}
		todo = todo[:len(todo)-1]
		family, err := i.manager.GetFamilyID(current)
		if err != nil {
			return false, err
		}
		op, err := i.manager.GetDeclKind(current)
		if err != nil {
			return false, err
		}
		if family == arithmetic {
			err = i.evalArithmetic(current, op, args)
		} else if family == i.manager.BasicFamilyID() {
			err = i.evalBasic(current, op, args)
		} else {
			var out ast.ASTRef
			out, err = i.manager.NewRef(ast.ASTHandle{})
			if err == nil {
				err = i.evaluateModelWithoutCompletion(current, out)
			}
			if err == nil {
				var value ast.ASTHandle
				value, err = i.manager.RefGet(out)
				if err == nil {
					err = i.assignValue(current, value)
				}
			}
			if out != (ast.ASTRef{}) {
				err = errors.Join(err, i.manager.DelRef(&out))
			}
		}
		if err != nil {
			return false, err
		}
		if err = i.reportEvaluation(35, "assigned ", current, i.evaluationTruth(current)+"\n"); err != nil {
			return false, err
		}
		if err = i.assertEvaluated(current, 852, "!is_unknown(curr)"); err != nil {
			return false, err
		}
	}
	hasX := false
	for _, formula := range formulas {
		if err = i.assertEvaluated(formula, 858, "!is_unknown(form)"); err != nil {
			return false, err
		}
		if err = i.traceMinimization("pdr_verbose", "check_model", 859, "formula is "+i.evaluationTruth(formula)+"\n", []ast.ASTHandle{formula}, nil); err != nil {
			return false, err
		}
		if i.isFalse(formula) {
			if err = i.reportEvaluation(0, "formula false in model: ", formula, "\n"); err != nil {
				return false, err
			}
			if err = diagnostic.BorrowProcessDebugContext().UnreachableSource("src/model/model_implicant.cpp", 864); err != nil {
				return false, err
			}
		}
		if i.isX(formula) {
			if err = i.reportEvaluation(0, "formula undetermined in model: ", formula, "\n"); err != nil {
				return false, err
			}
			if err = i.traceUndeterminedModel(); err != nil {
				return false, err
			}
			hasX = true
		}
	}
	return !hasX, nil
}

// EvalExpr evaluates with completion and normalizes array store tails using
// the source model_implicant::eval reconstruction policy.
func (i *ModelImplicant) EvalExpr(md *Model, expression ast.ASTHandle) (result ast.ASTRef, err error) {
	if err = i.setupModel(md); err != nil {
		return result, errors.Join(err, i.reset())
	}
	defer func() {
		err = errors.Join(err, i.reset())
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, i.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	if err = i.evaluator.SetModelCompletion(true); err != nil {
		return result, err
	}
	return i.evalCompletedExpression(expression)
}

// EvalDeclaration evaluates a zero-arity declaration, using EvalExpr for
// arrays and the model's registered interpretation for other ranges.
func (i *ModelImplicant) EvalDeclaration(md *Model, declaration ast.ASTHandle) (ast.ASTRef, error) {
	if err := i.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if arity, err := i.manager.GetArity(declaration); err != nil || arity != 0 {
		if err != nil {
			return ast.ASTRef{}, err
		}
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	rangeSort, err := i.manager.GetRange(declaration)
	if err != nil {
		return ast.ASTRef{}, err
	}
	family, err := i.manager.GetFamilyID(rangeSort)
	if err != nil {
		return ast.ASTRef{}, err
	}
	arrayFamily, err := i.manager.ArrayFamilyID()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if family == arrayFamily {
		constant, err := i.manager.MkApp(declaration, nil)
		if err != nil {
			return ast.ASTRef{}, err
		}
		return i.EvalExpr(md, constant)
	}
	value, err := md.Core().ConstantInterpretation(declaration)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return i.manager.NewRef(value)
}
