package arith

// Source: src/tactic/arith/bound_manager.cpp/.h.

import (
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type managerBound struct {
	value      big.Rat
	strict     bool
	dependency ast.ASTDependency // borrowed, exactly as in the source maps
}

// BoundManager owns one reference to each bounded expression. Dependencies
// remain borrowed from the source goals, which must outlive their use.
type BoundManager struct {
	self              *BoundManager
	manager           *ast.ASTManager
	context           *uparams.ParamsContext
	globals           *uparams.GlobalParamsService
	family            ast.FamilyID
	lowers, uppers    map[ast.ASTHandle]managerBound
	variables         []ast.ASTHandle
	trace             *diagnostic.TraceTags
	ownerBytes        uint64
	released          int
	resetting, closed bool
}

func NewBoundManager(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*BoundManager, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(BoundManager{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	b := &BoundManager{manager: m, context: c, globals: g, family: family, ownerBytes: bytes, lowers: make(map[ast.ASTHandle]managerBound), uppers: make(map[ast.ASTHandle]managerBound)}
	b.self = b
	return b, nil
}
func (b *BoundManager) owner() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return arena.ErrClosed
	}
	return nil
}
func (b *BoundManager) open() error {
	if err := b.owner(); err != nil {
		return err
	}
	if b.resetting {
		return ast.ErrASTDomain
	}
	return nil
}
func (b *BoundManager) Manager() *ast.ASTManager {
	if b == nil || b.open() != nil {
		return nil
	}
	return b.manager
}
func (b *BoundManager) Variables() ([]ast.ASTHandle, error) {
	if err := b.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), b.variables...), nil
}
func (b *BoundManager) Lower(term ast.ASTHandle) (value big.Rat, strict, found bool, err error) {
	if err = b.open(); err != nil {
		return
	}
	bound, found := b.lowers[term]
	if found {
		value.Set(&bound.value)
		strict = bound.strict
	}
	return value, strict, found, nil
}
func (b *BoundManager) Upper(term ast.ASTHandle) (value big.Rat, strict, found bool, err error) {
	if err = b.open(); err != nil {
		return
	}
	bound, found := b.uppers[term]
	if found {
		value.Set(&bound.value)
		strict = bound.strict
	}
	return value, strict, found, nil
}
func (b *BoundManager) HasLower(term ast.ASTHandle) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	_, found := b.lowers[term]
	return found, nil
}
func (b *BoundManager) HasUpper(term ast.ASTHandle) (bool, error) {
	if err := b.open(); err != nil {
		return false, err
	}
	_, found := b.uppers[term]
	return found, nil
}
func (b *BoundManager) LowerDependency(term ast.ASTHandle) (ast.ASTDependency, error) {
	if err := b.open(); err != nil {
		return ast.ASTDependency{}, err
	}
	return b.lowers[term].dependency, nil
}
func (b *BoundManager) UpperDependency(term ast.ASTHandle) (ast.ASTDependency, error) {
	if err := b.open(); err != nil {
		return ast.ASTDependency{}, err
	}
	return b.uppers[term].dependency, nil
}
func (b *BoundManager) insert(term ast.ASTHandle, lower, strict bool, value *big.Rat, dependency ast.ASTDependency) error {
	bounds, opposite := b.uppers, b.lowers
	if lower {
		bounds, opposite = b.lowers, b.uppers
	}
	old, found := bounds[term]
	if found {
		cmp := value.Cmp(&old.value)
		if !(lower && cmp > 0 || !lower && cmp < 0 || cmp == 0 && strict && !old.strict) {
			return nil
		}
	} else if _, exists := opposite[term]; !exists {
		if err := b.manager.IncRef(term); err != nil {
			return err
		}
		b.variables = append(b.variables, term)
	}
	var next managerBound
	next.value.Set(value)
	next.strict = strict
	next.dependency = old.dependency
	if !dependency.IsNull() {
		next.dependency = dependency
	}
	bounds[term] = next
	return nil
}
func (b *BoundManager) Reset() error {
	if err := b.owner(); err != nil {
		return err
	}
	b.resetting = true
	for b.released < len(b.variables) {
		consumed, err := b.manager.DecRefConsumed(b.variables[b.released])
		if consumed {
			b.released++
		}
		if err != nil {
			return err
		}
	}
	b.variables = nil
	b.released = 0
	clear(b.lowers)
	clear(b.uppers)
	b.resetting = false
	return nil
}
func (b *BoundManager) Close() error {
	if b == nil || b.self != b {
		return ast.ErrASTHandle
	}
	if b.closed {
		return nil
	}
	if err := b.Reset(); err != nil {
		return err
	}
	b.manager.Budget().Release(b.ownerBytes)
	b.ownerBytes = 0
	b.closed = true
	return nil
}

func (b *BoundManager) app(term ast.ASTHandle) (family ast.FamilyID, op int32, args []ast.ASTHandle, app bool, err error) {
	kind, err := b.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return family, op, nil, false, err
	}
	family, err = b.manager.GetFamilyID(term)
	if err != nil {
		return family, op, nil, false, err
	}
	op, err = b.manager.GetDeclKind(term)
	if err != nil {
		return family, op, nil, false, err
	}
	count, err := b.manager.GetNumArgs(term)
	if err != nil {
		return family, op, nil, false, err
	}
	args = make([]ast.ASTHandle, count)
	for i := range args {
		args[i], err = b.manager.GetArg(term, uint32(i))
		if err != nil {
			return family, op, nil, false, err
		}
	}
	return family, op, args, true, nil
}
func (b *BoundManager) constant(term ast.ASTHandle) (bool, error) {
	family, _, args, app, err := b.app(term)
	return app && family == ast.NullFamilyID && len(args) == 0, err
}
func (b *BoundManager) numeral(term ast.ASTHandle) (value big.Rat, integer, found bool, err error) {
	negative := false
	for {
		family, op, args, app, e := b.app(term)
		if e != nil {
			err = e
			return
		}
		if !app || family != b.family || op != 8 || len(args) != 1 {
			break
		}
		negative = !negative
		term = args[0]
	}
	text, integer, found, err := b.manager.ArithmeticNumeralText(term)
	if err != nil || !found {
		return value, integer, found, err
	}
	if _, ok := value.SetString(text); !ok {
		return value, false, false, ast.ErrASTDomain
	}
	if negative {
		value.Neg(&value)
	}
	return value, integer, true, nil
}
func boundSwap(op int32) (int32, error) {
	switch op {
	case 2:
		return 3, nil
	case 3:
		return 2, nil
	case 4:
		return 5, nil
	case 5:
		return 4, nil
	}
	return op, ast.ErrASTDomain
}
func boundNeg(op int32) (int32, error) {
	switch op {
	case 2:
		return 5, nil
	case 3:
		return 4, nil
	case 4:
		return 3, nil
	case 5:
		return 2, nil
	}
	return op, ast.ErrASTDomain
}
func boundNormalizeInteger(value *big.Rat, op int32) int32 {
	if op == 4 {
		value.Sub(value, big.NewRat(1, 1))
		return 2
	}
	if op == 5 {
		value.Add(value, big.NewRat(1, 1))
		return 3
	}
	return op
}
func (b *BoundManager) equality(form ast.ASTHandle) (variable ast.ASTHandle, value big.Rat, integer, found bool, err error) {
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return variable, value, false, false, err
	}
	if len(args) != 2 {
		return variable, value, false, false, ast.ErrASTDomain
	}
	x, y := args[0], args[1]
	constant, e := b.constant(x)
	if e != nil {
		return variable, value, false, false, e
	}
	if !constant {
		x, y = y, x
	}
	constant, e = b.constant(x)
	if e != nil || !constant {
		return variable, value, false, false, e
	}
	value, integer, found, err = b.numeral(y)
	return x, value, integer, found, err
}
func (b *BoundManager) disjunction(form ast.ASTHandle, dependency ast.ASTDependency) (bool, error) {
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpOr || len(args) == 0 {
		return false, err
	}
	var variable ast.ASTHandle
	var low, high big.Rat
	for _, eq := range args {
		x, n, integer, found, err := b.equality(eq)
		if err != nil || !found || !integer {
			return false, err
		}
		if !variable.IsNull() && x != variable {
			return false, nil
		}
		if variable.IsNull() {
			variable = x
			low.Set(&n)
			high.Set(&n)
		}
		if n.Cmp(&low) < 0 {
			low.Set(&n)
		}
		if n.Cmp(&high) > 0 {
			high.Set(&n)
		}
	}
	if err = b.traceBounds(&low, &high); err != nil {
		return false, err
	}
	if err = b.insert(variable, true, false, &low, dependency); err != nil {
		return false, err
	}
	return true, b.insert(variable, false, false, &high, dependency)
}
func (b *BoundManager) Collect(form ast.ASTHandle, dependency ast.ASTDependency) error {
	if err := b.open(); err != nil {
		return err
	}
	if err := b.traceProcessing(form); err != nil {
		return err
	}
	if found, err := b.disjunction(form, dependency); err != nil || found {
		return err
	}
	variable, value, _, found, err := b.equality(form)
	if err != nil {
		return err
	}
	if found {
		if err = b.insert(variable, true, false, &value, dependency); err != nil {
			return err
		}
		return b.insert(variable, false, false, &value, dependency)
	}
	positive := true
	for {
		family, op, args, app, e := b.app(form)
		if e != nil {
			return e
		}
		if !app || family != b.manager.BasicFamilyID() || op != ast.ASTOpNot || len(args) != 1 {
			break
		}
		positive = !positive
		form = args[0]
	}
	family, op, args, app, err := b.app(form)
	if err != nil || !app || family != b.family || op < 2 || op > 5 {
		return err
	}
	if len(args) < 2 {
		return ast.ErrASTDomain
	}
	x, y := args[0], args[1]
	constant, err := b.constant(x)
	if err != nil {
		return err
	}
	var integer bool
	if constant {
		value, integer, found, err = b.numeral(y)
	}
	if err != nil {
		return err
	}
	if !constant || !found {
		constant, err = b.constant(y)
		if err != nil || !constant {
			return err
		}
		value, integer, found, err = b.numeral(x)
		if err != nil || !found {
			return err
		}
		x = y
		op, err = boundSwap(op)
		if err != nil {
			return err
		}
	}
	if !positive {
		op, err = boundNeg(op)
		if err != nil {
			return err
		}
	}
	if integer {
		op = boundNormalizeInteger(&value, op)
	}
	if err = b.traceFound(x); err != nil {
		return err
	}
	return b.insert(x, op == 3 || op == 5, op == 4 || op == 5, &value, dependency)
}
func (b *BoundManager) CollectGoal(goal *tactic.Goal) error {
	if err := b.open(); err != nil {
		return err
	}
	if goal == nil || goal.Manager() != b.manager {
		return ast.ErrASTHandle
	}
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		form, e := goal.Form(i)
		if e != nil {
			return e
		}
		dep, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if err = b.Collect(form, dep); err != nil {
			return err
		}
	}
	return nil
}
