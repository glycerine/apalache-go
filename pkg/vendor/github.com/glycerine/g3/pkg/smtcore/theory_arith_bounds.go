package smtcore

// Source: src/smt/theory_arith.h:215-421 and
// src/smt/theory_arith_aux.h:330-420,732-800.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// smtArithmeticValue represents infinity + rational + epsilon in
// lexicographic order. Zero infinity covers mi_ext; zero epsilon covers i_ext.
type smtArithmeticValue struct {
	infinity, rational, infinitesimal big.Rat
}

func newSMTArithmeticValue(rational, infinitesimal *big.Rat) smtArithmeticValue {
	var result smtArithmeticValue
	if rational != nil {
		result.rational.Set(rational)
	}
	if infinitesimal != nil {
		result.infinitesimal.Set(infinitesimal)
	}
	return result
}
func (v *smtArithmeticValue) set(other *smtArithmeticValue) {
	v.infinity.Set(&other.infinity)
	v.rational.Set(&other.rational)
	v.infinitesimal.Set(&other.infinitesimal)
}
func (v *smtArithmeticValue) add(other *smtArithmeticValue) {
	v.infinity.Add(&v.infinity, &other.infinity)
	v.rational.Add(&v.rational, &other.rational)
	v.infinitesimal.Add(&v.infinitesimal, &other.infinitesimal)
}
func (v *smtArithmeticValue) sub(other *smtArithmeticValue) {
	v.infinity.Sub(&v.infinity, &other.infinity)
	v.rational.Sub(&v.rational, &other.rational)
	v.infinitesimal.Sub(&v.infinitesimal, &other.infinitesimal)
}
func (v *smtArithmeticValue) cmp(other *smtArithmeticValue) int {
	if c := v.infinity.Cmp(&other.infinity); c != 0 {
		return c
	}
	if c := v.rational.Cmp(&other.rational); c != 0 {
		return c
	}
	return v.infinitesimal.Cmp(&other.infinitesimal)
}

type smtArithmeticBoundKind uint8

const (
	smtArithmeticLowerBound smtArithmeticBoundKind = iota
	smtArithmeticUpperBound
)

type smtArithmeticAtomKind uint8

const (
	smtArithmeticLowerAtom smtArithmeticAtomKind = iota
	smtArithmeticUpperAtom
)

type smtArithmeticAntecedents struct {
	literals                                  []SMTLiteral
	equalities                                []SMTEnodePair
	literalCoefficients, equalityCoefficients []*big.Rat
}

func (a *smtArithmeticAntecedents) reset() {
	a.literals = a.literals[:0]
	a.equalities = a.equalities[:0]
	a.literalCoefficients = a.literalCoefficients[:0]
	a.equalityCoefficients = a.equalityCoefficients[:0]
}
func (a *smtArithmeticAntecedents) pushLiteral(literal SMTLiteral, coefficient *big.Rat, proofs bool) error {
	if coefficient == nil {
		return ast.ErrASTDomain
	}
	a.literals = append(a.literals, literal)
	if proofs {
		a.literalCoefficients = append(a.literalCoefficients, new(big.Rat).Set(coefficient))
	}
	return nil
}
func (a *smtArithmeticAntecedents) pushEquality(pair SMTEnodePair, coefficient *big.Rat, proofs bool) error {
	if coefficient == nil || pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	a.equalities = append(a.equalities, pair)
	if proofs {
		a.equalityCoefficients = append(a.equalityCoefficients, new(big.Rat).Set(coefficient))
	}
	return nil
}
func (a *smtArithmeticAntecedents) numParameters() int {
	if len(a.literalCoefficients) == 0 && len(a.equalityCoefficients) == 0 {
		return 0
	}
	return 1 + len(a.literalCoefficients) + len(a.equalityCoefficients)
}

type smtArithmeticBoundJustification interface {
	pushJustification(*smtArithmeticAntecedents, *big.Rat, bool) error
}

type smtArithmeticEqualityBound struct {
	smtArithmeticBound
	left, right *SMTEnode
}

func newSMTArithmeticEqualityBound(variable SMTTheoryVar, value *smtArithmeticValue, kind smtArithmeticBoundKind, left, right *SMTEnode) (*smtArithmeticEqualityBound, error) {
	if variable < 0 || value == nil || kind > smtArithmeticUpperBound || left == nil || right == nil || left.Root() != right.Root() {
		return nil, ast.ErrASTDomain
	}
	result := &smtArithmeticEqualityBound{left: left, right: right}
	result.variable, result.kind = variable, kind
	result.value.set(value)
	return result, nil
}
func (b *smtArithmeticEqualityBound) pushJustification(out *smtArithmeticAntecedents, coefficient *big.Rat, proofs bool) error {
	if b == nil || out == nil || b.left == nil || b.right == nil || b.left.Root() != b.right.Root() {
		return ast.ErrASTDomain
	}
	return out.pushEquality(SMTEnodePair{First: b.left, Second: b.right}, coefficient, proofs)
}

type smtArithmeticAtom struct {
	smtArithmeticBound
	boolean   SMTBoolVar
	threshold smtArithmeticValue
	atomKind  smtArithmeticAtomKind
	truth     bool
}

func newSMTArithmeticAtom(boolean SMTBoolVar, variable SMTTheoryVar, threshold *smtArithmeticValue, kind smtArithmeticAtomKind) (*smtArithmeticAtom, error) {
	if boolean < SMTFirstBoolVar || variable < 0 || threshold == nil || kind > smtArithmeticUpperAtom {
		return nil, ast.ErrASTDomain
	}
	result := &smtArithmeticAtom{boolean: boolean, atomKind: kind}
	result.variable, result.atom = variable, true
	result.threshold.set(threshold)
	return result, nil
}
func (a *smtArithmeticAtom) assign(truth bool, epsilon *smtArithmeticValue) error {
	if a == nil || epsilon == nil {
		return ast.ErrASTDomain
	}
	a.truth = truth
	a.value.set(&a.threshold)
	if truth {
		a.kind = smtArithmeticBoundKind(a.atomKind)
		return nil
	}
	if a.atomKind == smtArithmeticLowerAtom {
		a.value.sub(epsilon)
		a.kind = smtArithmeticUpperBound
	} else {
		a.value.add(epsilon)
		a.kind = smtArithmeticLowerBound
	}
	return nil
}
func (a *smtArithmeticAtom) pushJustification(out *smtArithmeticAntecedents, coefficient *big.Rat, proofs bool) error {
	if a == nil || out == nil {
		return ast.ErrASTDomain
	}
	return out.pushLiteral(NewSMTLiteral(a.boolean, !a.truth), coefficient, proofs)
}

type smtArithmeticDerivedBound struct {
	smtArithmeticBound
	literals   []SMTLiteral
	equalities []SMTEnodePair
}

func (b *smtArithmeticDerivedBound) pushLiteral(literal SMTLiteral, _ *big.Rat) error {
	b.literals = append(b.literals, literal)
	return nil
}
func (b *smtArithmeticDerivedBound) pushEquality(pair SMTEnodePair, _ *big.Rat) error {
	if pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	b.equalities = append(b.equalities, pair)
	return nil
}

func (b *smtArithmeticDerivedBound) pushJustification(out *smtArithmeticAntecedents, coefficient *big.Rat, proofs bool) error {
	if b == nil || out == nil || coefficient == nil {
		return ast.ErrASTDomain
	}
	for _, literal := range b.literals {
		if err := out.pushLiteral(literal, coefficient, proofs); err != nil {
			return err
		}
	}
	for _, equality := range b.equalities {
		if err := out.pushEquality(equality, coefficient, proofs); err != nil {
			return err
		}
	}
	return nil
}

type smtArithmeticJustifiedDerivedBound struct {
	smtArithmeticDerivedBound
	literalCoefficients, equalityCoefficients []*big.Rat
}

func (b *smtArithmeticJustifiedDerivedBound) pushLiteral(literal SMTLiteral, coefficient *big.Rat) error {
	if coefficient == nil {
		return ast.ErrASTDomain
	}
	for index, old := range b.literals {
		if old == literal {
			b.literalCoefficients[index].Add(b.literalCoefficients[index], coefficient)
			return nil
		}
	}
	b.literals = append(b.literals, literal)
	b.literalCoefficients = append(b.literalCoefficients, new(big.Rat).Set(coefficient))
	return nil
}
func sameSMTEnodePair(left, right SMTEnodePair) bool {
	return left.First == right.First && left.Second == right.Second
}
func (b *smtArithmeticJustifiedDerivedBound) pushEquality(pair SMTEnodePair, coefficient *big.Rat) error {
	if coefficient == nil || pair.First == nil || pair.Second == nil {
		return ast.ErrASTDomain
	}
	for index, old := range b.equalities {
		if sameSMTEnodePair(old, pair) {
			b.equalityCoefficients[index].Add(b.equalityCoefficients[index], coefficient)
			return nil
		}
	}
	b.equalities = append(b.equalities, pair)
	b.equalityCoefficients = append(b.equalityCoefficients, new(big.Rat).Set(coefficient))
	return nil
}
func (b *smtArithmeticJustifiedDerivedBound) pushJustification(out *smtArithmeticAntecedents, coefficient *big.Rat, proofs bool) error {
	if b == nil || out == nil || coefficient == nil {
		return ast.ErrASTDomain
	}
	for index, literal := range b.literals {
		scaled := new(big.Rat).Mul(coefficient, b.literalCoefficients[index])
		if err := out.pushLiteral(literal, scaled, proofs); err != nil {
			return err
		}
	}
	for index, equality := range b.equalities {
		scaled := new(big.Rat).Mul(coefficient, b.equalityCoefficients[index])
		if err := out.pushEquality(equality, scaled, proofs); err != nil {
			return err
		}
	}
	return nil
}

type smtArithmeticDerivedTarget interface {
	pushLiteral(SMTLiteral, *big.Rat) error
	pushEquality(SMTEnodePair, *big.Rat) error
}

func smtAccumulateArithmeticJustification(source smtArithmeticBoundJustification, target smtArithmeticDerivedTarget, coefficient *big.Rat, proofs bool, literals map[int32]bool, equalities map[SMTEnodePair]bool) error {
	if source == nil || target == nil || coefficient == nil || literals == nil || equalities == nil {
		return ast.ErrASTDomain
	}
	var antecedents smtArithmeticAntecedents
	if err := source.pushJustification(&antecedents, coefficient, proofs); err != nil {
		return err
	}
	for index, literal := range antecedents.literals {
		if literals[literal.Index()] {
			continue
		}
		value := new(big.Rat)
		if proofs {
			value.Set(antecedents.literalCoefficients[index])
		} else {
			literals[literal.Index()] = true
		}
		if err := target.pushLiteral(literal, value); err != nil {
			return err
		}
	}
	for index, equality := range antecedents.equalities {
		if equalities[equality] {
			continue
		}
		value := new(big.Rat)
		if proofs {
			value.Set(antecedents.equalityCoefficients[index])
		} else {
			equalities[equality] = true
		}
		if err := target.pushEquality(equality, value); err != nil {
			return err
		}
	}
	return nil
}

func smtRatFloor(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	if value.Sign() < 0 && remainder.Sign() != 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	return quotient
}
func smtNormalizeArithmeticIntegerBound(value *smtArithmeticValue, kind smtArithmeticBoundKind) (smtArithmeticValue, error) {
	if value == nil || value.infinity.Sign() != 0 || kind > smtArithmeticUpperBound {
		return smtArithmeticValue{}, ast.ErrASTDomain
	}
	floor := smtRatFloor(&value.rational)
	integral := value.rational.IsInt()
	result := newSMTArithmeticValue(nil, nil)
	if kind == smtArithmeticLowerBound {
		result.rational.SetInt(floor)
		if !integral || value.infinitesimal.Sign() > 0 {
			result.rational.Add(&result.rational, big.NewRat(1, 1))
		}
	} else {
		result.rational.SetInt(floor)
		if integral && value.infinitesimal.Sign() < 0 {
			result.rational.Sub(&result.rational, big.NewRat(1, 1))
		}
	}
	return result, nil
}

type smtArithmeticVariableKind uint8

const (
	smtArithmeticNonBase smtArithmeticVariableKind = iota
	smtArithmeticBase
	smtArithmeticQuasiBase
)

type smtArithmeticVariableData struct {
	rowID                        int
	kind                         smtArithmeticVariableKind
	integer, nonlinearPropagated bool
}
type smtArithmeticBoundTrail struct {
	encoded SMTTheoryVar
	old     *smtArithmeticBound
}

func newSMTArithmeticBoundTrail(variable SMTTheoryVar, old *smtArithmeticBound, upper bool) (smtArithmeticBoundTrail, error) {
	if variable < 0 {
		return smtArithmeticBoundTrail{}, ast.ErrASTDomain
	}
	encoded := variable << 1
	if upper {
		encoded++
	}
	return smtArithmeticBoundTrail{encoded: encoded, old: old}, nil
}
func (t smtArithmeticBoundTrail) upper() bool            { return t.encoded&1 != 0 }
func (t smtArithmeticBoundTrail) variable() SMTTheoryVar { return t.encoded >> 1 }
