package smtcore

// Source: src/smt/theory_lra.cpp:1435,2129,2229,2285-2327.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// A fresh source context borrows parent fparams and copies current params.
// Raw internalizers have no parameter owner, so their default owners are local
// to this concrete context. Every partial constructor remains reachable here.
type smtLRAValidationContext struct {
	manager *ast.ASTManager
	kernel  *SMTKernel
	current ast.ASTRef
	params  *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	catalog *uparams.GlobalParamsCatalog
	full    *SMTParams
}

func (c *smtLRAValidationContext) initialize(t *SMTLRATheory) error {
	s := t.state
	m := s.manager
	c.manager = m
	full := s.smtParams
	var local uparams.ParamsRef
	current := s.contextParams
	var err error
	if full == nil {
		if current != nil {
			return uparams.ErrParamsOwner
		}
		c.params, err = uparams.NewParamsContext(m.Symbols, m.NumeralService(), uparams.ParamsConfig{Budget: m.Budget()})
		if err != nil {
			return err
		}
		c.catalog, err = uparams.NewGlobalParamsCatalog(m.Symbols, uparams.GlobalParamsCatalogConfig{Budget: m.Budget()})
		if err != nil {
			return err
		}
		c.globals, err = uparams.NewGlobalParamsService(m.Symbols, c.params, uparams.GlobalParamsConfig{Budget: m.Budget(), Catalog: c.catalog})
		if err != nil {
			return err
		}
		if err = c.globals.InitializeSource(); err != nil {
			return err
		}
		local = c.params.EmptyValue()
		current = &local
		c.full, err = NewSMTParams(c.params, current, c.globals, m.Budget())
		if err != nil {
			return err
		}
		full = c.full
	} else if current == nil {
		local = full.Context.EmptyValue()
		current = &local
	}
	c.kernel, err = NewSMTKernel(m, nil)
	if err != nil {
		return err
	}
	if err = c.kernel.configurePreprocessingParams(full.Context, full.Globals, current, full, true); err != nil {
		return err
	}
	// The C++ context constructor has no logic argument at these call sites.
	return smtInstallRetainedSetup(c.kernel, m, "")
}
func (c *smtLRAValidationContext) close() error {
	if c.current != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.current); err != nil {
			return err
		}
	}
	if c.kernel != nil {
		if err := c.kernel.Close(); err != nil {
			return err
		}
		c.kernel = nil
	}
	if c.full != nil {
		if err := c.full.Close(); err != nil {
			return err
		}
		c.full = nil
	}
	if c.globals != nil {
		if err := c.globals.Close(); err != nil {
			return err
		}
		c.globals = nil
	}
	if c.catalog != nil {
		if err := c.catalog.Close(); err != nil {
			return err
		}
		c.catalog = nil
	}
	if c.params != nil {
		if err := c.params.Close(); err != nil {
			return err
		}
		c.params = nil
	}
	return nil
}
func (c *smtLRAValidationContext) assert(expression ast.ASTHandle) error {
	var err error
	if c.current == (ast.ASTRef{}) {
		c.current, err = c.manager.NewRef(expression)
	} else {
		err = c.manager.SetRef(c.current, expression)
	}
	if err != nil {
		return err
	}
	if err = c.kernel.Assert(expression, ast.ASTHandle{}); err != nil {
		return err
	}
	return c.manager.DelRef(&c.current)
}
func (t *SMTLRATheory) closeValidation() error {
	if t.validation == nil {
		return nil
	}
	if err := t.validation.close(); err != nil {
		return err
	}
	t.validation = nil
	return nil
}
func (t *SMTLRATheory) dumpValidation(core []SMTLiteral, consequent SMTLiteral) error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	if !t.config.DumpLemmas {
		return nil
	}
	_, err := t.state.DumpLemmaAsSMTProblem(core, nil, consequent, usymbol.Symbol{})
	var dump *SMTLemmaDumpError
	if errors.As(err, &dump) {
		return dump.OperandError
	}
	return err
}

// In the retained immediate path m_eqs is empty: equality/definition constraint
// sources are produced only by the constructor-disabled delayed path.
func (t *SMTLRATheory) validateBackground(core []SMTLiteral, conclusion SMTLiteral, left, right *SMTEnode, traceLine int) (valid bool, resultErr error) {
	if t == nil {
		return false, ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return false, err
	}
	if t.validation != nil {
		return false, ast.ErrASTPending
	}
	c := &smtLRAValidationContext{}
	t.validation = c
	defer func() {
		err := t.closeValidation()
		resultErr = errors.Join(resultErr, err)
		if resultErr != nil {
			valid = false
		}
	}()
	if err := c.initialize(t); err != nil {
		return false, err
	}
	for _, lit := range core {
		expression, err := t.state.literalExpression(lit)
		if err != nil {
			return false, err
		}
		if err = c.assert(expression); err != nil {
			return false, err
		}
	}
	if left != nil || right != nil {
		if left == nil || right == nil || left.manager != t.state.manager || right.manager != t.state.manager || t.state.context.enodes[left.owner] != left || t.state.context.enodes[right.owner] != right {
			return false, ast.ErrASTDomain
		}
		eq, err := c.manager.MkEq(left.owner, right.owner)
		if err != nil {
			return false, err
		}
		c.current, err = c.manager.NewRef(eq)
		if err != nil {
			return false, err
		}
		negated, err := c.manager.MkNot(eq)
		if err != nil {
			return false, err
		}
		if err = c.assert(negated); err != nil {
			return false, err
		}
	}
	result, err := c.kernel.Check()
	if err != nil {
		return false, err
	}
	valid = result != lbool.True // UNKNOWN validates, exactly as l_true != nctx.check().
	if !valid && traceLine != 0 {
		function := "validate_conflict"
		if traceLine == 2306 {
			function = "validate_assign"
		}
		err = t.emitLRA(function, traceLine, func(out io.Writer) error {
			dumpCore := core
			if traceLine == 2306 {
				dumpCore = core[:len(core)-1]
			}
			if err := t.state.DisplayLemmaAsSMTProblem(out, dumpCore, nil, conclusion, usymbol.Symbol{}); err != nil {
				return err
			}
			return t.Display(out)
		})
		if err != nil {
			return false, err
		}
	}
	return valid, nil
}

func (t *SMTLRATheory) validateConflict(core []SMTLiteral) (bool, error) {
	if err := t.dumpValidation(core, SMTFalseLiteral); err != nil {
		return false, err
	}
	return t.validateBackground(core, SMTFalseLiteral, nil, nil, 2292)
}
func (t *SMTLRATheory) validateAssignment(core []SMTLiteral, lit SMTLiteral) (bool, error) {
	if err := t.dumpValidation(core, lit); err != nil {
		return false, err
	}
	premises := append(append([]SMTLiteral(nil), core...), lit.Negated())
	return t.validateBackground(premises, lit, nil, nil, 2306)
}
func (t *SMTLRATheory) validateEquality(core []SMTLiteral, left, right *SMTEnode) (bool, error) {
	if left == nil || right == nil {
		return false, ast.ErrASTDomain
	}
	return t.validateBackground(core, SMTFalseLiteral, left, right, 0)
}

type smtLRAValidationKind uint8

const (
	smtLRAValidateConflict smtLRAValidationKind = iota
	smtLRAValidateAssignment
	smtLRAValidateEquality
)

func (t *SMTLRATheory) checkValidation(kind smtLRAValidationKind, core []SMTLiteral, literal SMTLiteral, left, right *SMTEnode) error {
	if !diagnostic.DebugBuild {
		return nil
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		return err
	}
	var valid bool
	var line int
	var condition string
	switch kind {
	case smtLRAValidateConflict:
		valid, err = t.validateConflict(core)
		line = 2229
		condition = "validate_conflict()"
	case smtLRAValidateAssignment:
		valid, err = t.validateAssignment(core, literal)
		line = 1435
		condition = "validate_assign(lit)"
	case smtLRAValidateEquality:
		valid, err = t.validateEquality(core, left, right)
		line = 2129
		condition = "validate_eq(x, y)"
	default:
		return ast.ErrASTDomain
	}
	if err != nil {
		return err
	}
	if !valid {
		return diagnostic.DebugAssertionFailure(d, "src/smt/theory_lra.cpp", int32(line), condition)
	}
	return nil
}
