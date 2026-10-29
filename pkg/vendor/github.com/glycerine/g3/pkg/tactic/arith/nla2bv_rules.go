package arith

// Source: src/tactic/arith/nla2bv_tactic.cpp:84-382.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

func (c *nla2BVInvocation) appendOwned(dst *[]ast.ASTHandle, h ast.ASTHandle) error {
	if c.local.err != nil {
		return c.local.err
	}
	if err := c.t.manager.IncRef(h); err != nil {
		return err
	}
	*dst = append(*dst, h)
	return nil
}
func (c *nla2BVInvocation) run(g *tactic.Goal, out *tactic.TacticResult) (err error) {
	m := c.t.manager
	c.filter, err = tactic.NewFilterModel(m)
	if err != nil {
		return err
	}
	if err = c.filterOwner.Prepare(m); err != nil {
		return err
	}
	if err = c.filterOwner.SetModelConverter(c.filter); err != nil {
		return errors.Join(err, c.filter.Close())
	}
	if err = c.bounds.CollectGoal(g); err != nil {
		return err
	}
	if err = c.collectPower2(g); err != nil {
		return err
	}
	ok, err := c.collectVars(g)
	if err != nil {
		return err
	}
	if !ok {
		return tactic.NewTacticException("goal is not in the fragment supported by nla2bv")
	}
	c.report, err = tactic.NewTacticReport("nla->bv", g)
	if err != nil {
		return err
	}
	defer func() {
		if c.report != nil {
			closeErr := c.report.Close()
			if closeErr == nil {
				c.report = nil
			}
			err = errors.Join(err, closeErr)
		}
	}()
	if err = c.substitute(g); err != nil {
		return err
	}
	if err = c.reduceInteger(g); err != nil {
		return err
	}
	if err = c.reduceReal(g); err != nil {
		return err
	}
	c.extension, err = tactic.NewExtensionModel(m, c.t.services)
	if err != nil {
		return err
	}
	if err = c.extensionOwner.Prepare(m); err != nil {
		return err
	}
	if err = c.extensionOwner.SetModelConverter(c.extension); err != nil {
		return errors.Join(err, c.extension.Close())
	}
	c.looseConcat, err = tactic.ConcatModels(m.Budget(), c.filter, c.extension)
	if err != nil {
		return err
	}
	if err = out.SetModelConverter(c.looseConcat); err != nil {
		return err
	}
	c.looseConcat = nil
	for i, decl := range c.vars {
		if err = c.extension.Insert(decl, c.defs[i]); err != nil {
			return err
		}
	}
	decls, err := c.real.AuxDecls()
	if err != nil {
		return err
	}
	for _, decl := range decls {
		if err = c.filter.Insert(decl); err != nil {
			return err
		}
	}
	if err = g.IncDepth(); err != nil {
		return err
	}
	if !c.satPreserving {
		return g.UpdatePrecision(tactic.GoalUnder)
	}
	return nil
}
func (c *nla2BVInvocation) collectPower2(g *tactic.Goal) error {
	if err := c.integer.CollectPower2(g); err != nil {
		return err
	}
	powers, err := c.integer.Power2()
	if err != nil {
		return err
	}
	if len(powers) == 0 {
		return nil
	}
	for key, exponent := range powers {
		width, err := c.t.manager.BitVectorSize(exponent)
		if err != nil {
			return err
		}
		bits := c.local.bv(45, c.local.numeral(big.NewRat(1, 1), width), exponent)
		value := c.local.hold(c.t.manager.MkBitVectorToInt(bits))
		if err = c.appendOwned(&c.trail, value); err != nil {
			return err
		}
		if err = c.t.manager.ExprSubstitutionInsert(c.sub, key, value, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	if err = c.substitute(g); err != nil {
		return err
	}
	return c.t.manager.ExprSubstitutionReset(c.sub)
}
func (c *nla2BVInvocation) substitute(g *tactic.Goal) error {
	var err error
	c.replacer, err = rewriter.NewDefaultExprReplacer(c.t.manager)
	if err != nil {
		return err
	}
	if err = c.replacer.SetSubstitution(&c.sub); err != nil {
		return err
	}
	out, err := c.t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.local.refs = append(c.local.refs, out)
	for i := uint32(0); ; i++ {
		n, err := g.Size()
		if err != nil {
			return err
		}
		if i >= n {
			break
		}
		h, err := g.Form(i)
		if err != nil {
			return err
		}
		if err = c.replacer.Rewrite(h, out); err != nil {
			return err
		}
		value, err := c.t.manager.RefGet(out)
		if err != nil {
			return err
		}
		if err = g.Update(i, value, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	if err = c.replacer.Close(); err != nil {
		return err
	}
	c.replacer = nil
	return nil
}
func (c *nla2BVInvocation) reduceInteger(g *tactic.Goal) error {
	var err error
	c.intRewrite, err = NewBV2IntRewriter(c.integer)
	if err != nil {
		return err
	}
	out, err := c.t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.local.refs = append(c.local.refs, out)
	for i := uint32(0); ; i++ {
		n, err := g.Size()
		if err != nil {
			return err
		}
		if i >= n {
			break
		}
		h, err := g.Form(i)
		if err != nil {
			return err
		}
		if err = c.intRewrite.Rewrite(h, out); err != nil {
			return err
		}
		value, err := c.t.manager.RefGet(out)
		if err != nil {
			return err
		}
		if err = g.Update(i, value, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	side, err := c.integer.SideConditions()
	if err != nil {
		return err
	}
	if err = c.assertSide(g, side); err != nil {
		return err
	}
	if err = c.intRewrite.Close(); err != nil {
		return err
	}
	c.intRewrite = nil
	return nil
}
func (c *nla2BVInvocation) reduceReal(g *tactic.Goal) error {
	var err error
	c.realRewrite, err = NewBV2RealRewriter(c.real)
	if err != nil {
		return err
	}
	out, err := c.t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.local.refs = append(c.local.refs, out)
	for i := uint32(0); ; i++ {
		n, err := g.Size()
		if err != nil {
			return err
		}
		if i >= n {
			break
		}
		h, err := g.Form(i)
		if err != nil {
			return err
		}
		if err = c.realRewrite.Rewrite(h, out); err != nil {
			return err
		}
		value, err := c.t.manager.RefGet(out)
		if err != nil {
			return err
		}
		contains, err := c.real.ContainsBV2Real(value)
		if err != nil {
			return err
		}
		if contains {
			return tactic.NewTacticException("nla2bv could not eliminate reals")
		}
		if err = g.Update(i, value, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
			return err
		}
	}
	side, err := c.real.SideConditions()
	if err != nil {
		return err
	}
	if err = c.assertSide(g, side); err != nil {
		return err
	}
	if err = c.realRewrite.Close(); err != nil {
		return err
	}
	c.realRewrite = nil
	return nil
}
func (c *nla2BVInvocation) assertSide(g *tactic.Goal, side []ast.ASTHandle) error {
	for _, h := range side {
		if err := g.Assert(h); err != nil {
			return err
		}
		c.satPreserving = false
	}
	return nil
}

// The source routine returns ceil(log2(n)), with a minimum result of one.
func nla2BVLog2(n *big.Rat) uint32 {
	pow, two := big.NewRat(1, 1), big.NewRat(2, 1)
	var bits uint32
	for pow.Cmp(n) < 0 {
		bits++
		pow.Mul(pow, two)
	}
	if bits == 0 {
		bits = 1
	}
	return bits
}
func (c *nla2BVInvocation) fresh(variable ast.ASTHandle, suffix string, width uint32) (ast.ASTHandle, error) {
	m := c.t.manager
	sort, err := m.BitVectorSort(width)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := m.GetDecl(variable)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	name, err := m.GetASTName(decl)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := m.Symbols.String(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	h := c.local.hold(m.MkFreshConst(text+suffix, sort))
	if c.local.err != nil {
		return ast.ASTHandle{}, c.local.err
	}
	decl, err = m.GetDecl(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return h, c.filter.Insert(decl)
}
func (c *nla2BVInvocation) addDefinition(variable, definition ast.ASTHandle) error {
	if err := c.appendOwned(&c.trail, definition); err != nil {
		return err
	}
	m := c.t.manager
	if err := m.ExprSubstitutionInsert(c.sub, variable, definition, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	decl, err := m.GetDecl(variable)
	if err != nil {
		return err
	}
	if err = c.appendOwned(&c.vars, decl); err != nil {
		return err
	}
	return c.appendOwned(&c.defs, definition)
}
func (c *nla2BVInvocation) addInteger(variable ast.ASTHandle) error {
	low, strict, hasLow, err := c.bounds.Lower(variable)
	if err != nil {
		return err
	}
	if hasLow {
		if err = nla2BVAssert(214, "!is_strict", !strict); err != nil {
			return err
		}
	}
	up, strict, hasUp, err := c.bounds.Upper(variable)
	if err != nil {
		return err
	}
	if hasUp {
		if err = nla2BVAssert(218, "!is_strict", !strict); err != nil {
			return err
		}
	}
	bits := c.numBits
	if hasLow && hasUp {
		span := new(big.Rat).Sub(&up, &low)
		span.Abs(span)
		span.Add(span, big.NewRat(1, 1))
		bits = nla2BVLog2(span)
	} else {
		c.satPreserving = false
	}
	fresh, err := c.fresh(variable, "", bits)
	if err != nil {
		return err
	}
	value := c.local.hold(c.t.manager.MkBitVectorToInt(fresh))
	if hasLow {
		if low.Sign() != 0 {
			value = c.local.arith(6, value, c.local.hold(c.t.manager.ArithmeticNumeralRat(&low, true)))
		}
	} else if hasUp {
		value = c.local.arith(7, c.local.hold(c.t.manager.ArithmeticNumeralRat(&up, true)), value)
	} else {
		mid := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		value = c.local.arith(7, value, c.local.hold(c.t.manager.ArithmeticNumeralRat(new(big.Rat).SetInt(mid), true)))
	}
	if c.local.err != nil {
		return c.local.err
	}
	return c.addDefinition(variable, value)
}
func (c *nla2BVInvocation) addReal(variable ast.ASTHandle) error {
	c.satPreserving = false
	s, err := c.fresh(variable, "", c.numBits)
	if err != nil {
		return err
	}
	t, err := c.fresh(variable, "_r", c.numBits)
	if err != nil {
		return err
	}
	encoded := c.local.encoding(s, t, &c.real.defaultDivisor, &c.real.defaultRoot)
	if err = c.appendOwned(&c.trail, encoded); err != nil {
		return err
	}
	if err = c.t.manager.ExprSubstitutionInsert(c.sub, variable, encoded, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	decl, err := c.t.manager.GetDecl(variable)
	if err != nil {
		return err
	}
	if err = c.appendOwned(&c.vars, decl); err != nil {
		return err
	}
	reduced := c.local.reduced(s, t, &c.real.defaultDivisor, &c.real.defaultRoot)
	return c.appendOwned(&c.defs, reduced)
}

func (c *nla2BVInvocation) collectVars(g *tactic.Goal) (bool, error) {
	terms, err := c.postorder(g)
	if err != nil {
		return false, err
	}
	m := c.t.manager
	var variables []ast.ASTHandle
	supported := true
	for _, h := range terms {
		kind, err := m.GetKind(h)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			supported = false
			continue
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		op, err := m.GetDeclKind(h)
		if err != nil {
			return false, err
		}
		n, err := m.GetNumArgs(h)
		if err != nil {
			return false, err
		}
		sort, err := m.GetSort(h)
		if err != nil {
			return false, err
		}
		sf, err := m.GetFamilyID(sort)
		if err != nil {
			return false, err
		}
		sk, err := m.GetDeclKind(sort)
		if err != nil {
			return false, err
		}
		decl, err := m.GetDecl(h)
		if err != nil {
			return false, err
		}
		lt, err := c.real.PositiveLTDecl()
		if err != nil {
			return false, err
		}
		le, err := c.real.PositiveLEDecl()
		if err != nil {
			return false, err
		}
		if family == ast.NullFamilyID && n == 0 && sf == c.real.arith && (sk == 0 || sk == 1) {
			variables = append(variables, h)
		} else if family == ast.NullFamilyID && n == 0 && sf == m.BasicFamilyID() {
			// Uninterpreted Boolean constants are retained.
		} else if !(family == c.real.arith && (op == 0 || op >= 2 && op <= 9) || decl == lt || decl == le || family == m.BasicFamilyID()) {
			supported = false
		}
		text, integer, numeral, err := m.ArithmeticNumeralText(h)
		if err != nil {
			return false, err
		}
		if numeral && integer {
			n, ok := new(big.Rat).SetString(text)
			if !ok {
				return false, ast.ErrASTDomain
			}
			n.Abs(n)
			bits := nla2BVLog2(n)
			if c.numBits <= bits {
				c.numBits = bits + 1
			}
		}
	}
	// Source builds substitutions even when the completed traversal discovered
	// an unsupported node. Refusal occurs after all collected variables.
	for _, v := range variables {
		sort, err := m.GetSort(v)
		if err != nil {
			return false, err
		}
		kind, err := m.GetDeclKind(sort)
		if err != nil {
			return false, err
		}
		if kind == 1 {
			err = c.addInteger(v)
		} else {
			err = nla2BVAssert(202, "m_arith.is_real(n)", kind == 0)
			if err == nil {
				err = c.addReal(v)
			}
		}
		if err != nil {
			return false, err
		}
	}
	return supported && len(variables) != 0, nil
}

// This concrete goal visitor marks all nodes across all roots and visits
// arguments, quantifier bodies and annotations before their parent.
func (c *nla2BVInvocation) postorder(g *tactic.Goal) ([]ast.ASTHandle, error) {
	type frame struct {
		h    ast.ASTHandle
		next uint32
	}
	m := c.t.manager
	seen := map[ast.ASTHandle]bool{}
	var result []ast.ASTHandle
	n, err := g.Size()
	if err != nil {
		return nil, err
	}
	for i := uint32(0); i < n; i++ {
		root, err := g.Form(i)
		if err != nil {
			return nil, err
		}
		if seen[root] {
			continue
		}
		seen[root] = true
		stack := []frame{{h: root}}
		for len(stack) > 0 {
			last := len(stack) - 1
			f := &stack[last]
			kind, err := m.GetKind(f.h)
			if err != nil {
				return nil, err
			}
			var count, patterns uint32
			switch kind {
			case ast.ASTApp:
				count, err = m.GetNumArgs(f.h)
			case ast.ASTQuantifier:
				patterns, err = m.GetNumPatterns(f.h)
				if err == nil {
					count, err = m.GetNumNoPatterns(f.h)
				}
				count += patterns + 1
			case ast.ASTVar:
			default:
				return nil, ast.ErrASTDomain
			}
			if err != nil {
				return nil, err
			}
			if f.next < count {
				index := f.next
				f.next++
				var child ast.ASTHandle
				if kind == ast.ASTApp {
					child, err = m.GetArg(f.h, index)
				} else if index == 0 {
					child, err = m.GetQuantifierBody(f.h)
				} else if index <= patterns {
					child, err = m.GetPattern(f.h, index-1)
				} else {
					child, err = m.GetNoPattern(f.h, index-1-patterns)
				}
				if err != nil {
					return nil, err
				}
				if !seen[child] {
					seen[child] = true
					stack = append(stack, frame{h: child})
				}
				continue
			}
			result = append(result, f.h)
			stack = stack[:last]
		}
	}
	return result, nil
}
