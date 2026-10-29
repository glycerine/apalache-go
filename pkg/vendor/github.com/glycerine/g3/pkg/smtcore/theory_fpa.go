package smtcore

// Source: src/smt/theory_fpa.cpp and src/smt/theory_fpa.h floating-point
// reduction to bit-vectors inside the incremental SMT context.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	astfpa "github.com/glycerine/g3/pkg/ast/fpa"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	modelfpa "github.com/glycerine/g3/pkg/model/fpa"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
)

type smtFPAConversion struct {
	key   ast.ASTRef
	value ast.ASTRef
}

type smtFPAValueProcedure struct {
	manager      *ast.ASTManager
	dependencies []SMTModelValueDependency
	ebits        uint32
	sbits        uint32
	rounding     bool
}

func (p *smtFPAValueProcedure) Dependencies() []SMTModelValueDependency {
	return append([]SMTModelValueDependency(nil), p.dependencies...)
}

func (*smtFPAValueProcedure) IsFresh() bool { return false }

func (p *smtFPAValueProcedure) MakeValue(generator *SMTModelGenerator, values []ast.ASTHandle) (ast.ASTHandle, error) {
	if p == nil || p.manager == nil || generator == nil || len(values) != len(p.dependencies) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err := p.traceInputs(generator, values); err != nil {
		return ast.ASTHandle{}, err
	}
	var value ast.ASTHandle
	var err error
	if p.rounding {
		value, err = p.manager.MkFPARoundingModeModelValue(values)
	} else {
		value, err = p.manager.MkFPAModelValue(p.ebits, p.sbits, values)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = p.traceResult(generator, values, value); err != nil {
		return ast.ASTHandle{}, err
	}
	return value, nil
}

// SMTFloatingPointTheory preserves FP and rounding-mode terms in the e-graph
// and defines every reached atom through the retained FPA-to-BV converter.
type SMTFloatingPointTheory struct {
	*SMTTheoryBase
	converter  *astfpa.Converter
	rewriter   *rewriter.FPA2BVRewriter
	converted  map[ast.ASTHandle]ast.ASTHandle
	conversion []smtFPAConversion
	scopes     []int
}

func NewSMTFloatingPointTheory(family ast.FamilyID) (*SMTFloatingPointTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTFloatingPointTheory{
		SMTTheoryBase: base,
		converted:     make(map[ast.ASTHandle]ast.ASTHandle),
	}, nil
}

func (t *SMTFloatingPointTheory) Initialize(state *SMTInternalizerState) (result error) {
	if t == nil || t.converter != nil || t.rewriter != nil {
		return ast.ErrASTHandle
	}
	if err := t.SMTTheoryBase.Initialize(state); err != nil {
		return err
	}
	converter, err := astfpa.NewConverter(state.manager)
	if err != nil {
		return fmt.Errorf("floating-point converter construction: %w", err)
	}
	t.converter = converter
	if err = converter.SetConstantFactories(t.convertFloatingConstant, t.convertRoundingConstant); err != nil {
		return errors.Join(fmt.Errorf("floating-point converter constants: %w", err), converter.Close())
	}
	rw, err := rewriter.NewFPA2BVRewriter(state.manager, converter)
	if err != nil {
		if rw != nil {
			err = errors.Join(err, rw.Close())
		}
		return errors.Join(fmt.Errorf("floating-point rewriter construction: %w", err), converter.Close())
	}
	t.rewriter = rw
	return nil
}

func (t *SMTFloatingPointTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTFloatingPointTheory) wrap(expression ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(234, "m_fpa_util.is_float(e) || m_fpa_util.is_rm(e)", floating || rounding); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(235, "!m_fpa_util.is_bvwrap(e)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return !own || op != ast.FPAOpBVWrap, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if !floating && !rounding {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if op, own, operationErr := t.operation(expression); operationErr != nil {
		return ast.ASTHandle{}, operationErr
	} else if own && op == ast.FPAOpFP {
		arguments := make([]ast.ASTHandle, 3)
		for i := range arguments {
			arguments[i], err = state.manager.GetArg(expression, uint32(i))
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return state.manager.MkBitVectorConcat(arguments)
	}
	if !rounding {
		if err = fpaSourceAssertBool(252, "m_converter.is_float(es)", floating); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpBVWrap, nil, []ast.ASTHandle{expression})
}

func (t *SMTFloatingPointTheory) unwrapFloating(expression, sort ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(267, "!m_fpa_util.is_fp(e)", func() (bool, error) {
		op, own, e := t.operation(expression)
		return !own || op != ast.FPAOpFP, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	width, widthErr := state.manager.BitVectorSize(expression)
	if err = fpaSourceAssertBool(268, "m_bv_util.is_bv(e)", widthErr == nil); err != nil {
		return ast.ASTHandle{}, err
	}
	if widthErr != nil {
		return ast.ASTHandle{}, widthErr
	}
	ebits, sbits, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(269, "m_fpa_util.is_float(s) || m_fpa_util.is_rm(s)", floating || rounding); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(284, "m_fpa_util.is_float(s)", floating); err != nil {
		return ast.ASTHandle{}, err
	}
	if !floating {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if err = fpaSourceAssertBool(286, "bv_sz == m_fpa_util.get_ebits(s) + sbits", width == ebits+sbits); err != nil {
		return ast.ASTHandle{}, err
	}
	if width != ebits+sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sign, err := state.manager.MkBitVectorExtract(width-1, width-1, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := state.manager.MkBitVectorExtract(width-2, sbits-1, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := state.manager.MkBitVectorExtract(sbits-2, 0, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
}

func (t *SMTFloatingPointTheory) convertFloatingConstant(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(46, "f->get_family_id() == null_family_id", func() (bool, error) {
		family, e := state.manager.GetFamilyID(declaration)
		return family == ast.NullFamilyID, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(47, "f->get_arity() == 0", func() (bool, error) {
		arity, e := state.manager.GetArity(declaration)
		return arity == 0, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	original, err := state.manager.MkApp(declaration, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	packed, err := t.wrap(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := state.manager.GetSort(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil || !floating {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width, err := state.manager.BitVectorSize(packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssertBool(58, "bv_sz == m_th.m_fpa_util.get_ebits(s) + sbits", width == ebits+sbits); err != nil {
		return ast.ASTHandle{}, err
	}
	if width != ebits+sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sign, err := state.manager.MkBitVectorExtract(width-1, width-1, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := state.manager.MkBitVectorExtract(width-2, sbits-1, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := state.manager.MkBitVectorExtract(sbits-2, 0, packed)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpFP, nil, []ast.ASTHandle{sign, exponent, significand})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(62, "m_th.m_fpa_util.is_float(result)", func() (bool, error) {
		resultSort, e := state.manager.GetSort(result)
		if e != nil {
			return false, e
		}
		_, _, matched, e := state.manager.FloatingPointSortShape(resultSort)
		return matched, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

func (t *SMTFloatingPointTheory) convertRoundingConstant(declaration ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(70, "f->get_family_id() == null_family_id", func() (bool, error) {
		family, e := state.manager.GetFamilyID(declaration)
		return family == ast.NullFamilyID, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(71, "f->get_arity() == 0", func() (bool, error) {
		arity, e := state.manager.GetArity(declaration)
		return arity == 0, e
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = fpaSourceAssert(77, "is_rm(f->get_range())", func() (bool, error) {
		rangeSort, e := state.manager.GetRange(declaration)
		if e != nil {
			return false, e
		}
		return state.manager.IsRoundingModeSort(rangeSort)
	}); err != nil {
		return ast.ASTHandle{}, err
	}
	original, err := state.manager.MkApp(declaration, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	packed, err := t.wrap(original)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpBV2RM, nil, []ast.ASTHandle{packed})
}

func (t *SMTFloatingPointTheory) convert(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if err := t.traceFPAExpression("t_fpa", "convert", 354, "converting ", expression); err != nil {
		return ast.ASTHandle{}, err
	}
	if value, found := t.converted[expression]; found {
		if err := t.traceFPAConversion(358, "cached:", expression, value); err != nil {
			return ast.ASTHandle{}, err
		}
		return value, nil
	}
	state, err := t.State()
	if err != nil || t.rewriter == nil {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	inputSort, err := state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	booleanSort, err := state.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if inputSort == booleanSort {
		if err = t.traceFPAConvertAtom(expression); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// fpa2bv already represents this constructor by its three BV components.
	// Keeping it unchanged is also the fixed point that prevents translated
	// FP equalities from recursively generating fresh translated equalities.
	if op, own, operationErr := t.operation(expression); operationErr != nil {
		return ast.ASTHandle{}, operationErr
	} else if own && op == ast.FPAOpFP {
		if err := t.traceFPAConversion(372, "converted; caching:", expression, expression); err != nil {
			return ast.ASTHandle{}, err
		}
		key, keyErr := state.manager.NewRef(expression)
		if keyErr != nil {
			return ast.ASTHandle{}, keyErr
		}
		value, valueErr := state.manager.NewRef(expression)
		if valueErr != nil {
			_ = state.manager.DelRef(&key)
			return ast.ASTHandle{}, valueErr
		}
		t.converted[expression] = expression
		t.conversion = append(t.conversion, smtFPAConversion{key: key, value: value})
		return expression, nil
	}
	output, err := state.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.rewriter.Rewrite(expression, output); err != nil {
		_ = state.manager.DelRef(&output)
		return ast.ASTHandle{}, err
	}
	value, err := state.manager.RefGet(output)
	if err != nil {
		_ = state.manager.DelRef(&output)
		return ast.ASTHandle{}, err
	}
	if inputSort == booleanSort {
		if err = fpaSourceAssert(302, "is_app(res)", func() (bool, error) {
			kind, e := state.manager.GetKind(value)
			return kind == ast.ASTApp, e
		}); err == nil {
			err = fpaSourceAssert(303, "m.is_bool(res)", func() (bool, error) {
				resultSort, e := state.manager.GetSort(value)
				return resultSort == booleanSort, e
			})
		}
	} else {
		_, _, floating, shapeErr := state.manager.FloatingPointSortShape(inputSort)
		if shapeErr != nil {
			err = shapeErr
		}
		rounding := false
		if err == nil {
			rounding, err = state.manager.IsRoundingModeSort(inputSort)
		}
		if err == nil && (floating || rounding) {
			if err = t.traceFPAConvertTerm(expression, value); err != nil {
				_ = state.manager.DelRef(&output)
				return ast.ASTHandle{}, err
			}
			err = fpaSourceAssertBool(308, "m_fpa_util.is_rm(e) || m_fpa_util.is_float(e)", true)
			if err == nil {
				resultOp, resultOwn, operationErr := t.operation(value)
				if operationErr != nil {
					err = operationErr
				} else if rounding {
					err = fpaSourceAssertBool(320, "m_fpa_util.is_bv2rm(e_conv)", resultOwn && resultOp == ast.FPAOpBV2RM)
				} else {
					err = fpaSourceAssertBool(326, "m_fpa_util.is_fp(e_conv)", resultOwn && resultOp == ast.FPAOpFP)
				}
			}
		} else if err == nil {
			err = fpaSourceAssert(341, "to_app(e)->get_family_id() == get_family_id()", func() (bool, error) {
				family, e := state.manager.GetFamilyID(expression)
				return family == t.FamilyID(), e
			})
		}
	}
	if err != nil {
		_ = state.manager.DelRef(&output)
		return ast.ASTHandle{}, err
	}
	if err = t.traceFPAConversion(372, "converted; caching:", expression, value); err != nil {
		_ = state.manager.DelRef(&output)
		return ast.ASTHandle{}, err
	}
	key, err := state.manager.NewRef(expression)
	if err != nil {
		_ = state.manager.DelRef(&output)
		return ast.ASTHandle{}, err
	}
	t.converted[expression] = value
	t.conversion = append(t.conversion, smtFPAConversion{key: key, value: output})
	return value, nil
}

func (t *SMTFloatingPointTheory) assertConstraint(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil || state.manager.IsTrue(expression) {
		return err
	}
	if err = t.traceFPAExpression("t_fpa_detail", "assert_cnstr", 410, "asserting ", expression); err != nil {
		return err
	}
	if err = state.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
}

func (t *SMTFloatingPointTheory) flushSideConstraints() (result error) {
	if t.converter == nil {
		return ast.ErrASTHandle
	}
	assertions, err := t.converter.FlushExtraAssertions()
	if err != nil {
		return err
	}
	state, stateErr := t.State()
	if stateErr != nil {
		result = stateErr
	} else {
		condition, conditionErr := state.manager.MkAndArgs(assertions)
		var simplified ast.ASTRef
		if conditionErr == nil {
			simplified, conditionErr = state.manager.NewRef(ast.ASTHandle{})
		}
		if conditionErr == nil {
			conditionErr = state.theoryRewriter.Rewrite(condition, simplified)
		}
		if conditionErr == nil {
			condition, conditionErr = state.manager.RefGet(simplified)
		}
		if conditionErr == nil {
			conditionErr = t.traceFPASideCondition(condition)
		}
		if simplified != (ast.ASTRef{}) {
			conditionErr = errors.Join(conditionErr, state.manager.DelRef(&simplified))
		}
		if conditionErr != nil {
			result = conditionErr
		}
		for _, assertion := range assertions {
			if result == nil {
				result = t.assertConstraint(assertion)
			}
		}
	}
	if state != nil {
		for _, assertion := range assertions {
			result = errors.Join(result, state.manager.DecRef(assertion))
		}
	}
	return result
}

func (t *SMTFloatingPointTheory) bridgeAtom(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	converted, err := t.convert(expression)
	if err != nil {
		return err
	}
	if err = t.flushSideConstraints(); err != nil {
		return err
	}
	if err = state.Internalize(converted, true); err != nil {
		return err
	}
	definition, err := state.Literal(converted)
	if err != nil {
		return err
	}
	literal := NewSMTLiteral(variable, false)
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), definition}); err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal, definition.Negated()})
}

func (t *SMTFloatingPointTheory) bitVectorEquality(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	leftWidth, err := state.manager.BitVectorSize(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightWidth, err := state.manager.BitVectorSize(right)
	if err != nil || leftWidth != rightWidth || leftWidth == 0 {
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	bits := make([]ast.ASTHandle, leftWidth)
	for index := uint32(0); index < leftWidth; index++ {
		leftBit, bitErr := state.manager.MkBitVectorBit2Bool(index, left)
		if bitErr != nil {
			return ast.ASTHandle{}, bitErr
		}
		rightBit, bitErr := state.manager.MkBitVectorBit2Bool(index, right)
		if bitErr != nil {
			return ast.ASTHandle{}, bitErr
		}
		bits[index], bitErr = state.manager.MkIff(leftBit, rightBit)
		if bitErr != nil {
			return ast.ASTHandle{}, bitErr
		}
	}
	return state.manager.MkAndArgs(bits)
}

func (t *SMTFloatingPointTheory) encoded(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil || (!floating && !rounding) {
		return ast.ASTHandle{}, false, err
	}
	converted, err := t.convert(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if floating {
		value, wrapErr := t.wrap(converted)
		return value, true, wrapErr
	}
	op, own, err := t.operation(converted)
	if err != nil || !own || op != ast.FPAOpBV2RM {
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		return ast.ASTHandle{}, false, ast.ErrASTDomain
	}
	value, err := state.manager.GetArg(converted, 0)
	return value, true, err
}

func (t *SMTFloatingPointTheory) InternalizeAtom(expression ast.ASTHandle, _ bool) (bool, error) {
	if err := t.traceFPAExpression("t_fpa_internalize", "internalize_atom", 426, "internalizing atom: ", expression); err != nil {
		return false, err
	}
	_, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = fpaSourceAssert(427, "atom->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), e
	}); err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := state.manager.GetArg(expression, i)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
	}
	variable, found := state.BoolVar(expression)
	if !found {
		variable, err = state.MkBoolVar(expression)
		if err != nil {
			return false, err
		}
	}
	data, err := state.BooleanData(variable)
	if err != nil {
		return false, err
	}
	if !data.IsTheoryAtom() {
		if err = state.SetVariableTheory(variable, t.FamilyID()); err != nil {
			return false, err
		}
	}
	if found {
		return true, nil
	}
	return true, t.bridgeAtom(expression, variable)
}

func (t *SMTFloatingPointTheory) ensureVariable(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < count; i++ {
		argument, argumentErr := state.manager.GetArg(expression, i)
		if argumentErr != nil {
			return false, argumentErr
		}
		if argumentErr = state.Internalize(argument, false); argumentErr != nil {
			return false, argumentErr
		}
	}
	node := state.context.enodes[expression]
	if node == nil {
		node, err = state.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	if t.IsAttachedToVariable(node) {
		return false, nil
	}
	_, err = t.createFPAVariable(node)
	return err == nil, err
}

func (t *SMTFloatingPointTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	if err := t.traceFPAExpression("t_fpa_internalize", "internalize_term", 452, "internalizing term: ", expression); err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	state, err := t.State()
	if err != nil {
		return false, err
	}
	if err = fpaSourceAssert(453, "term->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), e
	}); err != nil {
		return false, err
	}
	if err = fpaSourceAssertBool(454, "!get_context().e_internalized(term)", state.context.enodes[expression] == nil); err != nil {
		return false, err
	}
	created, err := t.ensureVariable(expression)
	if err != nil || !created {
		return true, err
	}
	switch op {
	case ast.FPAOpToFP, ast.FPAOpToUBV, ast.FPAOpToSBV, ast.FPAOpToReal, ast.FPAOpToIEEEBV:
		converted, conversionErr := t.convert(expression)
		if conversionErr != nil {
			return false, conversionErr
		}
		equality, conversionErr := state.manager.MkEq(expression, converted)
		if conversionErr == nil {
			conversionErr = t.assertConstraint(equality)
		}
		if conversionErr == nil {
			conversionErr = t.flushSideConstraints()
		}
		return conversionErr == nil, conversionErr
	default:
		return true, nil
	}
}

func (t *SMTFloatingPointTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(left)
	if err != nil {
		return err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil || (!floating && !rounding) {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	converted := make([]ast.ASTHandle, 2)
	for index, original := range []ast.ASTHandle{left, right} {
		var conversionErr error
		converted[index], conversionErr = t.convert(original)
		if conversionErr != nil {
			return conversionErr
		}
	}
	if err = t.traceFPAConvertedEquality(converted[0], converted[1]); err != nil {
		return err
	}
	var defined ast.ASTHandle
	if floating {
		defined, err = t.converter.MkEq(converted[0], converted[1])
		if err != nil {
			return err
		}
	} else {
		encoded := make([]ast.ASTHandle, 2)
		for index := range converted {
			op, own, operationErr := t.operation(converted[index])
			if operationErr != nil || !own || op != ast.FPAOpBV2RM {
				if operationErr != nil {
					return operationErr
				}
				return ast.ErrASTDomain
			}
			encoded[index], err = state.manager.GetArg(converted[index], 0)
			if err != nil {
				return err
			}
		}
		defined, err = t.bitVectorEquality(encoded[0], encoded[1])
		if err != nil {
			return err
		}
	}
	if err = t.flushSideConstraints(); err != nil {
		return err
	}
	if err = state.Internalize(defined, true); err != nil {
		return err
	}
	definition, err := state.Literal(defined)
	if err != nil {
		return err
	}
	literal := NewSMTLiteral(variable, false)
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), definition}); err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal, definition.Negated()})
}

func (t *SMTFloatingPointTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	if node == nil {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceFPAExpression("t_fpa", "apply_sort_cnstr", 494, "apply sort cnstr for: ", node.Owner()); err != nil {
		return err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return err
	}
	ownerSort, err := state.manager.GetSort(node.Owner())
	if err != nil {
		return err
	}
	_, _, ownerFloating, err := state.manager.FloatingPointSortShape(ownerSort)
	if err != nil {
		return err
	}
	ownerRounding, err := state.manager.IsRoundingModeSort(ownerSort)
	if err != nil {
		return err
	}
	if err = fpaSourceAssert(495, "s->get_family_id() == get_family_id()", func() (bool, error) {
		family, e := state.manager.GetFamilyID(sort)
		return family == t.FamilyID(), e
	}); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(496, "m_fpa_util.is_float(s) || m_fpa_util.is_rm(s)", floating || rounding); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(497, "m_fpa_util.is_float(n->get_owner()) || m_fpa_util.is_rm(n->get_owner())", ownerFloating || ownerRounding); err != nil {
		return err
	}
	if err = fpaSourceAssertBool(498, "n->get_owner()->get_decl()->get_range() == s", ownerSort == sort); err != nil {
		return err
	}
	if !floating && !rounding || !ownerFloating && !ownerRounding || ownerSort != sort {
		return ast.ErrASTDomain
	}
	if t.IsAttachedToVariable(node) {
		return nil
	}
	if _, err = t.createFPAVariable(node); err != nil {
		return err
	}
	if rounding {
		owner := node.Owner()
		if op, own, operationErr := t.operation(owner); operationErr != nil {
			return operationErr
		} else if !own || op != ast.FPAOpBV2RM {
			packed, wrapErr := t.wrap(owner)
			if wrapErr != nil {
				return wrapErr
			}
			limit, limitErr := state.manager.BitVectorNumeralUint64(4, 3)
			if limitErr != nil {
				return limitErr
			}
			valid, validErr := state.manager.MkBitVectorULE(packed, limit)
			if validErr != nil {
				return validErr
			}
			return t.assertConstraint(valid)
		}
	}
	return nil
}

// Syntactic FP/RM equalities are bridged in InternalizeEquality. The retained
// e-graph currently forbids creating a new Boolean circuit from inside a merge
// callback because doing so invalidates its parent rollback ordering.
func (t *SMTFloatingPointTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.traceFPAEqualityCallback("new_eq_eh", 528, "=", left, right)
}
func (t *SMTFloatingPointTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.traceFPAEqualityCallback("new_diseq_eh", 572, "!=", left, right)
}
func (t *SMTFloatingPointTheory) Assign(variable int32, isTrue bool) error {
	return t.traceFPAAssignment(variable, isTrue)
}

func (t *SMTFloatingPointTheory) Relevant(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceFPAExpression("t_fpa", "relevant_eh", 647, "relevant_eh for: ", expression); err != nil {
		return err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	_, _, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil {
		return err
	}
	if !floating && !rounding {
		family, familyErr := state.manager.GetFamilyID(expression)
		if familyErr != nil {
			return familyErr
		}
		if family == t.FamilyID() {
			return fpaSourceAssertBool(686, "!m_fpa_util.is_float(n) && !m_fpa_util.is_rm(n)", true)
		}
		return fpaSourceAssert(695, "m_bv_util.is_bv(n)", func() (bool, error) {
			_, e := state.manager.BitVectorSize(expression)
			return e == nil, nil
		})
	}
	if floating {
		if op, own, operationErr := t.operation(expression); operationErr != nil {
			return operationErr
		} else if own && op == ast.FPAOpFP {
			return nil
		}
		wrapper, wrapErr := t.wrap(expression)
		if wrapErr != nil {
			return wrapErr
		}
		var linked ast.ASTHandle
		if value, valueErr := state.manager.IsValue(expression); valueErr != nil {
			return valueErr
		} else if value {
			converted, conversionErr := t.convert(expression)
			if conversionErr != nil {
				return conversionErr
			}
			if conversionErr = fpaSourceAssert(666, "is_app(bv_val_e)", func() (bool, error) {
				kind, e := state.manager.GetKind(converted)
				return kind == ast.ASTApp, e
			}); conversionErr != nil {
				return conversionErr
			}
			if conversionErr = fpaSourceAssert(667, "to_app(bv_val_e)->get_num_args() == 3", func() (bool, error) {
				count, e := state.manager.GetNumArgs(converted)
				return count == 3, e
			}); conversionErr != nil {
				return conversionErr
			}
			linked, conversionErr = t.wrap(converted)
			if conversionErr != nil {
				return conversionErr
			}
		} else {
			linked, err = t.unwrapFloating(wrapper, sort)
			if err != nil {
				return err
			}
			linked, err = state.manager.MkEq(linked, expression)
			if err != nil {
				return err
			}
			if err = t.traceFPAWrapUnwrap(linked); err != nil {
				return err
			}
			if err = t.assertConstraint(linked); err != nil {
				return err
			}
			return t.flushSideConstraints()
		}
		constraint, constraintErr := t.bitVectorEquality(wrapper, linked)
		if constraintErr != nil {
			return constraintErr
		}
		if constraintErr = t.assertConstraint(constraint); constraintErr != nil {
			return constraintErr
		}
		return t.flushSideConstraints()
	}
	if op, own, operationErr := t.operation(expression); operationErr != nil {
		return operationErr
	} else if own && (op == ast.FPAOpFP || op == ast.FPAOpBV2RM) {
		return nil
	}
	packed, matched, err := t.encoded(expression)
	if err != nil || !matched {
		return err
	}
	wrapper, err := t.wrap(expression)
	if err != nil {
		return err
	}
	wrapperWidth, err := state.manager.BitVectorSize(wrapper)
	if err != nil {
		return err
	}
	if err = fpaSourceAssertBool(276, "bv_sz == 3", wrapperWidth == 3); err != nil {
		return err
	}
	if wrapperWidth != 3 {
		return ast.ErrASTDomain
	}
	constraint, err := t.bitVectorEquality(wrapper, packed)
	if err != nil {
		return err
	}
	if err = t.assertConstraint(constraint); err != nil {
		return err
	}
	return t.flushSideConstraints()
}

func (t *SMTFloatingPointTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, len(t.conversion))
	return nil
}

func (t *SMTFloatingPointTheory) PopScopes(count uint32) (result error) {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	level := len(t.scopes) - int(count)
	limit := t.scopes[level]
	for i := len(t.conversion) - 1; i >= limit; i-- {
		entry := &t.conversion[i]
		key, keyErr := state.manager.RefGet(entry.key)
		if keyErr == nil {
			delete(t.converted, key)
		}
		if keyErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion key: %w", keyErr))
		}
		if releaseErr := state.manager.DelRef(&entry.key); releaseErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion key release: %w", releaseErr))
		}
		if releaseErr := state.manager.DelRef(&entry.value); releaseErr != nil {
			result = errors.Join(result, fmt.Errorf("floating-point scoped conversion value release: %w", releaseErr))
		}
	}
	t.conversion = t.conversion[:limit]
	t.scopes = t.scopes[:level]
	if traceErr := t.traceFPAPop(count, len(t.scopes)); traceErr != nil {
		result = errors.Join(result, traceErr)
	}
	if baseErr := t.SMTTheoryBase.PopScopes(count); baseErr != nil {
		result = errors.Join(result, fmt.Errorf("floating-point base scope pop: %w", baseErr))
	}
	return result
}

func (t *SMTFloatingPointTheory) Close() (result error) {
	if t == nil || t.SMTTheoryBase == nil {
		return ast.ErrASTHandle
	}
	state, stateErr := t.State()
	if stateErr == nil {
		for i := range t.conversion {
			if err := state.manager.DelRef(&t.conversion[i].key); err != nil {
				result = errors.Join(result, fmt.Errorf("floating-point conversion key release: %w", err))
			}
			if err := state.manager.DelRef(&t.conversion[i].value); err != nil {
				result = errors.Join(result, fmt.Errorf("floating-point conversion value release: %w", err))
			}
		}
	} else {
		result = fmt.Errorf("floating-point state close: %w", stateErr)
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("floating-point rewriter close: %w", err))
		}
	}
	if t.converter != nil {
		if err := t.converter.Close(); err != nil {
			result = errors.Join(result, fmt.Errorf("floating-point converter close: %w", err))
		}
	}
	t.converted = nil
	t.conversion = nil
	t.scopes = nil
	t.rewriter = nil
	t.converter = nil
	if err := fpaSourceAssertBool(118, "m_trail_stack.get_num_scopes() == 0", len(t.scopes) == 0); err != nil {
		result = errors.Join(result, err)
	}
	if err := fpaSourceAssertBool(119, "m_conversions.empty()", len(t.conversion) == 0); err != nil {
		result = errors.Join(result, err)
	}
	if err := fpaSourceAssertBool(120, "m_is_added_to_model.empty()", true); err != nil {
		result = errors.Join(result, err)
	}
	return result
}

func (t *SMTFloatingPointTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	if err = t.traceFPAInitializeModel(generator); err != nil {
		return err
	}
	factory, err := protomodel.NewFloatingPointFactory(generator.manager, t.FamilyID())
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

func smtFPAEnodeDependency(node *SMTEnode) ([]SMTModelValueDependency, error) {
	dependency, err := NewSMTEnodeValueDependency(node)
	if err != nil {
		return nil, err
	}
	return []SMTModelValueDependency{dependency}, nil
}

func (t *SMTFloatingPointTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.model == nil || !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil || generator.manager != state.manager {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	owner := node.Owner()
	if err = fpaSourceAssertBool(739, "ctx.e_internalized(owner)", state.context.enodes[owner] != nil); err != nil {
		return nil, err
	}
	sort, err := state.manager.GetSort(owner)
	if err != nil {
		return nil, err
	}
	if err = t.traceFPAModelValue(generator, owner, sort); err != nil {
		return nil, err
	}
	if value, valueErr := state.manager.IsValue(owner); valueErr != nil {
		return nil, valueErr
	} else if value {
		return NewSMTExpressionValueProcedure(owner)
	}
	ebits, sbits, floating, err := state.manager.FloatingPointSortShape(sort)
	if err != nil {
		return nil, err
	}
	rounding, err := state.manager.IsRoundingModeSort(sort)
	if err != nil || (!floating && !rounding) {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	op, own, err := t.operation(owner)
	if err != nil {
		return nil, err
	}
	procedure := &smtFPAValueProcedure{manager: state.manager, ebits: ebits, sbits: sbits, rounding: rounding}
	if own && op == ast.FPAOpFP {
		if err = fpaSourceAssert(759, "to_app(owner)->get_num_args() == 3", func() (bool, error) {
			count, e := state.manager.GetNumArgs(owner)
			return count == 3, e
		}); err != nil {
			return nil, err
		}
		procedure.dependencies = make([]SMTModelValueDependency, 3)
		nodes := make([]*SMTEnode, 3)
		for i := range procedure.dependencies {
			argument, argumentErr := state.manager.GetArg(owner, uint32(i))
			if argumentErr != nil {
				return nil, argumentErr
			}
			argumentNode := state.context.enodes[argument]
			if argumentNode == nil {
				return nil, ast.ErrASTDomain
			}
			nodes[i] = argumentNode
			procedure.dependencies[i], argumentErr = NewSMTEnodeValueDependency(argumentNode)
			if argumentErr != nil {
				return nil, argumentErr
			}
		}
		if err = t.traceFPADependencies(generator, 770, nodes...); err != nil {
			return nil, err
		}
		if err = fpaSourceAssertBool(808, "res != 0", procedure != nil); err != nil {
			return nil, err
		}
		return procedure, nil
	}
	if own && op == ast.FPAOpBV2RM {
		if err = fpaSourceAssert(777, "to_app(owner)->get_num_args() == 1", func() (bool, error) {
			count, e := state.manager.GetNumArgs(owner)
			return count == 1, e
		}); err != nil {
			return nil, err
		}
		argument, argumentErr := state.manager.GetArg(owner, 0)
		if argumentErr != nil {
			return nil, argumentErr
		}
		argumentNode := state.context.enodes[argument]
		if argumentNode == nil {
			return nil, ast.ErrASTDomain
		}
		procedure.dependencies, err = smtFPAEnodeDependency(argumentNode)
		if err == nil {
			err = t.traceFPADependencies(generator, 782, argumentNode)
		}
		if err == nil {
			err = fpaSourceAssertBool(808, "res != 0", procedure != nil)
		}
		return procedure, err
	}
	wrapped, err := t.wrap(owner)
	if err != nil {
		return nil, err
	}
	if err = fpaSourceAssert(750, "m_bv_util.is_bv(wrapped)", func() (bool, error) {
		_, e := state.manager.BitVectorSize(wrapped)
		return e == nil, nil
	}); err != nil {
		return nil, err
	}
	if wrappedNode := state.context.enodes[wrapped]; wrappedNode != nil {
		procedure.dependencies, err = smtFPAEnodeDependency(wrappedNode)
		if err == nil {
			err = t.traceFPADependencies(generator, 798, wrappedNode)
		}
		if err == nil {
			err = fpaSourceAssertBool(808, "res != 0", procedure != nil)
		}
		return procedure, err
	}
	if err = t.traceFPAMissingDependency(generator, wrapped, state.context.enodes[owner] != nil); err != nil {
		return nil, err
	}
	if rounding {
		fallback, fallbackErr := state.manager.MkFamilyApp(t.FamilyID(), ast.FPAOpRMTowardZero, nil, nil)
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		return NewSMTExpressionValueProcedure(fallback)
	}
	fallback, err := state.manager.MkFPAZeroOfSort(sort, false)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(fallback)
}

func (t *SMTFloatingPointTheory) FinalizeModel(generator *SMTModelGenerator) (result error) {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil || t.converter == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	converter, err := modelfpa.NewConverter(state.manager, t.converter)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, converter.Close()) }()
	target, err := model.NewModelCore(state.manager)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, target.Close()) }()
	source := generator.model.Core()
	seen, err := converter.ConvertFunctions(source, target)
	if err != nil {
		return err
	}
	for declaration := range seen {
		if err = source.UnregisterDeclaration(declaration); err != nil {
			return err
		}
	}
	constantCount, err := target.NumConstants()
	if err != nil {
		return err
	}
	for index := uint32(0); index < constantCount; index++ {
		declaration, copyErr := target.Constant(index)
		if copyErr != nil {
			return copyErr
		}
		value, copyErr := target.ConstantInterpretation(declaration)
		if copyErr != nil {
			return copyErr
		}
		if copyErr = source.RegisterConstant(declaration, value); copyErr != nil {
			return copyErr
		}
	}
	functionCount := target.NumFunctionDeclarations()
	for index := uint32(0); index < functionCount; index++ {
		declaration, copyErr := target.Function(index)
		if copyErr != nil {
			return copyErr
		}
		interpretation, copyErr := target.FunctionInterpretation(declaration)
		if copyErr != nil || interpretation == nil {
			if copyErr != nil {
				return copyErr
			}
			return ast.ErrASTDomain
		}
		copy, copyErr := interpretation.Copy()
		if copyErr != nil {
			return copyErr
		}
		if copyErr = source.RegisterFunction(declaration, copy); copyErr != nil {
			_ = copy.Close()
			return copyErr
		}
	}
	return nil
}

var _ SMTTheoryInternalizer = (*SMTFloatingPointTheory)(nil)
var _ SMTModelValueTheory = (*SMTFloatingPointTheory)(nil)
var _ SMTModelFinalizer = (*SMTFloatingPointTheory)(nil)
