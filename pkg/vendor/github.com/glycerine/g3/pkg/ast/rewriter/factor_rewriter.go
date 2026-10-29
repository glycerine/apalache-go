package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp;
// src/ast/rewriter/factor_rewriter.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

const (
	factorOpLE     int32 = 2
	factorOpGE     int32 = 3
	factorOpLT     int32 = 4
	factorOpGT     int32 = 5
	factorOpAdd    int32 = 6
	factorOpSub    int32 = 7
	factorOpUMinus int32 = 8
	factorOpMul    int32 = 9
)

type factorTerm struct {
	expr     ast.ASTHandle
	positive bool
}

type factorPower struct {
	expr  ast.ASTHandle
	count uint32
}

type factorCall struct {
	owner   *FactorRewriter
	refs    []ast.ASTRef
	adds    []factorTerm
	muls    [][]ast.ASTHandle
	factors []ast.ASTHandle
	powers  []factorPower
}

func (c *factorCall) close() (result error) {
	for i := len(c.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, c.owner.manager.DelRef(&c.refs[i]))
	}
	return result
}

func (c *factorCall) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return h, err
}

func (c *factorCall) app(h ast.ASTHandle, op int32) (bool, error) {
	k, err := c.owner.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := c.owner.manager.GetFamilyID(h)
	if err != nil || f != c.owner.arithFamily {
		return false, err
	}
	d, err := c.owner.manager.GetDeclKind(h)
	return d == op, err
}

func (c *factorCall) args(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := c.owner.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	r := make([]ast.ASTHandle, n)
	for i := range r {
		r[i], err = c.owner.manager.GetArg(h, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (c *factorCall) numeral(h ast.ASTHandle) (text string, integer, matched bool, err error) {
	return c.owner.manager.ArithmeticNumeralText(h)
}

func (c *factorCall) numeralFor(expr ast.ASTHandle, value int64) (ast.ASTHandle, error) {
	sort, err := c.owner.manager.GetSort(expr)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	kind, err := c.owner.manager.GetDeclKind(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	service := c.owner.manager.NumeralService()
	q, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer service.Del(&q)
	if err = service.SetInt64(&q, value); err != nil {
		return ast.ASTHandle{}, err
	}
	h, err := c.owner.manager.ArithmeticNumeral(&q, kind == 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (c *factorCall) negateNumeral(h ast.ASTHandle) (ast.ASTHandle, error) {
	text, integer, matched, err := c.numeral(h)
	if err != nil || !matched {
		return ast.ASTHandle{}, err
	}
	s := c.owner.manager.NumeralService()
	q, err := s.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer s.Del(&q)
	if err = s.SetString(&q, text); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.NegAssign(&q); err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.owner.manager.ArithmeticNumeral(&q, integer)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(r)
}

func (c *factorCall) mkArith(op int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	h, err := c.owner.manager.MkFamilyApp(c.owner.arithFamily, op, nil, args)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (c *factorCall) mkBasic(makeNode func() (ast.ASTHandle, error)) (ast.ASTHandle, error) {
	h, err := makeNode()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(h)
}

func (c *factorCall) makeAdds(lhs, rhs ast.ASTHandle) error {
	c.adds = append(c.adds[:0], factorTerm{lhs, true}, factorTerm{rhs, false})
	for i := 0; i < len(c.adds); {
		term := &c.adds[i]
		isAdd, err := c.app(term.expr, factorOpAdd)
		if err != nil {
			return err
		}
		isSub, err := c.app(term.expr, factorOpSub)
		if err != nil {
			return err
		}
		isNeg, err := c.app(term.expr, factorOpUMinus)
		if err != nil {
			return err
		}
		switch {
		case isAdd || isSub:
			children, err := c.args(term.expr)
			if err != nil {
				return err
			}
			if len(children) == 0 {
				i++
				continue
			}
			sign := term.positive
			term.expr = children[0]
			for j := 1; j < len(children); j++ {
				childSign := sign
				if isSub {
					childSign = !sign
				}
				c.adds = append(c.adds, factorTerm{children[j], childSign})
			}
		case isNeg:
			child, err := c.owner.manager.GetArg(term.expr, 0)
			if err != nil {
				return err
			}
			term.expr, term.positive = child, !term.positive
		default:
			text, _, numeral, err := c.numeral(term.expr)
			if err != nil {
				return err
			}
			if numeral && text == "0" {
				c.adds[i] = c.adds[len(c.adds)-1]
				c.adds = c.adds[:len(c.adds)-1]
			} else {
				i++
			}
		}
	}
	return nil
}

func (c *factorCall) expandMuls(row []ast.ASTHandle) ([]ast.ASTHandle, error) {
	for i := 0; i < len(row); {
		mul, err := c.app(row[i], factorOpMul)
		if err != nil {
			return nil, err
		}
		if !mul {
			i++
			continue
		}
		children, err := c.args(row[i])
		if err != nil {
			return nil, err
		}
		if len(children) == 0 {
			i++
			continue
		}
		row[i] = children[0]
		row = append(row, children[1:]...)
	}
	return row, nil
}

func (c *factorCall) makeMuls() error {
	c.muls = c.muls[:0]
	for i := 0; i < len(c.adds); {
		row, err := c.expandMuls([]ast.ASTHandle{c.adds[i].expr})
		if err != nil {
			return err
		}
		if len(row) == 0 {
			c.adds = append(c.adds[:i], c.adds[i+1:]...)
			continue
		}
		c.muls = append(c.muls, row)
		i++
	}
	return nil
}

func (c *factorCall) collectPowers() {
	c.powers = c.powers[:0]
	positions := make(map[ast.ASTHandle]int, len(c.factors))
	for _, h := range c.factors {
		if i, ok := positions[h]; ok {
			c.powers[i].count++
		} else {
			positions[h] = len(c.powers)
			c.powers = append(c.powers, factorPower{h, 1})
		}
	}
}

func removeFactor(row []ast.ASTHandle, i int) []ast.ASTHandle {
	return append(row[:i], row[i+1:]...)
}

func (c *factorCall) extractFactors() (bool, error) {
	c.factors = c.factors[:0]
	if len(c.muls) == 1 {
		if len(c.muls[0]) <= 1 {
			return false, nil
		}
		c.factors = append(c.factors, c.muls[0]...)
		if !c.adds[0].positive {
			found := false
			for i, h := range c.factors {
				_, _, numeral, err := c.numeral(h)
				if err != nil {
					return false, err
				}
				if numeral {
					c.factors[i], err = c.negateNumeral(h)
					if err != nil {
						return false, err
					}
					found = true
					break
				}
			}
			if !found {
				minusOne, err := c.numeralFor(c.muls[0][0], -1)
				if err != nil {
					return false, err
				}
				c.factors = append(c.factors, minusOne)
			}
		}
		c.collectPowers()
		return true, nil
	}
	for i := 0; i < len(c.muls[0]); {
		candidate := c.muls[0][i]
		positions := make([]int, len(c.muls))
		positions[0] = i
		ok := true
		for j := 1; j < len(c.muls); j++ {
			positions[j] = -1
			for k, h := range c.muls[j] {
				if h == candidate {
					positions[j] = k
					break
				}
			}
			if positions[j] < 0 {
				ok = false
				break
			}
		}
		if !ok {
			i++
			continue
		}
		c.factors = append(c.factors, candidate)
		for j, p := range positions {
			c.muls[j] = removeFactor(c.muls[j], p)
		}
	}
	if len(c.factors) == 0 {
		return false, nil
	}
	trail := make([]ast.ASTHandle, 0, len(c.adds))
	for i, term := range c.adds {
		var h ast.ASTHandle
		var err error
		switch len(c.muls[i]) {
		case 0:
			h, err = c.numeralFor(c.factors[0], 1)
		case 1:
			h = c.muls[i][0]
		default:
			h, err = c.mkArith(factorOpMul, c.muls[i]...)
		}
		if err != nil {
			return false, err
		}
		if !term.positive {
			h, err = c.mkArith(factorOpUMinus, h)
			if err != nil {
				return false, err
			}
		}
		trail = append(trail, h)
	}
	if len(trail) == 1 {
		c.factors = append(c.factors, trail[0])
	} else if len(trail) > 1 {
		h, err := c.mkArith(factorOpAdd, trail...)
		if err != nil {
			return false, err
		}
		c.factors = append(c.factors, h)
	}
	c.collectPowers()
	return true, nil
}

func (c *factorCall) isNegative() (ast.ASTHandle, []ast.ASTHandle, error) {
	zero, err := c.numeralFor(c.powers[0].expr, 0)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	pos, err := c.mkBasic(c.owner.manager.MkTrue)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	neg, err := c.mkBasic(c.owner.manager.MkFalse)
	if err != nil {
		return ast.ASTHandle{}, nil, err
	}
	eqs := make([]ast.ASTHandle, 0, len(c.powers))
	for _, power := range c.powers {
		eq, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkEq(zero, power.expr) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		eqs = append(eqs, eq)
		if power.count&1 == 0 {
			continue
		}
		positive, err := c.mkArith(factorOpLT, zero, power.expr)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		negative, err := c.mkArith(factorOpLT, power.expr, zero)
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		if c.owner.manager.IsFalse(neg) {
			pos, neg = positive, negative
			continue
		}
		pp, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkAnd(positive, pos) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		nn, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkAnd(negative, neg) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		nextPos, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkOr(pp, nn) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		np, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkAnd(negative, pos) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		pn, err := c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkAnd(positive, neg) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		neg, err = c.mkBasic(func() (ast.ASTHandle, error) { return c.owner.manager.MkOr(np, pn) })
		if err != nil {
			return ast.ASTHandle{}, nil, err
		}
		pos = nextPos
	}
	return neg, eqs, nil
}

func (c *factorCall) rewriteRelation(op int32, lhs, rhs ast.ASTHandle) (ast.ASTHandle, RewriteStatus, error) {
	if err := c.makeAdds(lhs, rhs); err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if err := c.makeMuls(); err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if len(c.muls) == 0 {
		if op == factorOpLT {
			h, err := c.owner.manager.MkFalse()
			return h, RewriteDone, err
		}
		h, err := c.owner.manager.MkTrue()
		return h, RewriteDone, err
	}
	ok, err := c.extractFactors()
	if err != nil || !ok {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if op == -1 {
		eqs := make([]ast.ASTHandle, 0, len(c.powers))
		for _, power := range c.powers {
			zero, err := c.numeralFor(power.expr, 0)
			if err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			eq, err := c.owner.manager.MkEq(power.expr, zero)
			if err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			if _, err = c.hold(eq); err != nil {
				return ast.ASTHandle{}, RewriteFailed, err
			}
			eqs = append(eqs, eq)
		}
		h, err := c.owner.manager.MkOrArray(eqs)
		return h, RewriteDone, err
	}
	negative, eqs, err := c.isNegative()
	if err != nil {
		return ast.ASTHandle{}, RewriteFailed, err
	}
	if op == factorOpLE {
		eqs = append(eqs, negative)
		h, err := c.owner.manager.MkOrArray(eqs)
		return h, RewriteDone, err
	}
	for i, eq := range eqs {
		eqs[i], err = c.owner.manager.MkNot(eq)
		if err != nil {
			return ast.ASTHandle{}, RewriteFailed, err
		}
		if _, err = c.hold(eqs[i]); err != nil {
			return ast.ASTHandle{}, RewriteFailed, err
		}
	}
	eqs = append(eqs, negative)
	h, err := c.owner.manager.MkAndArray(eqs)
	return h, RewriteDone, err
}

type factorRewriterConfiguration struct {
	DefaultConfiguration
	owner *FactorRewriter
}

func (c *factorRewriterConfiguration) RewritePatterns() bool                 { return false }
func (c *factorRewriterConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *factorRewriterConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return RewriteFailed, err
	}
	return c.owner.RewriteApp(decl, args, out)
}

// FactorRewriter ports factor_rewriter and its retained no-proof traversal.
type FactorRewriter struct {
	self        *FactorRewriter
	manager     *ast.ASTManager
	arithFamily ast.FamilyID
	config      factorRewriterConfiguration
	traversal   *Traversal
	closed      bool
}

func NewFactorRewriter(m *ast.ASTManager) (*FactorRewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	r := &FactorRewriter{manager: m, arithFamily: family}
	r.self, r.config.owner = r, r
	r.traversal, err = NewTraversal(m, &r.config)
	if err != nil {
		if r.traversal != nil {
			err = errors.Join(err, r.traversal.Close())
		}
		return r, err
	}
	return r, nil
}

func (r *FactorRewriter) open() error {
	if r == nil || r.self != r || r.closed || r.manager == nil || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return nil
}

// RewriteApp applies the source factor_rewriter core to a declaration and arguments.
func (r *FactorRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status RewriteStatus, result error) {
	if err := r.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	op, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family == r.manager.BasicFamilyID() && op == ast.ASTOpEq {
		op = -1
	} else if family != r.arithFamily || op < factorOpLE || op > factorOpGT {
		return RewriteFailed, nil
	}
	if ast.ASTSourceDebug && args.Count() != 2 {
		return RewriteFailed, ast.ErrASTDomain
	}
	lhs, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	rhs, err := args.At(1)
	if err != nil {
		return RewriteFailed, err
	}
	if op == -1 {
		sort, err := r.manager.GetSort(lhs)
		if err != nil {
			return RewriteFailed, err
		}
		f, err := r.manager.GetFamilyID(sort)
		if err != nil || f != r.arithFamily {
			return RewriteFailed, err
		}
	}
	if op == factorOpGE {
		lhs, rhs, op = rhs, lhs, factorOpLE
	} else if op == factorOpGT {
		lhs, rhs, op = rhs, lhs, factorOpLT
	}
	call := factorCall{owner: r}
	defer func() { result = errors.Join(result, call.close()) }()
	h, status, err := call.rewriteRelation(op, lhs, rhs)
	if err != nil || status == RewriteFailed {
		return status, err
	}
	return status, r.manager.SetRef(out, h)
}

// Rewrite factors supported relations throughout root.
func (r *FactorRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

func (r *FactorRewriter) Close() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
	}
	r.closed = true
	return nil
}
