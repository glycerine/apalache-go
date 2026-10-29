package smtcore

// Source: src/smt/smt_farkas_util.cpp and src/smt/smt_farkas_util.h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// SMTFarkasUtil combines arithmetic premises using Farkas coefficients. The
// parameter services are borrowed; the utility owns its private rewriter and
// each premise retained by Add until Reset or Close.
type SMTFarkasUtil struct {
	manager         *ast.ASTManager
	family          ast.FamilyID
	rewriter        *rewriter.TheoryRewriter
	inequalities    []ast.ASTHandle
	coefficients    []*big.Rat
	normalizeFactor big.Rat
	splitLiterals   bool
	closed          bool
}

func NewSMTFarkasUtil(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (result *SMTFarkasUtil, err error) {
	if m == nil || context == nil || globals == nil || context.Symbols() != m.Symbols || globals.Context != context {
		return nil, ast.ErrASTDomain
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	local, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, context.Del(&local)) }()
	name := "gcd_rounding"
	if err = context.SetBoolName(&local, &name, true); err != nil {
		return nil, err
	}
	rw, err := rewriter.NewTheoryRewriter(m, context, globals, &local)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return nil, err
	}
	result = &SMTFarkasUtil{manager: m, family: family, rewriter: rw}
	result.normalizeFactor.SetInt64(1)
	return result, nil
}

func (f *SMTFarkasUtil) open() error {
	if f == nil || f.closed || f.manager == nil || f.rewriter == nil || f.rewriter.Manager() != f.manager {
		return ast.ErrASTHandle
	}
	return nil
}

func (f *SMTFarkasUtil) Close() (result error) {
	if f == nil {
		return ast.ErrASTHandle
	}
	if f.closed {
		return nil
	}
	result = f.Reset()
	if f.rewriter != nil {
		result = errors.Join(result, f.rewriter.Close())
		f.rewriter = nil
	}
	f.closed = true
	return result
}

func (f *SMTFarkasUtil) Reset() (result error) {
	if err := f.open(); err != nil {
		return err
	}
	for _, premise := range f.inequalities {
		result = errors.Join(result, f.manager.DecRef(premise))
	}
	f.inequalities = f.inequalities[:0]
	f.coefficients = f.coefficients[:0]
	return result
}

func (f *SMTFarkasUtil) SetSplitLiterals(enabled bool) error {
	if err := f.open(); err != nil {
		return err
	}
	f.splitLiterals = enabled
	return nil
}

func (f *SMTFarkasUtil) NormalizeFactor() (*big.Rat, error) {
	if err := f.open(); err != nil {
		return nil, err
	}
	return new(big.Rat).Set(&f.normalizeFactor), nil
}

func (f *SMTFarkasUtil) operation(expression ast.ASTHandle) (int32, ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, err := f.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := f.manager.GetFamilyID(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	op, err := f.manager.GetDeclKind(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	valid := family == f.family && (op == smtArithOpLE || op == smtArithOpGE || op == smtArithOpLT || op == smtArithOpGT) ||
		family == f.manager.BasicFamilyID() && op == ast.ASTOpEq
	if !valid {
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	count, err := f.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	left, err := f.manager.GetArg(expression, 0)
	if err != nil {
		return op, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err := f.manager.GetArg(expression, 1)
	return op, left, right, err == nil, err
}

func (f *SMTFarkasUtil) isInteger(expression ast.ASTHandle) (bool, error) {
	_, left, _, matched, err := f.operation(expression)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return false, err
	}
	sort, err := f.manager.GetSort(left)
	if err != nil {
		return false, err
	}
	family, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(sort)
	if err != nil {
		return false, err
	}
	if family != f.family || kind != 0 && kind != 1 {
		return false, ast.ErrASTDomain
	}
	return kind == 1, nil
}

func (f *SMTFarkasUtil) app(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	return f.manager.MkFamilyApp(f.family, op, nil, args)
}

func (f *SMTFarkasUtil) one() (ast.ASTHandle, error) {
	return f.manager.ArithmeticNumeralRat(big.NewRat(1, 1), true)
}

func (f *SMTFarkasUtil) fixSign(positive bool, constraint ast.ASTHandle) (ast.ASTHandle, error) {
	op, left, right, matched, err := f.operation(constraint)
	if err != nil || !matched {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	integer, err := f.isInteger(constraint)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if integer && positive && (op == smtArithOpLT || op == smtArithOpGT) {
		if op == smtArithOpGT {
			left, right = right, left
		}
		one, err := f.one()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		left, err = f.app(smtArithOpAdd, left, one)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.app(smtArithOpLE, left, right)
	}
	if integer && !positive && (op == smtArithOpLE || op == smtArithOpGE) {
		if op == smtArithOpGE {
			left, right = right, left
		}
		one, err := f.one()
		if err != nil {
			return ast.ASTHandle{}, err
		}
		right, err = f.app(smtArithOpAdd, right, one)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.app(smtArithOpGE, left, right)
	}
	if positive {
		return constraint, nil
	}
	complement := map[int32]int32{smtArithOpLE: smtArithOpGT, smtArithOpLT: smtArithOpGE, smtArithOpGE: smtArithOpLT, smtArithOpGT: smtArithOpLE}
	result, found := complement[op]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return f.app(result, left, right)
}

func (f *SMTFarkasUtil) Add(coefficient *big.Rat, constraint ast.ASTHandle) error {
	if err := f.open(); err != nil || coefficient == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	positive := true
	for {
		negated, child, err := f.manager.IsNot(constraint)
		if err != nil {
			return err
		}
		if !negated {
			break
		}
		positive = !positive
		constraint = child
	}
	if coefficient.Sign() == 0 || f.manager.IsTrue(constraint) {
		return nil
	}
	constraint, err := f.fixSign(positive, constraint)
	if err != nil {
		return err
	}
	if err = f.manager.IncRef(constraint); err != nil {
		return err
	}
	f.inequalities = append(f.inequalities, constraint)
	f.coefficients = append(f.coefficients, new(big.Rat).Set(coefficient))
	return nil
}

func smtRatLCM(left, right *big.Int) *big.Int {
	if left.Sign() == 0 || right.Sign() == 0 {
		return new(big.Int)
	}
	gcd := new(big.Int).GCD(nil, nil, left, right)
	return new(big.Int).Abs(new(big.Int).Mul(new(big.Int).Quo(new(big.Int).Set(left), gcd), right))
}

func (f *SMTFarkasUtil) normalizeCoefficients() {
	lcm := big.NewInt(1)
	for _, coefficient := range f.coefficients {
		lcm = smtRatLCM(lcm, coefficient.Denom())
	}
	f.normalizeFactor.SetInt(lcm)
	if lcm.Cmp(big.NewInt(1)) == 0 {
		return
	}
	factor := new(big.Rat).SetInt(lcm)
	for index := range f.coefficients {
		f.coefficients[index].Mul(f.coefficients[index], factor)
	}
}

func (f *SMTFarkasUtil) addMultiple(sum ast.ASTHandle, coefficient *big.Rat, term ast.ASTHandle) (ast.ASTHandle, error) {
	part := term
	if coefficient.Cmp(big.NewRat(1, 1)) != 0 {
		integer, err := f.termIsInteger(term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		numeral, err := f.manager.ArithmeticNumeralRat(coefficient, coefficient.IsInt() && integer)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		part, err = f.app(smtArithOpMul, numeral, term)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return f.app(smtArithOpAdd, sum, part)
}

func (f *SMTFarkasUtil) termIsInteger(term ast.ASTHandle) (bool, error) {
	sort, err := f.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	family, err := f.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	kind, err := f.manager.GetDeclKind(sort)
	return family == f.family && kind == 1, err
}

func (f *SMTFarkasUtil) extractConsequence(low, high int, out ast.ASTRef) (err error) {
	integer, err := f.isInteger(f.inequalities[0])
	if err != nil {
		return err
	}
	zero, err := f.manager.ArithmeticNumeralRat(new(big.Rat), integer)
	if err != nil {
		return err
	}
	sum := zero
	strict, equalityOnly := false, true
	for index := low; index < high; index++ {
		op, left, right, matched, err := f.operation(f.inequalities[index])
		if err != nil || !matched {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return err
		}
		if op == smtArithOpGE || op == smtArithOpGT {
			left, right = right, left
		}
		coefficient := f.coefficients[index]
		sum, err = f.addMultiple(sum, coefficient, left)
		if err != nil {
			return err
		}
		sum, err = f.addMultiple(sum, new(big.Rat).Neg(coefficient), right)
		if err != nil {
			return err
		}
		if op != ast.ASTOpEq {
			equalityOnly = false
		}
		if op == smtArithOpLT || op == smtArithOpGT {
			strict = true
		}
	}
	integer, err = f.termIsInteger(sum)
	if err != nil {
		return err
	}
	zero, err = f.manager.ArithmeticNumeralRat(new(big.Rat), integer)
	if err != nil {
		return err
	}
	var consequence ast.ASTHandle
	if equalityOnly {
		consequence, err = f.manager.MkEq(sum, zero)
	} else if strict {
		consequence, err = f.app(smtArithOpLT, sum, zero)
	} else {
		consequence, err = f.app(smtArithOpLE, sum, zero)
	}
	if err != nil {
		return err
	}
	consequence, err = f.manager.MkNot(consequence)
	if err != nil {
		return err
	}
	temporary, err := f.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.manager.DelRef(&temporary)) }()
	if err = f.rewriter.Rewrite(consequence, temporary); err != nil {
		return err
	}
	rewritten, err := f.manager.RefGet(temporary)
	if err != nil {
		return err
	}
	rewritten, err = f.fixDifferenceLogic(rewritten)
	if err != nil {
		return err
	}
	return f.manager.SetRef(out, rewritten)
}

func (f *SMTFarkasUtil) fixDifferenceLogic(expression ast.ASTHandle) (ast.ASTHandle, error) {
	negated, child, err := f.manager.IsNot(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if negated {
		child, err = f.fixDifferenceLogic(child)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return f.manager.MkNot(child)
	}
	_, left, right, matched, err := f.operation(expression)
	if err != nil || !matched {
		return expression, err
	}
	family, err := f.manager.GetFamilyID(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := f.manager.GetDeclKind(left)
	if err != nil || family != f.family || op != smtArithOpAdd {
		return expression, err
	}
	count, err := f.manager.GetNumArgs(left)
	if err != nil || count != 2 {
		return expression, err
	}
	first, err := f.manager.GetArg(left, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstFamily, err := f.manager.GetFamilyID(first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	firstOp, err := f.manager.GetDeclKind(first)
	if err != nil || firstFamily != f.family || firstOp != smtArithOpMul {
		return expression, err
	}
	second, err := f.manager.GetArg(left, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err = f.app(smtArithOpAdd, second, first)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := f.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return f.manager.MkApp(declaration, []ast.ASTHandle{left, right})
}

func (f *SMTFarkasUtil) processTerm(expression ast.ASTHandle, parent, size map[uint32]uint32) (uint32, error) {
	root, err := f.manager.GetASTID(expression)
	if err != nil {
		return 0, err
	}
	seen := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		term := stack[last]
		stack = stack[:last]
		if seen[term] {
			continue
		}
		seen[term] = true
		kind, err := f.manager.GetKind(term)
		if err != nil {
			return 0, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := f.manager.GetFamilyID(term)
		if err != nil {
			return 0, err
		}
		if family == ast.NullFamilyID {
			id, err := f.manager.GetASTID(term)
			if err != nil {
				return 0, err
			}
			f.union(root, id, parent, size)
		}
		count, err := f.manager.GetNumArgs(term)
		if err != nil {
			return 0, err
		}
		for index := uint32(0); index < count; index++ {
			child, err := f.manager.GetArg(term, index)
			if err != nil {
				return 0, err
			}
			stack = append(stack, child)
		}
	}
	return root, nil
}

func (f *SMTFarkasUtil) find(index uint32, parent, size map[uint32]uint32) uint32 {
	if _, found := parent[index]; !found {
		parent[index], size[index] = index, 1
	}
	for parent[index] != index {
		index = parent[index]
	}
	return index
}

func (f *SMTFarkasUtil) union(left, right uint32, parent, size map[uint32]uint32) {
	left, right = f.find(left, parent, size), f.find(right, parent, size)
	if left == right {
		return
	}
	if size[left] > size[right] {
		left, right = right, left
	}
	parent[left] = right
	size[right] += size[left]
}

func (f *SMTFarkasUtil) partition() ([]int, error) {
	parent, size := make(map[uint32]uint32), make(map[uint32]uint32)
	representatives := make([]uint32, len(f.inequalities))
	for index, inequality := range f.inequalities {
		var err error
		representatives[index], err = f.processTerm(inequality, parent, size)
		if err != nil {
			return nil, err
		}
	}
	var ends []int
	for head := 0; head < len(f.inequalities); {
		representative := f.find(representatives[head], parent, size)
		tail := head
		for index := head + 1; index < len(f.inequalities); index++ {
			if f.find(representatives[index], parent, size) != representative {
				continue
			}
			tail++
			if tail != index {
				representatives[tail], representatives[index] = representatives[index], representatives[tail]
				f.inequalities[tail], f.inequalities[index] = f.inequalities[index], f.inequalities[tail]
				f.coefficients[tail], f.coefficients[index] = f.coefficients[index], f.coefficients[tail]
			}
		}
		head = tail + 1
		ends = append(ends, head)
	}
	return ends, nil
}

// Get stores the complement of the weighted premises in out.
func (f *SMTFarkasUtil) Get(out ast.ASTRef) (err error) {
	if err := f.open(); err != nil {
		return err
	}
	f.normalizeFactor.SetInt64(1)
	if len(f.coefficients) == 0 {
		value, err := f.manager.MkBoolVal(false)
		if err != nil {
			return err
		}
		return f.manager.SetRef(out, value)
	}
	integer, err := f.isInteger(f.inequalities[0])
	if err != nil {
		return err
	}
	if integer {
		f.normalizeCoefficients()
	}
	if !f.splitLiterals {
		return f.extractConsequence(0, len(f.coefficients), out)
	}
	ends, err := f.partition()
	if err != nil {
		return err
	}
	literals := make([]ast.ASTHandle, 0, len(ends))
	refs := make([]ast.ASTRef, 0, len(ends))
	defer func() {
		for index := range refs {
			err = errors.Join(err, f.manager.DelRef(&refs[index]))
		}
	}()
	low := 0
	for _, high := range ends {
		ref, err := f.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		if err = f.extractConsequence(low, high, ref); err != nil {
			return err
		}
		literal, err := f.manager.RefGet(ref)
		if err != nil {
			return err
		}
		literals = append(literals, literal)
		low = high
	}
	disjunction, err := f.manager.MkOrArgs(literals)
	if err != nil {
		return err
	}
	return f.manager.SetRef(out, disjunction)
}
