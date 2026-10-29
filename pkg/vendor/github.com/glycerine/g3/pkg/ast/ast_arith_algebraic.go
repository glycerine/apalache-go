package ast

// Source: src/ast/arith_decl_plugin.cpp:20-72,99-112.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/util/cooperate"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

// astAlgebraicNumbersWrapper is the concrete lazy m_aw owner. Values are
// indexed by the source id_gen; a recycled slot contains canonical zero.
type astAlgebraicNumbersWrapper struct {
	owner       *astArithmeticPluginOwner
	rationals   *unumeral.RationalManager
	manager     *polynomial.AlgebraicManager
	cooperation *cooperate.Cooperation
	primes      *primegen.Generator
	values      []polynomial.Algebraic
	freeIDs     []uint32
	nextID      uint32
	closed      bool
}

func (p *astArithmeticPluginOwner) algebraicWrapper() (*astAlgebraicNumbersWrapper, error) {
	if p == nil || p.self != p || p.owner == nil || p.owner.arithmeticPlugin != p || !p.published {
		return nil, ErrASTHandle
	}
	if p.algebraic != nil {
		if p.algebraic.closed {
			return nil, arena.ErrClosed
		}
		return p.algebraic, nil
	}
	m := p.owner
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil {
		return nil, uparams.ErrGlobalParamsOwner
	}
	parameters, e := g.SourceGetRef()
	if e != nil {
		return nil, e
	}
	w := &astAlgebraicNumbersWrapper{owner: p}
	w.rationals, e = m.numerals.NewPrivateRationalManager()
	if e != nil {
		return nil, e
	}
	fail := func(cause error) (*astAlgebraicNumbersWrapper, error) {
		_ = w.close()
		return nil, cause
	}
	w.cooperation, e = cooperate.NewCooperation(m.BudgetField)
	if e != nil {
		return fail(e)
	}
	w.primes, e = primegen.NewGenerator(m.BudgetField)
	if e != nil {
		return fail(e)
	}
	w.manager, e = polynomial.NewAlgebraicManager(polynomial.AlgebraicManagerConfig{
		Rationals: m.numerals, Numerals: w.rationals, Limit: &m.Limit,
		Cooperation: w.cooperation, Params: parameters, Globals: g,
		Factorization: polynomial.UPolynomialFactorizationConfig{Primes: w.primes},
	})
	if e != nil {
		return fail(e)
	}
	p.algebraic = w
	return w, nil
}

func (w *astAlgebraicNumbersWrapper) mkID(source *polynomial.AlgebraicRootCore, value *polynomial.Algebraic) (uint32, error) {
	if w == nil || w.closed || w.manager == nil || source == nil || value == nil {
		return 0, ErrASTHandle
	}
	var id uint32
	if n := len(w.freeIDs); n != 0 {
		id = w.freeIDs[n-1]
		w.freeIDs = w.freeIDs[:n-1]
	} else {
		id = w.nextID
		if id == math.MaxUint32 {
			return 0, arena.ErrBounds
		}
		w.nextID++
	}
	if uint64(id)+1 > uint64(^uint(0)>>1) {
		return 0, arena.ErrBounds
	}
	for len(w.values) <= int(id) {
		w.values = append(w.values, polynomial.Algebraic{})
	}
	if e := w.manager.CopyFromCore(&w.values[id], source, value); e != nil {
		return 0, e
	}
	return id, nil
}

func (w *astAlgebraicNumbersWrapper) value(id uint32) (*polynomial.Algebraic, error) {
	if w == nil || w.closed || uint64(id) >= uint64(len(w.values)) {
		return nil, ErrASTHandle
	}
	v := &w.values[id]
	zero, e := w.manager.IsZero(v)
	if e != nil || zero {
		if e != nil {
			return nil, e
		}
		return nil, ErrASTHandle
	}
	return v, nil
}

func (w *astAlgebraicNumbersWrapper) recycle(id uint32) error {
	v, e := w.value(id)
	if e != nil {
		return e
	}
	if e = w.manager.Del(v); e != nil {
		return e
	}
	w.freeIDs = append(w.freeIDs, id)
	return nil
}

func (w *astAlgebraicNumbersWrapper) close() error {
	if w == nil || w.closed {
		return nil
	}
	for i := range w.values {
		if w.manager != nil {
			if e := w.manager.Del(&w.values[i]); e != nil {
				return e
			}
		}
	}
	w.values = nil
	w.freeIDs = nil
	if w.manager != nil {
		if e := w.manager.Close(); e != nil {
			return e
		}
		w.manager = nil
	}
	if w.primes != nil {
		if e := w.primes.Close(); e != nil {
			return e
		}
		w.primes = nil
	}
	if w.cooperation != nil {
		if e := w.cooperation.Close(); e != nil {
			return e
		}
		w.cooperation = nil
	}
	if w.rationals != nil {
		if e := w.rationals.CloseChecked(); e != nil {
			return e
		}
		w.rationals = nil
	}
	w.closed = true
	return nil
}

// ArithmeticAlgebraicManager returns the arithmetic plugin's lazy algebraic
// owner. The returned manager is borrowed and remains valid until AST Close.
func (m *ASTManager) ArithmeticAlgebraicManager() (*polynomial.AlgebraicManager, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if m.arithmeticPlugin == nil {
		return nil, ErrASTHandle
	}
	w, e := m.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return nil, e
	}
	return w.manager, nil
}

// ArithmeticAlgebraicNumeralValue recognizes an irrational arithmetic
// numeral and copies its value into out. The caller owns the copy and must
// release it with the borrowed ArithmeticAlgebraicManager.
func (m *ASTManager) ArithmeticAlgebraicNumeralValue(h ASTHandle, out *polynomial.Algebraic) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if out == nil || m.arithmeticPlugin == nil {
		return false, ErrASTHandle
	}
	r, e := m.requireExpr(h)
	if e != nil {
		return false, e
	}
	value, matched, e := m.arithmeticAlgebraicValue(m.arithmeticPlugin.family, r)
	if e != nil || !matched {
		return false, e
	}
	if e = m.arithmeticPlugin.algebraic.manager.Copy(out, value); e != nil {
		return false, e
	}
	return true, nil
}

// ArithmeticAlgebraicNumeral implements the irrational branch of
// arith_decl_plugin::mk_numeral(anum,false).
func (m *ASTManager) ArithmeticAlgebraicNumeral(value *polynomial.Algebraic) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	p := m.arithmeticPlugin
	if p == nil || value == nil {
		return ASTHandle{}, ErrASTHandle
	}
	w, e := p.algebraicWrapper()
	if e != nil {
		return ASTHandle{}, e
	}
	rational, e := w.manager.IsRational(value)
	if e != nil {
		return ASTHandle{}, e
	}
	if rational {
		return ASTHandle{}, ErrASTDomain
	}
	id, e := w.mkID(w.manager.AlgebraicRootCore, value)
	if e != nil {
		return ASTHandle{}, e
	}
	parameter := astParameterRecord{Kind: ASTParameterExternal, external: id}
	if e = m.beginFormatInfo(false, p.family, 1, &parameter); e != nil {
		return ASTHandle{}, e
	}
	decl, e := m.mkFuncDecl(p.names[2], nil, m.external(p.fields[0]), ASTFuncDeclInfo{m.origin, math.MaxUint32 - 1, m.formatLexical.epoch})
	m.endFormatInfo()
	if e != nil {
		return ASTHandle{}, e
	}
	app, e := m.mkApp(decl, nil)
	return m.external(app), e
}

// TranslateArithmeticExternalParameter implements the arithmetic plugin's
// cross-manager translation callback for an attached external parameter.
func (m *ASTManager) TranslateArithmeticExternalParameter(source ASTParameter, target *ASTManager) (ASTParameter, error) {
	if target == nil || target == m {
		return ASTParameter{}, ErrASTHandle
	}
	if e := m.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer m.mu.Unlock()
	if e := target.lock(); e != nil {
		return ASTParameter{}, e
	}
	defer target.mu.Unlock()
	return m.translateArithmeticExternalParameter(source, target)
}

func (m *ASTManager) translateArithmeticExternalParameter(source ASTParameter, target *ASTManager) (ASTParameter, error) {
	if target == nil || target.Self != target || target == m {
		return ASTParameter{}, ErrASTHandle
	}
	sr, e := m.parameterOfKind(source, ASTParameterExternal)
	if e != nil {
		return ASTParameter{}, e
	}
	sw, e := m.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return ASTParameter{}, e
	}
	v, e := sw.value(sr.external)
	if e != nil {
		return ASTParameter{}, e
	}
	tw, e := target.arithmeticPlugin.algebraicWrapper()
	if e != nil {
		return ASTParameter{}, e
	}
	id, e := tw.mkID(sw.manager.AlgebraicRootCore, v)
	if e != nil {
		return ASTParameter{}, e
	}
	p, e := target.allocParameter()
	if e != nil {
		_ = tw.recycle(id)
		return ASTParameter{}, e
	}
	r := target.parameterAt(p.slot)
	r.Kind, r.external = ASTParameterExternal, id
	return p, nil
}
