package pdr

// Source: the live scaler and closure paths used by
// core_convex_hull_generalizer::method1 in src/muz/pdr/pdr_closure.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	closureArithLE  int32 = 2
	closureArithGE  int32 = 3
	closureArithLT  int32 = 4
	closureArithGT  int32 = 5
	closureArithAdd int32 = 6
	closureArithMul int32 = 9
)

type closureScaler struct {
	manager    *ast.ASTManager
	arithmetic ast.FamilyID
	k          ast.ASTHandle
	translate  map[ast.ASTHandle]ast.ASTHandle
	cache      [2]map[ast.ASTHandle]ast.ASTHandle
	workspace  []ast.ASTRef
}

func newClosureScaler(manager *ast.ASTManager, k ast.ASTHandle, translate map[ast.ASTHandle]ast.ASTHandle) (*closureScaler, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &closureScaler{
		manager: manager, arithmetic: arithmetic, k: k, translate: translate,
		cache: [2]map[ast.ASTHandle]ast.ASTHandle{
			make(map[ast.ASTHandle]ast.ASTHandle),
			make(map[ast.ASTHandle]ast.ASTHandle),
		},
	}, nil
}

func (s *closureScaler) pin(handle ast.ASTHandle) error {
	reference, err := s.manager.NewRef(handle)
	if err != nil {
		return err
	}
	s.workspace = append(s.workspace, reference)
	return nil
}

func (s *closureScaler) scale(expression ast.ASTHandle, insideMultiplication bool) (ast.ASTHandle, error) {
	cacheIndex := 0
	if insideMultiplication {
		cacheIndex = 1
	}
	if result, found := s.cache[cacheIndex][expression]; found {
		return result, nil
	}
	kind, err := s.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTApp {
		return expression, nil
	}
	declaration, err := s.manager.GetDecl(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if translated, found := s.translate[declaration]; found {
		return translated, nil
	}
	if !insideMultiplication {
		_, _, numeral, x := s.manager.ArithmeticNumeralText(expression)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if numeral {
			result, x := s.manager.MkArithmeticMul(s.k, expression)
			if x != nil {
				return ast.ASTHandle{}, x
			}
			if x = s.pin(result); x != nil {
				return ast.ASTHandle{}, x
			}
			s.cache[cacheIndex][expression] = result
			return result, nil
		}
	}
	family, err := s.manager.GetFamilyID(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	operator, err := s.manager.GetDeclKind(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	recursiveMultiplication := insideMultiplication || family == s.arithmetic && operator == closureArithMul
	arguments, err := s.manager.AppArgumentView(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	scaled := make([]ast.ASTHandle, arguments.Count())
	for index := range scaled {
		scaled[index], err = s.scaleArgument(arguments, uint32(index), recursiveMultiplication)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	result, err := s.manager.MkApp(declaration, scaled)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.pin(result); err != nil {
		return ast.ASTHandle{}, err
	}
	s.cache[cacheIndex][expression] = result
	return result, nil
}

func (s *closureScaler) scaleArgument(arguments ast.AppArguments, index uint32, insideMultiplication bool) (ast.ASTHandle, error) {
	argument, err := arguments.At(index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.scale(argument, insideMultiplication)
}

func (s *closureScaler) apply(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	result, err := s.scale(expression, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return s.manager.NewRef(result)
}

func (s *closureScaler) close() (result error) {
	if s == nil || s.manager == nil {
		return nil
	}
	result = closeASTRefs(s.manager, s.workspace)
	s.workspace = nil
	return result
}

func scaleClosureExpression(manager *ast.ASTManager, expression, k ast.ASTHandle, translate map[ast.ASTHandle]ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	scaler, err := newClosureScaler(manager, k, translate)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, scaler.close()) }()
	return scaler.apply(expression)
}

type convexClosure struct {
	manager     *ast.ASTManager
	transformer *PredicateTransformer
	arithmetic  ast.FamilyID
	isClosure   bool
	sigmas      []ast.ASTHandle
	variables   []map[ast.ASTHandle]ast.ASTHandle
	references  []ast.ASTRef
	closed      bool
}

func newConvexClosure(transformer *PredicateTransformer, isClosure bool) (*convexClosure, error) {
	if transformer == nil {
		return nil, ErrPredicateTransformerOwner
	}
	if err := transformer.open(); err != nil {
		return nil, err
	}
	arithmetic, err := transformer.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &convexClosure{manager: transformer.manager, transformer: transformer, arithmetic: arithmetic, isClosure: isClosure}, nil
}

func (c *convexClosure) open() error {
	if c == nil || c.closed || c.manager == nil || c.transformer == nil {
		return ErrPredicateTransformerOwner
	}
	return c.transformer.open()
}

func (c *convexClosure) pin(handle ast.ASTHandle) error {
	reference, err := c.manager.NewRef(handle)
	if err != nil {
		return err
	}
	c.references = append(c.references, reference)
	return nil
}

func (c *convexClosure) arithmeticApp(operator int32, arguments ...ast.ASTHandle) (ast.ASTHandle, error) {
	return c.manager.MkFamilyApp(c.arithmetic, operator, nil, arguments)
}

func (c *convexClosure) ensureVariables(count int) error {
	if count <= 0 {
		return ast.ErrASTDomain
	}
	realSort, err := c.manager.MkFamilySort(c.arithmetic, 0, nil)
	if err != nil {
		return err
	}
	for len(c.sigmas) < count {
		sigma, x := c.manager.MkFreshConst("sigma", realSort)
		if x != nil {
			return x
		}
		if x = c.pin(sigma); x != nil {
			return x
		}
		c.sigmas = append(c.sigmas, sigma)
		c.variables = append(c.variables, make(map[ast.ASTHandle]ast.ASTHandle))
	}
	return nil
}

func (c *convexClosure) addVariables(count int, formulas *[]ast.ASTHandle, workspace *[]ast.ASTRef) (resultErr error) {
	if err := c.ensureVariables(count); err != nil {
		return err
	}
	pin := func(handle ast.ASTHandle) error {
		reference, err := c.manager.NewRef(handle)
		if err == nil {
			*workspace = append(*workspace, reference)
		}
		return err
	}
	signature, err := c.transformer.Signature()
	if err != nil {
		return err
	}
	for _, oldDeclaration := range signature {
		newDeclaration, x := c.transformer.owner.OldToNew(oldDeclaration, 0)
		if x != nil {
			return x
		}
		sort, x := c.manager.GetRange(oldDeclaration)
		if x != nil {
			return x
		}
		family, x := c.manager.GetFamilyID(sort)
		if x != nil {
			return x
		}
		sortKind, x := c.manager.GetDeclKind(sort)
		if x != nil {
			return x
		}
		if family != c.arithmetic || sortKind != 0 && sortKind != 1 {
			continue
		}
		name, x := c.manager.GetASTName(newDeclaration)
		if x != nil {
			return x
		}
		prefix, x := c.manager.Symbols.Display(name)
		if x != nil {
			return x
		}
		copies := make([]ast.ASTHandle, count)
		for index := range copies {
			copyVariable, found := c.variables[index][newDeclaration]
			if !found {
				copyVariable, x = c.manager.MkFreshConst(prefix, sort)
				if x != nil {
					return x
				}
				if x = c.pin(copyVariable); x != nil {
					return x
				}
				c.variables[index][newDeclaration] = copyVariable
			}
			copies[index] = copyVariable
		}
		current, x := c.manager.MkApp(newDeclaration, nil)
		if x != nil {
			return x
		}
		if x = pin(current); x != nil {
			return x
		}
		sum, x := c.arithmeticApp(closureArithAdd, copies...)
		if x != nil {
			return x
		}
		if x = pin(sum); x != nil {
			return x
		}
		equality, x := c.manager.MkEq(current, sum)
		if x != nil {
			return x
		}
		if x = pin(equality); x != nil {
			return x
		}
		*formulas = append(*formulas, equality)
	}
	zero, err := c.manager.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	if err = pin(zero); err != nil {
		return err
	}
	for index := 0; index < count; index++ {
		operator := closureArithGT
		if c.isClosure {
			operator = closureArithGE
		}
		bound, x := c.arithmeticApp(operator, c.sigmas[index], zero)
		if x != nil {
			return x
		}
		if x = pin(bound); x != nil {
			return x
		}
		*formulas = append(*formulas, bound)
	}
	one, err := c.manager.ArithmeticRealNumeralText("1")
	if err != nil {
		return err
	}
	if err = pin(one); err != nil {
		return err
	}
	sigmaSum, err := c.arithmeticApp(closureArithAdd, c.sigmas[:count]...)
	if err != nil {
		return err
	}
	if err = pin(sigmaSum); err != nil {
		return err
	}
	normalization, err := c.manager.MkEq(one, sigmaSum)
	if err != nil {
		return err
	}
	if err = pin(normalization); err != nil {
		return err
	}
	*formulas = append(*formulas, normalization)
	return nil
}

func (c *convexClosure) arithmeticRelation(expression ast.ASTHandle) (operator int32, left, right ast.ASTHandle, matched bool, err error) {
	kind, err := c.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	declaration, err := c.manager.GetDecl(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	family, err := c.manager.GetFamilyID(declaration)
	if err != nil || family != c.arithmetic {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	operator, err = c.manager.GetDeclKind(declaration)
	if err != nil || operator < closureArithLE || operator > closureArithGT {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	arguments, err := c.manager.AppArgumentView(expression)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	if arguments.Count() != 2 {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	left, err = arguments.At(0)
	if err != nil {
		return 0, ast.ASTHandle{}, ast.ASTHandle{}, false, err
	}
	right, err = arguments.At(1)
	return operator, left, right, err == nil, err
}

func (c *convexClosure) closeFormula(expression ast.ASTHandle) (ast.ASTHandle, error) {
	operator, left, right, relation, err := c.arithmeticRelation(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if relation {
		switch operator {
		case closureArithLT:
			return c.manager.MkArithmeticLE(left, right)
		case closureArithGT:
			return c.manager.MkArithmeticGE(left, right)
		case closureArithLE, closureArithGE:
			return expression, nil
		}
	}
	negated, inner, err := c.manager.IsNot(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if negated {
		innerOperator, innerLeft, innerRight, innerRelation, x := c.arithmeticRelation(inner)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if innerRelation {
			switch innerOperator {
			case closureArithGE:
				return c.manager.MkArithmeticLE(innerLeft, innerRight)
			case closureArithLE:
				return c.manager.MkArithmeticGE(innerLeft, innerRight)
			case closureArithGT, closureArithLT:
				return expression, nil
			}
		}
	}
	kind, err := c.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if kind != ast.ASTApp {
		return c.manager.MkTrue()
	}
	family, err := c.manager.GetFamilyID(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	operator, err = c.manager.GetDeclKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if family == c.manager.BasicFamilyID() && operator == ast.ASTOpEq {
		return expression, nil
	}
	return c.manager.MkTrue()
}

func (c *convexClosure) closeConjunction(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	conjuncts, err := c.manager.FlattenAnd([]ast.ASTHandle{expression})
	if err != nil {
		return ast.ASTRef{}, err
	}
	references := make([]ast.ASTRef, 0, len(conjuncts))
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, references)) }()
	for index := range conjuncts {
		conjuncts[index], err = c.closeFormula(conjuncts[index])
		if err != nil {
			return ast.ASTRef{}, err
		}
		reference, x := c.manager.NewRef(conjuncts[index])
		if x != nil {
			return ast.ASTRef{}, x
		}
		references = append(references, reference)
	}
	result, err := c.manager.MkAndArray(conjuncts)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(result)
}

func (c *convexClosure) relax(index int, formula ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	scaled, err := scaleClosureExpression(c.manager, formula, c.sigmas[index], c.variables[index])
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.manager.DelRef(&scaled)) }()
	handle, err := c.manager.RefGet(scaled)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.closeConjunction(handle)
}

// Apply returns an owning reference to the convex closure or interior. The
// caller must release it, and it remains valid after Close releases the
// builder's sigma and copied-variable roots.
func (c *convexClosure) Apply(regions []ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if err := c.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if len(regions) == 0 {
		falsity, err := c.manager.MkFalse()
		if err != nil {
			return ast.ASTRef{}, err
		}
		return c.manager.NewRef(falsity)
	}
	if len(regions) == 1 {
		return c.manager.NewRef(regions[0])
	}
	formulas := make([]ast.ASTHandle, 0, len(regions)*2+1)
	workspace := make([]ast.ASTRef, 0, len(regions)*3+4)
	defer func() { resultErr = errors.Join(resultErr, closeASTRefs(c.manager, workspace)) }()
	if err := c.addVariables(len(regions), &formulas, &workspace); err != nil {
		return ast.ASTRef{}, err
	}
	for index, region := range regions {
		relaxed, err := c.relax(index, region)
		if err != nil {
			return ast.ASTRef{}, err
		}
		handle, err := c.manager.RefGet(relaxed)
		if err != nil {
			_ = c.manager.DelRef(&relaxed)
			return ast.ASTRef{}, err
		}
		workspace = append(workspace, relaxed)
		formulas = append(formulas, handle)
	}
	result, err := c.manager.MkAndArray(formulas)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(result)
}

func (c *convexClosure) Close() (result error) {
	if c == nil || c.manager == nil {
		return ErrPredicateTransformerOwner
	}
	if c.closed {
		return nil
	}
	c.closed = true
	result = closeASTRefs(c.manager, c.references)
	c.references = nil
	c.sigmas = nil
	c.variables = nil
	c.transformer = nil
	return result
}
