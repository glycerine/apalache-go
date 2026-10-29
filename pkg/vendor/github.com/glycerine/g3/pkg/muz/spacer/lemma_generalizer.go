package spacer

// Source: src/muz/spacer/spacer_generalizers.cpp
// lemma_bool_inductive_generalizer and src/muz/spacer/spacer_context.cpp
// pred_transformer::check_inductive.

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrLemmaSanity = errors.New("muz/spacer: generalized lemma is not inductive")

func (p *PredicateTransformer) checkInductiveCube(cube []ast.ASTHandle, level uint32) (_ []ast.ASTHandle, usesLevel uint32, inductive bool, resultErr error) {
	conjunction, err := p.manager.MkAndArray(cube)
	if err != nil {
		return nil, 0, false, err
	}
	states, err := p.manager.PushNot(conjunction)
	if err != nil {
		return nil, 0, false, err
	}
	head, err := p.Head()
	if err != nil {
		return nil, 0, false, err
	}
	assumptionRefs, err := p.makeAssumptions(head, states)
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, assumptionRefs)) }()
	background := make([]ast.ASTHandle, 0, len(assumptionRefs)+1)
	for i := range assumptionRefs {
		handle, getErr := p.manager.RefGet(assumptionRefs[i])
		if getErr != nil {
			return nil, 0, false, getErr
		}
		background = append(background, handle)
	}
	extension, err := p.manager.RefGet(p.extendLiteral)
	if err != nil {
		return nil, 0, false, err
	}
	background = append(background, extension)
	check, err := p.solver.CheckAssumptions(cube, nil, PropCheckOptions{
		Background: background, NeedCore: true, SubsetBasedCore: true, Level: &level,
	})
	if err != nil {
		return nil, 0, false, err
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close(p.manager)) }()
	if check.Status != lbool.False {
		return nil, check.UsesLevel, false, nil
	}
	core := make([]ast.ASTHandle, len(check.Core))
	for i := range check.Core {
		core[i], err = p.manager.RefGet(check.Core[i])
		if err != nil {
			return nil, 0, false, err
		}
	}
	return core, check.UsesLevel, true, nil
}

func containsGeneralizedLiteral(values []ast.ASTHandle, value ast.ASTHandle) bool {
	for _, prior := range values {
		if prior == value {
			return true
		}
	}
	return false
}

func (p *PredicateTransformer) expandInductiveLiteral(literal ast.ASTHandle) ([]ast.ASTHandle, error) {
	family, err := p.manager.GetFamilyID(literal)
	if err != nil || family != p.manager.BasicFamilyID() {
		return []ast.ASTHandle{literal}, err
	}
	kind, err := p.manager.GetDeclKind(literal)
	if err != nil || kind != ast.ASTOpEq {
		return []ast.ASTHandle{literal}, err
	}
	left, err := p.manager.GetArg(literal, 0)
	if err != nil {
		return nil, err
	}
	right, err := p.manager.GetArg(literal, 1)
	if err != nil {
		return nil, err
	}
	sort, err := p.manager.GetSort(left)
	if err != nil {
		return nil, err
	}
	sortFamily, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return nil, err
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	if sortFamily == arithmetic {
		le, err := p.manager.MkArithmeticLE(left, right)
		if err != nil {
			return nil, err
		}
		ge, err := p.manager.MkArithmeticGE(left, right)
		if err != nil {
			return nil, err
		}
		return []ast.ASTHandle{le, ge}, nil
	}
	if expanded, matched, err := p.expandInductiveDatatypeEquality(left, right); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveDatatypeEquality(right, left); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveBitVectorEquality(left, right); err != nil || matched {
		return expanded, err
	}
	if expanded, matched, err := p.expandInductiveBitVectorEquality(right, left); err != nil || matched {
		return expanded, err
	}
	return []ast.ASTHandle{literal}, nil
}

// expandInductiveDatatypeEquality treats value as the constructor side and
// term as the side replaced by a recognizer and accessor equalities.
func (p *PredicateTransformer) expandInductiveDatatypeEquality(term, value ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	kind, err := p.manager.GetKind(value)
	if err != nil || kind != ast.ASTApp {
		return nil, false, err
	}
	constructor, err := p.manager.GetDecl(value)
	if err != nil {
		return nil, false, err
	}
	family, err := p.manager.GetFamilyID(constructor)
	if err != nil {
		return nil, false, err
	}
	datatype, err := p.manager.RegisterDatatypePlugin()
	if err != nil || family != datatype {
		return nil, false, err
	}
	declKind, err := p.manager.GetDeclKind(constructor)
	if err != nil || declKind != ast.ASTDatatypeConstructor {
		return nil, false, err
	}
	recognizer, err := p.manager.DatatypeConstructorRecognizer(constructor)
	if err != nil {
		return nil, false, err
	}
	isConstructor, err := p.manager.MkApp(recognizer, []ast.ASTHandle{term})
	if err != nil {
		return nil, false, err
	}
	result := []ast.ASTHandle{isConstructor}
	accessors, err := p.manager.DatatypeConstructorAccessors(constructor)
	if err != nil {
		return nil, false, err
	}
	termConstructor := false
	if termKind, kindErr := p.manager.GetKind(term); kindErr != nil {
		return nil, false, kindErr
	} else if termKind == ast.ASTApp {
		termDecl, declErr := p.manager.GetDecl(term)
		if declErr != nil {
			return nil, false, declErr
		}
		termConstructor = termDecl == constructor
	}
	for index, accessor := range accessors {
		var selected ast.ASTHandle
		if termConstructor {
			selected, err = p.manager.GetArg(term, uint32(index))
		} else {
			selected, err = p.manager.MkApp(accessor, []ast.ASTHandle{term})
		}
		if err != nil {
			return nil, false, err
		}
		field, err := p.manager.GetArg(value, uint32(index))
		if err != nil {
			return nil, false, err
		}
		equality, err := p.manager.MkEq(selected, field)
		if err != nil {
			return nil, false, err
		}
		result = append(result, equality)
	}
	return result, true, nil
}

// expandInductiveBitVectorEquality treats value as the numeral side.
func (p *PredicateTransformer) expandInductiveBitVectorEquality(term, value ast.ASTHandle) ([]ast.ASTHandle, bool, error) {
	text, matched, err := p.manager.BitVectorNumeralText(value)
	if err != nil {
		return nil, false, fmt.Errorf("read bit-vector numeral text: %w", err)
	}
	if !matched {
		return nil, false, nil
	}
	width, err := p.manager.BitVectorSize(value)
	if err != nil {
		return nil, false, fmt.Errorf("read bit-vector numeral width: %w", err)
	}
	if width == 0 {
		return nil, false, ast.ErrASTDomain
	}
	integer := new(big.Int)
	if _, ok := integer.SetString(text, 10); !ok {
		return nil, false, ast.ErrASTDomain
	}
	one, err := p.manager.BitVectorNumeralUnsigned(1, 1)
	if err != nil {
		return nil, false, fmt.Errorf("construct bit-vector one: %w", err)
	}
	result := make([]ast.ASTHandle, 0, width)
	for bit := uint32(0); bit < width; bit++ {
		extract, err := p.manager.MkBitVectorExtract(bit, bit, term)
		if err != nil {
			return nil, false, err
		}
		literal, err := p.manager.MkEq(one, extract)
		if err != nil {
			return nil, false, err
		}
		if integer.Bit(int(bit)) == 0 {
			literal, err = p.manager.MkNot(literal)
			if err != nil {
				return nil, false, err
			}
		}
		result = append(result, literal)
	}
	return result, true, nil
}

func (p *PredicateTransformer) generalizeInductiveLemma(obligation *ProofObligation, lemma *Lemma) (*Lemma, error) {
	cube, err := lemma.Cube()
	if err != nil || len(cube) == 0 {
		return lemma, err
	}
	p.boolGeneralizations++
	started := time.Now()
	defer func() { p.boolGeneralizerTime += time.Since(started) }()
	level, err := lemma.Level()
	if err != nil {
		return lemma, err
	}
	truth, err := p.manager.MkTrue()
	if err != nil {
		return lemma, err
	}
	processed := make([]ast.ASTHandle, 0, len(cube))
	dirty := false
	usesLevel := level
	for index := 0; index < len(cube); {
		literal := cube[index]
		cube[index] = truth
		var core []ast.ASTHandle
		var used uint32
		var inductive bool
		if len(cube) > 1 {
			core, used, inductive, err = p.checkInductiveCube(cube, level)
			if err != nil {
				return lemma, err
			}
			if inductive {
				cube = core
				usesLevel = used
				dirty = true
				index = 0
				for index < len(cube) && containsGeneralizedLiteral(processed, cube[index]) {
					index++
				}
				continue
			}
		}
		expanded, expandErr := p.expandInductiveLiteral(literal)
		if expandErr != nil {
			return lemma, expandErr
		}
		found := false
		if len(expanded) > 1 || expanded[0] != literal {
			for _, replacement := range expanded {
				cube[index] = replacement
				core, used, inductive, err = p.checkInductiveCube(cube, level)
				if err != nil {
					return lemma, err
				}
				if inductive {
					cube = core
					usesLevel = used
					dirty = true
					found = true
					processed = append(processed, replacement)
					index = 0
					for index < len(cube) && containsGeneralizedLiteral(processed, cube[index]) {
						index++
					}
					break
				}
			}
		}
		if found {
			continue
		}
		cube[index] = literal
		processed = append(processed, literal)
		p.boolGeneralizerFailures++
		index++
	}
	if !dirty {
		return lemma, nil
	}
	return NewLemmaFromProofObligation(obligation, cube, usesLevel)
}

func (p *PredicateTransformer) checkLemmaSanity(lemma *Lemma) error {
	cube, err := lemma.Cube()
	if err != nil {
		return err
	}
	level, err := lemma.Level()
	if err != nil {
		return err
	}
	_, _, inductive, err := p.checkInductiveCube(cube, level)
	if err != nil {
		return err
	}
	if !inductive {
		return ErrLemmaSanity
	}
	return nil
}
