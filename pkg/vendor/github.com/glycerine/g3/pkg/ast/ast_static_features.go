package ast

// Source: src/ast/static_features.cpp:22-112,537-649;
// src/ast/static_features.h:27-179.

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// StaticFeatures owns the mutable counters and caches used by the source
// benchmark classifier. Collection is added in the same source-defined phases;
// Reset and the query/display surface are complete here.
type StaticFeatures struct {
	manager                           *ASTManager
	basicFID, arithFID, bitVectorFID  FamilyID
	labelFID, arrayFID, fpaFID        FamilyID
	sequenceFID                       FamilyID
	labelSym, patternSym, exprListSym usymbol.Symbol

	visited                                                                       map[ASTHandle]struct{}
	CNF                                                                           bool
	NumExprs, NumRoots, MaxDepth                                                  uint32
	NumQuantifiers, NumQuantifiersWithPatterns, NumQuantifiersWithMultiPatterns   uint32
	NumClauses, NumBinClauses, NumUnits, SumClauseSize                            uint32
	NumNestedFormulas, NumBoolExprs, NumBoolConstants                             uint32
	NumFormulaTrees, MaxFormulaDepth, SumFormulaDepth                             uint32
	NumOrAndTrees, MaxOrAndTreeDepth, SumOrAndTreeDepth                           uint32
	NumITETrees, MaxITETreeDepth, SumITETreeDepth                                 uint32
	NumANDs, NumORs, NumIFFs, NumITEFormulas, NumITETerms, NumSharing             uint32
	NumInterpretedExprs, NumUninterpretedExprs                                    uint32
	NumInterpretedConstants, NumUninterpretedConstants, NumUninterpretedFunctions uint32
	NumEqualities                                                                 uint32
	HasRational, HasInt, HasReal, HasBitVector, HasFloatingPoint                  bool
	HasString, HasNonStringSequence, HasArrays                                    bool
	ArithKSum                                                                     unumeral.OwnedRational
	arithScratch                                                                  unumeral.OwnedRational
	NumArithTerms, NumArithEqualities, NumArithInequalities                       uint32
	NumDiffTerms, NumDiffEqualities, NumDiffInequalities                          uint32
	NumSimpleEqualities, NumSimpleInequalities, NumNonLinear                      uint32
	NumApps                                                                       map[uint32]uint32
	NumTheoryTerms, NumTheoryAtoms, NumTheoryConstants, NumTheoryEqualities       []uint32
	NumAliens                                                                     uint32
	NumAliensPerFamily                                                            []uint32
	exprDepth                                                                     map[uint32]uint32
	orAndDepth, iteDepth, formulaDepth                                            map[uint32]uint32
	NumMarkedTheories                                                             uint32
	theories                                                                      []bool
	MaxStackDepth                                                                 uint32
	closed                                                                        bool
}

func NewStaticFeatures(m *ASTManager) (*StaticFeatures, error) {
	if m == nil {
		return nil, ErrASTHandle
	}
	mkFamily := func(name string) (FamilyID, error) {
		s, err := m.Symbols.Intern(name)
		if err != nil {
			return NullFamilyID, err
		}
		return m.families.MkID(s)
	}
	families := make([]FamilyID, 5)
	var err error
	for i, name := range []string{"arith", "bv", "array", "fpa", "seq"} {
		families[i], err = mkFamily(name)
		if err != nil {
			return nil, err
		}
	}
	lfid, err := mkFamily("label")
	if err != nil {
		return nil, err
	}
	syms := make([]usymbol.Symbol, 3)
	for i, name := range []string{"label", "pattern", "expr-list"} {
		syms[i], err = m.Symbols.Intern(name)
		if err != nil {
			return nil, err
		}
	}
	f := &StaticFeatures{manager: m, basicFID: m.BasicFamilyID(), arithFID: families[0], bitVectorFID: families[1],
		arrayFID: families[2], fpaFID: families[3], sequenceFID: families[4], labelFID: lfid,
		labelSym: syms[0], patternSym: syms[1], exprListSym: syms[2]}
	f.ArithKSum, err = m.numerals.NewValue()
	if err != nil {
		return nil, err
	}
	f.arithScratch, err = m.numerals.NewValue()
	if err != nil {
		_ = m.numerals.Del(&f.ArithKSum)
		return nil, err
	}
	if err = f.Reset(); err != nil {
		_ = m.numerals.Del(&f.arithScratch)
		_ = m.numerals.Del(&f.ArithKSum)
		return nil, err
	}
	return f, nil
}

func (f *StaticFeatures) open() error {
	if f == nil || f.manager == nil || f.closed {
		return ErrASTHandle
	}
	return nil
}

func (f *StaticFeatures) FlushCache() error {
	if err := f.open(); err != nil {
		return err
	}
	f.exprDepth = make(map[uint32]uint32)
	f.orAndDepth = make(map[uint32]uint32)
	f.iteDepth = make(map[uint32]uint32)
	f.formulaDepth = make(map[uint32]uint32)
	return nil
}

func (f *StaticFeatures) Reset() error {
	if err := f.open(); err != nil {
		return err
	}
	f.visited = make(map[ASTHandle]struct{})
	f.CNF = true
	f.NumExprs, f.NumRoots, f.MaxDepth = 0, 0, 0
	f.NumQuantifiers, f.NumQuantifiersWithPatterns, f.NumQuantifiersWithMultiPatterns = 0, 0, 0
	f.NumClauses, f.NumBinClauses, f.NumUnits, f.SumClauseSize = 0, 0, 0, 0
	f.NumNestedFormulas, f.NumBoolExprs, f.NumBoolConstants = 0, 0, 0
	f.NumFormulaTrees, f.MaxFormulaDepth, f.SumFormulaDepth = 0, 0, 0
	f.NumOrAndTrees, f.MaxOrAndTreeDepth, f.SumOrAndTreeDepth = 0, 0, 0
	f.NumITETrees, f.MaxITETreeDepth, f.SumITETreeDepth = 0, 0, 0
	f.NumANDs, f.NumORs, f.NumIFFs, f.NumITEFormulas, f.NumITETerms, f.NumSharing = 0, 0, 0, 0, 0, 0
	f.NumInterpretedExprs, f.NumUninterpretedExprs = 0, 0
	f.NumInterpretedConstants, f.NumUninterpretedConstants, f.NumUninterpretedFunctions = 0, 0, 0
	f.NumEqualities = 0
	f.HasRational, f.HasInt, f.HasReal, f.HasBitVector, f.HasFloatingPoint = false, false, false, false, false
	f.HasString, f.HasNonStringSequence, f.HasArrays = false, false, false
	if err := f.manager.numerals.Reset(&f.ArithKSum); err != nil {
		return err
	}
	if err := f.manager.numerals.Reset(&f.arithScratch); err != nil {
		return err
	}
	f.NumArithTerms, f.NumArithEqualities, f.NumArithInequalities = 0, 0, 0
	f.NumDiffTerms, f.NumDiffEqualities, f.NumDiffInequalities = 0, 0, 0
	f.NumSimpleEqualities, f.NumSimpleInequalities, f.NumNonLinear = 0, 0, 0
	f.NumApps, f.NumTheoryTerms, f.NumTheoryAtoms = make(map[uint32]uint32), nil, nil
	f.NumTheoryConstants, f.NumTheoryEqualities = nil, nil
	f.NumAliens, f.NumAliensPerFamily = 0, nil
	f.NumMarkedTheories, f.theories = 0, nil
	f.MaxStackDepth = 500
	return f.FlushCache()
}

func (f *StaticFeatures) HasUF() bool { return f != nil && f.NumUninterpretedFunctions > 0 }
func (f *StaticFeatures) NumNonUFTheories() uint32 {
	if f == nil {
		return 0
	}
	return f.NumMarkedTheories
}
func (f *StaticFeatures) NumTheories() uint32 {
	if f == nil {
		return 0
	}
	n := f.NumMarkedTheories
	if f.HasUF() {
		n++
	}
	return n
}

func (f *StaticFeatures) ArithKSumIsSmall() (bool, error) {
	if err := f.open(); err != nil {
		return false, err
	}
	return f.manager.numerals.LessInt(&f.ArithKSum, 2147483647/8)
}

func (f *StaticFeatures) internalFamily(s usymbol.Symbol) bool {
	return s == f.labelSym || s == f.patternSym || s == f.exprListSym
}

func boolInt(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}

func (f *StaticFeatures) displayFamilyData(out io.Writer, prefix string, data []uint32) error {
	for i, value := range data {
		name, err := f.manager.families.GetName(FamilyID(i))
		if err != nil {
			return err
		}
		if f.internalFamily(name) {
			continue
		}
		text, err := f.manager.Symbols.Display(name)
		if err != nil {
			return err
		}
		if _, err = fmt.Fprintln(out, prefix+"_"+text, value); err != nil {
			return err
		}
	}
	return nil
}

func (f *StaticFeatures) DisplayPrimitive(out io.Writer) error {
	if err := f.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrASTHandle
	}
	k, err := f.manager.numerals.String(&f.ArithKSum)
	if err != nil {
		return err
	}
	lines := []struct {
		name  string
		value any
	}{
		{"CNF", boolInt(f.CNF)}, {"NUM_EXPRS", f.NumExprs}, {"NUM_ROOTS", f.NumRoots}, {"MAX_DEPTH", f.MaxDepth},
		{"NUM_QUANTIFIERS", f.NumQuantifiers}, {"NUM_QUANTIFIERS_WITH_PATTERNS", f.NumQuantifiersWithPatterns}, {"NUM_QUANTIFIERS_WITH_MULTI_PATTERNS", f.NumQuantifiersWithMultiPatterns},
		{"NUM_CLAUSES", f.NumClauses}, {"NUM_BIN_CLAUSES", f.NumBinClauses}, {"NUM_UNITS", f.NumUnits}, {"SUM_CLAUSE_SIZE", f.SumClauseSize},
		{"NUM_NESTED_FORMULAS", f.NumNestedFormulas}, {"NUM_BOOL_EXPRS", f.NumBoolExprs}, {"NUM_BOOL_CONSTANTS", f.NumBoolConstants},
		{"NUM_FORMULA_TREES", f.NumFormulaTrees}, {"MAX_FORMULA_DEPTH", f.MaxFormulaDepth}, {"SUM_FORMULA_DEPTH", f.SumFormulaDepth},
		{"NUM_OR_AND_TREES", f.NumOrAndTrees}, {"MAX_OR_AND_TREE_DEPTH", f.MaxOrAndTreeDepth}, {"SUM_OR_AND_TREE_DEPTH", f.SumOrAndTreeDepth},
		{"NUM_ITE_TREES", f.NumITETrees}, {"MAX_ITE_TREE_DEPTH", f.MaxITETreeDepth}, {"SUM_ITE_TREE_DEPTH", f.SumITETreeDepth},
		{"NUM_ORS", f.NumORs}, {"NUM_ANDS", f.NumANDs}, {"NUM_IFFS", f.NumIFFs}, {"NUM_ITE_FORMULAS", f.NumITEFormulas}, {"NUM_ITE_TERMS", f.NumITETerms},
		{"NUM_SHARING", f.NumSharing}, {"NUM_INTERPRETED_EXPRS", f.NumInterpretedExprs}, {"NUM_UNINTERPRETED_EXPRS", f.NumUninterpretedExprs},
		{"NUM_INTERPRETED_CONSTANTS", f.NumInterpretedConstants}, {"NUM_UNINTERPRETED_CONSTANTS", f.NumUninterpretedConstants}, {"NUM_UNINTERPRETED_FUNCTIONS", f.NumUninterpretedFunctions},
		{"NUM_EQS", f.NumEqualities}, {"HAS_RATIONAL", boolInt(f.HasRational)}, {"HAS_INT", boolInt(f.HasInt)}, {"HAS_REAL", boolInt(f.HasReal)}, {"ARITH_K_SUM", k},
		{"NUM_ARITH_TERMS", f.NumArithTerms}, {"NUM_ARITH_EQS", f.NumArithEqualities}, {"NUM_ARITH_INEQS", f.NumArithInequalities},
		{"NUM_DIFF_TERMS", f.NumDiffTerms}, {"NUM_DIFF_EQS", f.NumDiffEqualities}, {"NUM_DIFF_INEQS", f.NumDiffInequalities},
		{"NUM_SIMPLE_EQS", f.NumSimpleEqualities}, {"NUM_SIMPLE_INEQS", f.NumSimpleInequalities}, {"NUM_NON_LINEAR", f.NumNonLinear}, {"NUM_ALIENS", f.NumAliens},
	}
	if _, err = fmt.Fprintln(out, "BEGIN_PRIMITIVE_STATIC_FEATURES"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err = fmt.Fprintln(out, line.name, line.value); err != nil {
			return err
		}
	}
	for _, data := range []struct {
		prefix string
		values []uint32
	}{{"NUM_TERMS", f.NumTheoryTerms}, {"NUM_ATOMS", f.NumTheoryAtoms}, {"NUM_CONSTANTS", f.NumTheoryConstants}, {"NUM_EQS", f.NumTheoryEqualities}, {"NUM_ALIENS", f.NumAliensPerFamily}} {
		if err = f.displayFamilyData(out, data.prefix, data.values); err != nil {
			return err
		}
	}
	if _, err = fmt.Fprintln(out, "NUM_THEORIES", f.NumTheories()); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "END_PRIMITIVE_STATIC_FEATURES")
	return err
}

func featureRatio(n, d uint32) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func featureFloat(v float64) string { return strconv.FormatFloat(v, 'g', 6, 64) }

func (f *StaticFeatures) Display(out io.Writer) error {
	if err := f.open(); err != nil {
		return err
	}
	if out == nil {
		return ErrASTHandle
	}
	isDiff := f.NumArithEqualities == f.NumDiffEqualities && f.NumArithInequalities == f.NumDiffInequalities && f.NumArithTerms == f.NumDiffTerms
	lines := []struct {
		name  string
		value string
	}{
		{"CNF", strconv.FormatUint(uint64(boolInt(f.CNF)), 10)}, {"MAX_DEPTH", strconv.FormatUint(uint64(f.MaxDepth), 10)},
		{"MAX_OR_AND_TREE_DEPTH", strconv.FormatUint(uint64(f.MaxOrAndTreeDepth), 10)}, {"MAX_ITE_TREE_DEPTH", strconv.FormatUint(uint64(f.MaxITETreeDepth), 10)},
		{"HAS_INT", strconv.FormatUint(uint64(boolInt(f.HasInt)), 10)}, {"HAS_REAL", strconv.FormatUint(uint64(boolInt(f.HasReal)), 10)},
		{"HAS_QUANTIFIERS", strconv.FormatUint(uint64(boolInt(f.NumQuantifiers > 0)), 10)},
		{"PERC_QUANTIFIERS_WITH_PATTERNS", featureFloat(featureRatio(f.NumQuantifiersWithPatterns, f.NumQuantifiers))},
		{"PERC_QUANTIFIERS_WITH_MULTI_PATTERNS", featureFloat(featureRatio(f.NumQuantifiersWithMultiPatterns, f.NumQuantifiers))},
		{"IS_NON_LINEAR", strconv.FormatUint(uint64(boolInt(f.NumNonLinear > 0)), 10)}, {"THEORY_COMBINATION", strconv.FormatUint(uint64(boolInt(f.NumTheories() > 1)), 10)},
		{"AVG_CLAUSE_SIZE", featureFloat(featureRatio(f.SumClauseSize, f.NumClauses))},
		{"PERC_BOOL_CONSTANTS", featureFloat(featureRatio(f.NumBoolConstants, f.NumUninterpretedConstants))},
		{"PERC_NESTED_FORMULAS", featureFloat(featureRatio(f.NumNestedFormulas, f.NumBoolExprs))},
		{"IS_DIFF", strconv.FormatUint(uint64(boolInt(isDiff)), 10)}, {"INEQ_EQ_RATIO", featureFloat(featureRatio(f.NumArithInequalities, f.NumArithEqualities))},
		{"PERC_ARITH_EQS", featureFloat(featureRatio(f.NumArithEqualities, f.NumEqualities))}, {"PERC_DIFF_EQS", featureFloat(featureRatio(f.NumDiffEqualities, f.NumArithEqualities))},
		{"PERC_DIFF_INEQS", featureFloat(featureRatio(f.NumDiffInequalities, f.NumArithInequalities))}, {"PERC_SIMPLE_EQS", featureFloat(featureRatio(f.NumSimpleEqualities, f.NumArithEqualities))},
		{"PERC_SIMPLE_INEQS", featureFloat(featureRatio(f.NumSimpleInequalities, f.NumArithInequalities))}, {"PERC_ALIENS", featureFloat(featureRatio(f.NumAliens, f.NumExprs))},
	}
	if _, err := fmt.Fprintln(out, "BEGIN_STATIC_FEATURES"); err != nil {
		return err
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line.name, line.value); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(out, "END_STATIC_FEATURES")
	return err
}

// GetFeatureVector retains the empty source method.
func (*StaticFeatures) GetFeatureVector(result []float64) []float64 { return result }

func (f *StaticFeatures) Close() error {
	if f == nil || f.manager == nil {
		return ErrASTHandle
	}
	if f.closed {
		return nil
	}
	err := errors.Join(f.manager.numerals.Del(&f.arithScratch), f.manager.numerals.Del(&f.ArithKSum))
	f.closed = true
	f.visited, f.orAndDepth, f.iteDepth, f.formulaDepth = nil, nil, nil, nil
	return err
}
