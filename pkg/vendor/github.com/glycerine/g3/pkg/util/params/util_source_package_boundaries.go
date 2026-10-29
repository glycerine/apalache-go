package params

// Source value and borrowed-owner boundaries used by AST and parameter consumers.

import (
	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// CStringView borrows the source bare_str storage; numerical symbols follow
// the source debug guard and release empty spelling. No copy or RC is acquired.
func SymbolCStringView(m *usymbol.Manager, s usymbol.Symbol) (ParamsStringView, error) {
	if e := m.Validate(s); e != nil {
		return ParamsStringView{}, e
	}
	if s.IsNumerical() {
		if udiag.DebugBuild {
			return ParamsStringView{}, usymbol.ErrInvalid
		}
		return LiteralParamsStringView(""), nil
	}
	if s.Value == 0 {
		return ParamsStringView{}, usymbol.ErrInvalid
	}
	return ParamsStringView{symbols: m, symbol: s, Mode: 3, PresentField: true}, nil
}
func (c *ParamsContext) CheckServices(s *usymbol.Manager, q *unumeral.RationalService) error {
	if c == nil || c.Self != c || c.SymbolsField != s || q != nil && c.NumeralsField != q {
		return ErrParamsOwner
	}
	return nil
}
func (c *ParamsContext) Symbols() *usymbol.Manager           { return c.SymbolsField }
func (c *ParamsContext) Numerals() *unumeral.RationalService { return c.NumeralsField }
func (c *ParamsContext) Budget() *arena.Budget               { return c.BudgetField }
func (g *GlobalParamsService) CheckOwner(c *ParamsContext, b *arena.Budget) error {
	if g == nil || g.self != g || g.Context != c || g.Config.Budget != b {
		return ErrParamsOwner
	}
	return nil
}
func (g *GlobalParamsService) CheckBudget(b *arena.Budget) error {
	if g == nil || g.self != g || g.Config.Budget != b {
		return ErrGlobalParamsOwner
	}
	return nil
}
func (g *GlobalParamsService) NamespaceVerifyFailure(file string, line int32, expression string) error {
	if g.namespaceAssertions == nil {
		return ErrGlobalParamsAssertion
	}
	return g.namespaceAssertions.VerifyFailure(file, line, expression)
}

// NamespaceAssertionFailureOptional preserves the enabled-default nil binding.
func NamespaceAssertionFailureOptional(g *GlobalParamsService, file string, line int32, expression string) error {
	if g == nil {
		return ErrGlobalParamsAssertion
	}
	return g.NamespaceAssertionFailure(file, line, expression)
}
func (c *ParamsContext) CheckOpen() error         { return c.open() }
func (c *ParamsContext) Check(p *ParamsRef) error { return c.check(p) }
func (g *GlobalParamsService) CheckOpen() error   { return g.open() }

// EmptyValue adapts params_ref::get_empty's static null reference to this
// context's owner. Copying the value allocates no parameter block.
func (c *ParamsContext) EmptyValue() ParamsRef { return ParamsRef{Owner: c.Owner} }

func (r ParamsRef) Initialized() bool { return r.Owner != 0 }

func (g *GlobalParams) CheckOpen() error          { return g.open() }
func (g *GlobalParams) Context() *ParamsContext   { return g.ContextField }
func (g *GlobalParams) Symbols() *usymbol.Manager { return g.SymbolsField }
