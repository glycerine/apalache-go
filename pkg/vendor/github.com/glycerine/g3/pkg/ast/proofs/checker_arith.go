package proofs

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

const (
	proofArithLE  int32 = 2
	proofArithGE  int32 = 3
	proofArithLT  int32 = 4
	proofArithGT  int32 = 5
	proofArithAdd int32 = 6
	proofArithMul int32 = 9
)

func (c *Checker) parameterSymbolsAndCoefficients(p ast.ASTHandle) (first, second string, coeffs []*big.Rat, result error) {
	d, err := c.manager.GetDecl(p)
	if err != nil {
		return "", "", nil, err
	}
	params, err := c.manager.GetDeclParametersCopy(d)
	if err != nil {
		return "", "", nil, err
	}
	defer func() {
		for i := range params {
			result = errors.Join(result, c.manager.DelParameter(&params[i]))
		}
	}()
	if len(params) == 0 {
		return "", "", nil, ast.ErrASTDomain
	}
	s, err := c.manager.ParameterSymbol(params[0])
	if err != nil {
		return "", "", nil, err
	}
	first, err = c.manager.Symbols.String(s)
	if err != nil {
		return "", "", nil, err
	}
	if len(params) == 1 {
		return first, "", nil, nil
	}
	s, err = c.manager.ParameterSymbol(params[1])
	if err != nil {
		return "", "", nil, err
	}
	second, err = c.manager.Symbols.String(s)
	if err != nil {
		return "", "", nil, err
	}
	service := c.manager.NumeralService()
	for i := 2; i < len(params); i++ {
		q, e := service.NewValue()
		if e != nil {
			return "", "", nil, e
		}
		matched, e := c.manager.ParameterRationalCopy(params[i], &q)
		if e != nil || !matched {
			return "", "", nil, errors.Join(e, service.Del(&q))
		}
		text, e := service.String(&q)
		e = errors.Join(e, service.Del(&q))
		if e != nil {
			return "", "", nil, e
		}
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			return "", "", nil, ast.ErrASTDomain
		}
		coeffs = append(coeffs, r)
	}
	return first, second, coeffs, nil
}

func scaleRationalCoefficients(values []*big.Rat) {
	lcm := big.NewInt(1)
	for _, q := range values {
		d := new(big.Int).Set(q.Denom())
		g := new(big.Int).GCD(nil, nil, lcm, d)
		lcm.Mul(new(big.Int).Quo(lcm, g), d)
	}
	if lcm.Cmp(big.NewInt(1)) == 0 {
		return
	}
	for _, q := range values {
		q.Mul(q, new(big.Rat).SetInt(lcm))
	}
}

type proofArithRewriter struct {
	context  *uparams.ParamsContext
	catalog  *uparams.GlobalParamsCatalog
	globals  *uparams.GlobalParamsService
	rewriter *rewriter.TheoryRewriter
	manager  *ast.ASTManager
}

func newProofArithRewriter(m *ast.ASTManager) (r *proofArithRewriter, result error) {
	r = &proofArithRewriter{manager: m}
	fail := func(e error) (*proofArithRewriter, error) { return r, errors.Join(e, r.Close()) }
	var err error
	r.context, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget(), Text: arena.VectorConfig{Descriptors: arena.Config{Budget: m.Budget()}}})
	if err != nil {
		return fail(err)
	}
	r.catalog, err = uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
	if err != nil {
		return fail(err)
	}
	r.globals, err = uparams.NewGlobalParamsService(m.Symbols, r.context, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: r.catalog})
	if err != nil {
		return fail(err)
	}
	if err = r.globals.InitializeSource(); err != nil {
		return fail(err)
	}
	r.rewriter, err = rewriter.NewTheoryRewriter(m, r.context, r.globals, nil)
	if err != nil {
		return fail(err)
	}
	return r, nil
}

func (r *proofArithRewriter) Close() (result error) {
	if r == nil {
		return nil
	}
	if r.rewriter != nil {
		result = errors.Join(result, r.rewriter.Close())
		r.rewriter = nil
	}
	if r.globals != nil {
		result = errors.Join(result, r.globals.Close())
		r.globals = nil
	}
	if r.catalog != nil {
		result = errors.Join(result, r.catalog.Close())
		r.catalog = nil
	}
	if r.context != nil {
		result = errors.Join(result, r.context.Close())
		r.context = nil
	}
	return result
}

func (r *proofArithRewriter) rewrite(h ast.ASTHandle) (result ast.ASTHandle, body error) {
	out, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { body = errors.Join(body, r.manager.DelRef(&out)) }()
	if err = r.rewriter.Rewrite(h, out); err != nil {
		return ast.ASTHandle{}, err
	}
	return r.manager.RefGet(out)
}

func (c *Checker) rationalNumeral(value *big.Rat, integer bool) (ast.ASTHandle, error) {
	s := c.manager.NumeralService()
	q, err := s.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer s.Del(&q)
	if err = s.SetString(&q, value.RatString()); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.manager.ArithmeticNumeral(&q, integer)
}

func (c *Checker) checkArithLiteral(arith ast.FamilyID, isPositive bool, literal ast.ASTHandle, coefficient *big.Rat, sum ast.ASTHandle, strict *bool, rw *proofArithRewriter) (ast.ASTHandle, bool, error) {
	if n, yes, err := c.op(literal, ast.ASTOpNot, 1); err != nil {
		return sum, false, err
	} else if yes {
		literal = n[0]
		isPositive = !isPositive
	}
	fid, err := c.manager.GetFamilyID(literal)
	if err != nil {
		return sum, false, err
	}
	kind, err := c.manager.GetDeclKind(literal)
	if err != nil {
		return sum, false, err
	}
	equality := fid == c.manager.BasicFamilyID() && kind == ast.ASTOpEq
	if !equality && (fid != arith || kind < proofArithLE || kind > proofArithGT) {
		return sum, false, nil
	}
	_, args, yes, err := c.app(literal)
	if err != nil || !yes || len(args) != 2 {
		return sum, false, err
	}
	sort, err := c.manager.GetSort(args[0])
	if err != nil {
		return sum, false, err
	}
	sortFamily, err := c.manager.GetFamilyID(sort)
	if err != nil {
		return sum, false, err
	}
	sortKind, err := c.manager.GetDeclKind(sort)
	if err != nil {
		return sum, false, err
	}
	if sortFamily != arith || sortKind != 0 && sortKind != 1 {
		return sum, false, nil
	}
	integer := sortKind == 1
	if !integer && (isPositive && (kind == proofArithGT || kind == proofArithLT) || !isPositive && (kind == proofArithGE || kind == proofArithLE)) {
		*strict = true
	}
	abs := new(big.Rat).Abs(new(big.Rat).Set(coefficient))
	sign1 := abs
	if equality {
		sign1 = new(big.Rat).Set(coefficient)
	}
	sign2 := new(big.Rat).Neg(new(big.Rat).Set(sign1))
	if sum.IsNull() {
		sum, err = c.rationalNumeral(big.NewRat(0, 1), integer)
		if err != nil {
			return sum, false, err
		}
	}
	a0, a1 := args[0], args[1]
	if isPositive && (kind == proofArithGE || kind == proofArithGT) || !isPositive && (kind == proofArithLE || kind == proofArithLT) {
		a0, a1 = a1, a0
	}
	strictInequality := isPositive && (kind == proofArithGT || kind == proofArithLT) || !isPositive && (kind == proofArithGE || kind == proofArithLE)
	n1, err := c.rationalNumeral(sign1, integer)
	if err != nil {
		return sum, false, err
	}
	if integer && strictInequality {
		sum, err = c.manager.MkFamilyApp(arith, proofArithAdd, nil, []ast.ASTHandle{sum, n1})
		if err != nil {
			return sum, false, err
		}
	}
	n2, err := c.rationalNumeral(sign2, integer)
	if err != nil {
		return sum, false, err
	}
	t0, err := c.manager.MkFamilyApp(arith, proofArithMul, nil, []ast.ASTHandle{n1, a0})
	if err != nil {
		return sum, false, err
	}
	sum, err = c.manager.MkFamilyApp(arith, proofArithAdd, nil, []ast.ASTHandle{sum, t0})
	if err != nil {
		return sum, false, err
	}
	t1, err := c.manager.MkFamilyApp(arith, proofArithMul, nil, []ast.ASTHandle{n2, a1})
	if err != nil {
		return sum, false, err
	}
	sum, err = c.manager.MkFamilyApp(arith, proofArithAdd, nil, []ast.ASTHandle{sum, t1})
	if err != nil {
		return sum, false, err
	}
	sum, err = rw.rewrite(sum)
	return sum, err == nil, err
}

func (c *Checker) checkTheoryLemma(p ast.ASTHandle) (valid bool, side []ast.ASTHandle, result error) {
	first, second, coeffs, err := c.parameterSymbolsAndCoefficients(p)
	if err != nil {
		return false, nil, err
	}
	if first != "arith" || second != "farkas" {
		return true, nil, nil
	}
	scaleRationalCoefficients(coeffs)
	arithSymbol, err := c.manager.Symbols.Intern("arith")
	if err != nil {
		return false, nil, err
	}
	arith, err := c.manager.FamilyIDForSymbol(arithSymbol)
	if err != nil {
		return false, nil, err
	}
	rw, err := newProofArithRewriter(c.manager)
	if err != nil {
		return false, nil, err
	}
	defer func() { result = errors.Join(result, rw.Close()) }()
	parents, err := c.parents(p)
	if err != nil {
		return false, nil, err
	}
	fact, has, err := c.fact(p)
	if err != nil || !has {
		return false, nil, err
	}
	offset := 0
	var sum ast.ASTHandle
	strict := false
	consume := func(positive bool, literal ast.ASTHandle) (bool, error) {
		if offset >= len(coeffs) {
			return false, nil
		}
		var ok bool
		sum, ok, err = c.checkArithLiteral(arith, positive, literal, coeffs[offset], sum, &strict, rw)
		offset++
		return ok, err
	}
	for _, parent := range parents {
		f, yes, e := c.fact(parent)
		if e != nil || !yes {
			return false, nil, e
		}
		ok, e := consume(true, f)
		if e != nil || !ok {
			return false, nil, e
		}
	}
	if disjuncts, yes, e := c.op(fact, ast.ASTOpOr, -1); e != nil {
		return false, nil, e
	} else if yes {
		for _, literal := range disjuncts {
			ok, e := consume(false, literal)
			if e != nil || !ok {
				return false, nil, e
			}
		}
	} else if !c.manager.IsFalse(fact) {
		ok, e := consume(false, fact)
		if e != nil || !ok {
			return false, nil, e
		}
	}
	if sum.IsNull() {
		return false, nil, nil
	}
	sort, err := c.manager.GetSort(sum)
	if err != nil {
		return false, nil, err
	}
	sortKind, err := c.manager.GetDeclKind(sort)
	if err != nil {
		return false, nil, err
	}
	zero, err := c.rationalNumeral(big.NewRat(0, 1), sortKind == 1)
	if err != nil {
		return false, nil, err
	}
	relation := proofArithLE
	if strict {
		relation = proofArithLT
	}
	claim, err := c.manager.MkFamilyApp(arith, relation, nil, []ast.ASTHandle{sum, zero})
	if err != nil {
		return false, nil, err
	}
	claim, err = rw.rewrite(claim)
	if err != nil {
		return false, nil, err
	}
	return c.manager.IsFalse(claim), nil, nil
}
