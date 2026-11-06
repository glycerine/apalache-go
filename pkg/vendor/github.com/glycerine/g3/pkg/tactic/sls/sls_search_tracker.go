package sls

// Source: src/tactic/sls/sls_tracker.h:370-479,572-1105.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

var ErrSLSSearch = errors.New("sls: unsupported or invalid search state")

// SearchTracker owns the source tracker state that is consumed only by the
// stochastic engine: random bits, assertion-local constant occurrences and
// unsatisfied-assertion selection cursors. The numerical score table remains
// owned by Tracker.
type SearchTracker struct {
	self            *SearchTracker
	base            *Tracker
	random          z3rand.Generator
	randomBits      uint32
	randomBitsCount uint32
	constantsOcc    map[ast.ASTHandle][]ast.ASTHandle
	lastPosition    uint32
	hasLastPosition bool
	temporarySeen   map[ast.ASTHandle]struct{}
	closed          bool
}

func NewSearchTracker(base *Tracker, seed uint32) (*SearchTracker, error) {
	if base == nil || base.open() != nil {
		return nil, ErrSLSSearch
	}
	s := &SearchTracker{base: base, constantsOcc: make(map[ast.ASTHandle][]ast.ASTHandle), temporarySeen: make(map[ast.ASTHandle]struct{})}
	s.random.SetSeed(seed)
	s.self = s
	return s, nil
}

func (s *SearchTracker) open() error {
	if s == nil || s.self != s || s.closed || s.base == nil || s.base.open() != nil {
		return ErrSLSSearch
	}
	return nil
}

func (s *SearchTracker) Close() error {
	if s == nil || s.self != s {
		return ErrSLSSearch
	}
	if s.closed {
		return nil
	}
	s.constantsOcc = nil
	s.temporarySeen = nil
	s.closed = true
	return nil
}

func (s *SearchTracker) SetRandomSeed(seed uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	s.random.SetSeed(seed)
	return nil
}

func (s *SearchTracker) refillRandom() {
	s.randomBits = s.random.Next()
	s.randomBitsCount = 15
}

func (s *SearchTracker) RandomBool() (*numeral.Int, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.randomBitsCount == 0 {
		s.refillRandom()
	}
	one := s.randomBits&1 != 0
	s.randomBits >>= 1
	s.randomBitsCount--
	if one {
		return &s.base.one, nil
	}
	return &s.base.zero, nil
}

// RandomUint preserves the source's unusual shift-after-increment bit order.
func (s *SearchTracker) RandomUint(bits uint32) (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if s.randomBitsCount == 0 {
		s.refillRandom()
	}
	var value uint32
	for bits != 0 {
		if s.randomBits&1 != 0 {
			value++
		}
		value <<= 1
		s.randomBits >>= 1
		s.randomBitsCount--
		if s.randomBitsCount == 0 {
			s.refillRandom()
		}
		bits--
	}
	return value, nil
}

func (s *SearchTracker) RandomValue(sort ast.ASTHandle, out *numeral.Int) (result error) {
	if err := s.open(); err != nil || out == nil {
		if err != nil {
			return err
		}
		return ErrSLSSearch
	}
	boolSort, err := s.base.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort == boolSort {
		value, err := s.RandomBool()
		if err != nil {
			return err
		}
		return s.base.numbers.Copy(out, value)
	}
	isBV, err := s.base.manager.IsBitVectorSort(sort)
	if err != nil || !isBV {
		if err != nil {
			return err
		}
		return ErrEvaluatorUnsupported
	}
	bits, err := s.base.manager.BitVectorSize(sort)
	if err != nil {
		return err
	}
	if err = s.base.numbers.Copy(out, &s.base.zero); err != nil {
		return err
	}
	temp := numeral.NewInt(0)
	defer func() { result = errors.Join(result, s.base.numbers.Del(&temp)) }()
	for bits != 0 {
		if err = s.base.numbers.Mul(out, &s.base.two, &temp); err != nil {
			return err
		}
		bit, x := s.RandomBool()
		if x != nil {
			return x
		}
		if err = s.base.numbers.Add(&temp, bit, out); err != nil {
			return err
		}
		bits--
	}
	return nil
}

// Initialize completes the source tracker initialization phases omitted from
// the structural foundation: occurrence lists and Boolean prune polarities.
func (s *SearchTracker) Initialize(roots []ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	if err := s.base.InitializeExpressions(roots); err != nil {
		return err
	}
	for _, root := range roots {
		seen := make(map[ast.ASTHandle]struct{})
		declSeen := make(map[ast.ASTHandle]struct{})
		var decls []ast.ASTHandle
		var walk func(ast.ASTHandle) error
		walk = func(node ast.ASTHandle) error {
			if _, found := seen[node]; found {
				return nil
			}
			seen[node] = struct{}{}
			n, err := s.base.manager.GetNumArgs(node)
			if err != nil {
				return err
			}
			if n == 0 {
				family, x := s.base.manager.GetFamilyID(node)
				if x != nil {
					return x
				}
				if family == ast.NullFamilyID {
					decl, x := s.base.manager.GetDecl(node)
					if x != nil {
						return x
					}
					if _, found := declSeen[decl]; !found {
						declSeen[decl] = struct{}{}
						decls = append(decls, decl)
					}
				}
				return nil
			}
			for i := uint32(0); i < n; i++ {
				child, x := s.base.manager.GetArg(node, i)
				if x != nil {
					return x
				}
				if x = walk(child); x != nil {
					return x
				}
			}
			return nil
		}
		if err := walk(root); err != nil {
			return err
		}
		s.constantsOcc[root] = decls
		if err := s.setupOccurrences(root, false); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchTracker) setupOccurrences(node ast.ASTHandle, negated bool) error {
	sort, err := s.base.manager.GetSort(node)
	if err != nil {
		return err
	}
	boolSort, err := s.base.manager.MkBoolSort()
	if err != nil {
		return err
	}
	if sort != boolSort {
		isBV, x := s.base.manager.IsBitVectorSort(sort)
		if x != nil || !isBV {
			if x != nil {
				return x
			}
			return ErrEvaluatorUnsupported
		}
		return nil
	}
	family, err := s.base.manager.GetFamilyID(node)
	if err != nil {
		return err
	}
	op, err := s.base.manager.GetDeclKind(node)
	if err != nil {
		return err
	}
	if family == s.base.manager.BasicFamilyID() && (op == ast.ASTOpAnd || op == ast.ASTOpOr) {
		if negated {
			return ErrSLSSearch
		}
		n, err := s.base.manager.GetNumArgs(node)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			child, x := s.base.manager.GetArg(node, i)
			if x != nil {
				return x
			}
			if _, found := s.temporarySeen[child]; !found {
				if x = s.setupOccurrences(child, false); x != nil {
					return x
				}
				s.temporarySeen[child] = struct{}{}
			}
		}
		return nil
	}
	if family == s.base.manager.BasicFamilyID() && op == ast.ASTOpNot {
		if negated {
			return ErrSLSSearch
		}
		child, err := s.base.manager.GetArg(node, 0)
		if err != nil {
			return err
		}
		return s.setupOccurrences(child, true)
	}
	score, err := s.base.existingScore(node)
	if err != nil {
		return err
	}
	if negated {
		score.hasNegOcc = 1
	} else {
		score.hasPosOcc = 1
	}
	return nil
}

func (s *SearchTracker) Reset() error {
	if err := s.open(); err != nil {
		return err
	}
	for _, entry := range s.base.entryPoints {
		if err := s.base.SetValue(entry, &s.base.zero); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchTracker) Randomize() (result error) {
	if err := s.open(); err != nil {
		return err
	}
	temp := numeral.NewInt(0)
	defer func() { result = errors.Join(result, s.base.numbers.Del(&temp)) }()
	for decl, entry := range s.base.entryPoints {
		sort, err := s.base.manager.GetRange(decl)
		if err != nil {
			return err
		}
		if err = s.RandomValue(sort, &temp); err != nil {
			return err
		}
		if err = s.base.SetValue(entry, &temp); err != nil {
			return err
		}
	}
	return nil
}

func (s *SearchTracker) Score(node ast.ASTHandle) (float64, error) {
	sort, err := s.base.manager.GetSort(node)
	if err != nil {
		return 0, err
	}
	boolSort, err := s.base.manager.MkBoolSort()
	if err != nil {
		return 0, err
	}
	if sort == boolSort {
		return s.scoreBool(node, false)
	}
	isBV, err := s.base.manager.IsBitVectorSort(sort)
	if err != nil || !isBV {
		if err != nil {
			return 0, err
		}
		return 0, ErrEvaluatorUnsupported
	}
	return 0, nil
}

func (s *SearchTracker) scoreBool(node ast.ASTHandle, negated bool) (result float64, resultErr error) {
	family, err := s.base.manager.GetFamilyID(node)
	if err != nil {
		return 0, err
	}
	op, err := s.base.manager.GetDeclKind(node)
	if err != nil {
		return 0, err
	}
	n, err := s.base.manager.GetNumArgs(node)
	if err != nil {
		return 0, err
	}
	if n == 0 && family == ast.NullFamilyID {
		value, err := s.base.GetValue(node)
		if err != nil {
			return 0, err
		}
		one, err := s.base.numbers.IsOne(value)
		if err != nil {
			return 0, err
		}
		if negated {
			one = !one
		}
		if one {
			return 1, nil
		}
		return 0, nil
	}
	arg := func(i uint32) (ast.ASTHandle, error) { return s.base.manager.GetArg(node, i) }
	switch {
	case family == s.base.manager.BasicFamilyID() && op == ast.ASTOpAnd:
		if negated {
			return 0, ErrSLSSearch
		}
		result = 1
		for i := uint32(0); i < n; i++ {
			child, x := arg(i)
			if x != nil {
				return 0, x
			}
			score, x := s.base.Score(child)
			if x != nil {
				return 0, x
			}
			if score < result {
				result = score
			}
		}
	case family == s.base.manager.BasicFamilyID() && op == ast.ASTOpOr:
		if negated {
			return 0, ErrSLSSearch
		}
		for i := uint32(0); i < n; i++ {
			child, x := arg(i)
			if x != nil {
				return 0, x
			}
			score, x := s.base.Score(child)
			if x != nil {
				return 0, x
			}
			if score > result {
				result = score
			}
		}
	case family == s.base.manager.BasicFamilyID() && op == ast.ASTOpIte:
		condition, x := arg(0)
		if x != nil {
			return 0, x
		}
		value, x := s.base.GetValue(condition)
		if x != nil {
			return 0, x
		}
		one, x := s.base.numbers.IsOne(value)
		if x != nil {
			return 0, x
		}
		branch := uint32(2)
		if one {
			branch = 1
		}
		child, x := arg(branch)
		if x != nil {
			return 0, x
		}
		return s.base.Score(child)
	case family == s.base.manager.BasicFamilyID() && (op == ast.ASTOpEq || op == ast.ASTOpIff):
		if n != 2 {
			return 0, ErrSLSSearch
		}
		left, x := arg(0)
		if x != nil {
			return 0, x
		}
		right, x := arg(1)
		if x != nil {
			return 0, x
		}
		lv, x := s.base.GetValue(left)
		if x != nil {
			return 0, x
		}
		rv, x := s.base.GetValue(right)
		if x != nil {
			return 0, x
		}
		equal, x := s.base.numbers.Equal(lv, rv)
		if x != nil {
			return 0, x
		}
		if negated {
			if !equal {
				result = 1
			}
			break
		}
		leftSort, x := s.base.manager.GetSort(left)
		if x != nil {
			return 0, x
		}
		boolSort, x := s.base.manager.MkBoolSort()
		if x != nil {
			return 0, x
		}
		if leftSort == boolSort {
			if equal {
				result = 1
			}
			break
		}
		isBV, x := s.base.manager.IsBitVectorSort(leftSort)
		if x != nil || !isBV {
			if x != nil {
				return 0, x
			}
			return 0, ErrEvaluatorUnsupported
		}
		bits, x := s.base.manager.BitVectorSize(leftSort)
		if x != nil || bits == 0 {
			return 0, x
		}
		diff := numeral.NewInt(0)
		defer func() { resultErr = errors.Join(resultErr, s.base.numbers.Del(&diff)) }()
		if x = s.base.numbers.BitwiseXor(lv, rv, &diff); x != nil {
			return 0, x
		}
		var distance uint32
		for {
			zero, x := s.base.numbers.IsZero(&diff)
			if x != nil {
				return 0, x
			}
			if zero {
				break
			}
			even, x := s.base.numbers.IsEven(&diff)
			if x != nil {
				return 0, x
			}
			if !even {
				distance++
			}
			if x = s.base.numbers.MachineDiv(&diff, &s.base.two, &diff); x != nil {
				return 0, x
			}
		}
		result = 1 - float64(distance)/float64(bits)
	case family == s.base.manager.BasicFamilyID() && op == ast.ASTOpNot:
		if negated || n != 1 {
			return 0, ErrSLSSearch
		}
		child, x := arg(0)
		if x != nil {
			return 0, x
		}
		result, resultErr = s.scoreBool(child, true)
	case family == s.base.manager.BasicFamilyID() && op == ast.ASTOpDistinct:
		var pairs, distinct uint32
		for i := uint32(0); i < n; i++ {
			for j := i + 1; j < n; j++ {
				// Preserve the source implementation's reached arg(0)/arg(1) reads.
				left, x := arg(0)
				if x != nil {
					return 0, x
				}
				right, x := arg(1)
				if x != nil {
					return 0, x
				}
				lv, x := s.base.GetValue(left)
				if x != nil {
					return 0, x
				}
				rv, x := s.base.GetValue(right)
				if x != nil {
					return 0, x
				}
				equal, x := s.base.numbers.Equal(lv, rv)
				if x != nil {
					return 0, x
				}
				pairs++
				if !equal {
					distinct++
				}
			}
		}
		if pairs == 0 {
			return 0, ErrSLSSearch
		}
		result = float64(distinct) / float64(pairs)
		if negated {
			result = 1 - result
		}
	default:
		bvFamily, x := s.base.manager.BitVectorUtilityFamilyID()
		if x != nil || family != bvFamily || (op != slsBVULE && op != slsBVSLE) || n != 2 {
			if x != nil {
				return 0, x
			}
			return 0, ErrEvaluatorUnsupported
		}
		left, x := arg(0)
		if x != nil {
			return 0, x
		}
		right, x := arg(1)
		if x != nil {
			return 0, x
		}
		lv, x := s.base.GetValue(left)
		if x != nil {
			return 0, x
		}
		rv, x := s.base.GetValue(right)
		if x != nil {
			return 0, x
		}
		bits, x := s.base.manager.BitVectorSize(left)
		if x != nil || bits == 0 {
			return 0, x
		}
		a, b := numeral.NewInt(0), numeral.NewInt(0)
		defer func() {
			resultErr = errors.Join(resultErr, s.base.numbers.Del(&b))
			resultErr = errors.Join(resultErr, s.base.numbers.Del(&a))
		}()
		if x = s.base.numbers.Copy(&a, lv); x != nil {
			return 0, x
		}
		if x = s.base.numbers.Copy(&b, rv); x != nil {
			return 0, x
		}
		if op == slsBVSLE {
			if x = s.signed(&a, bits); x != nil {
				return 0, x
			}
			if x = s.signed(&b, bits); x != nil {
				return 0, x
			}
		}
		cmp, x := s.base.numbers.Compare(&a, &b)
		if x != nil {
			return 0, x
		}
		satisfied := cmp <= 0
		if negated {
			satisfied = cmp > 0
		}
		if satisfied {
			result = 1
		} else {
			diff := numeral.NewInt(0)
			defer func() { resultErr = errors.Join(resultErr, s.base.numbers.Del(&diff)) }()
			if negated {
				if x = s.base.numbers.Sub(&b, &a, &diff); x == nil {
					x = s.base.numbers.Inc(&diff)
				}
			} else {
				x = s.base.numbers.Sub(&a, &b, &diff)
			}
			if x != nil {
				return 0, x
			}
			ratio, x := s.ratioPowerTwo(&diff, bits)
			if x != nil {
				return 0, x
			}
			result = 1 - ratio
			if result < 0 {
				result = 0
			} else if result > 1 {
				result = 1
			}
		}
	}
	if family, err := s.base.manager.GetFamilyID(node); err != nil {
		return 0, err
	} else if bvFamily, err := s.base.manager.BitVectorUtilityFamilyID(); err == nil && family == bvFamily && result < 1 {
		result *= s.base.scaleUnsat
	}
	if result < 0 || result > 1 || math.IsNaN(result) {
		return 0, ErrSLSSearch
	}
	return result, resultErr
}

func (s *SearchTracker) signed(v *numeral.Int, bits uint32) error {
	power, err := s.base.powers.borrow(bits)
	if err != nil {
		return err
	}
	half, err := s.base.powers.borrow(bits - 1)
	if err != nil {
		return err
	}
	negative, err := s.base.numbers.GreaterEqual(v, half)
	if err != nil || !negative {
		return err
	}
	return s.base.numbers.Sub(v, power, v)
}

func (s *SearchTracker) ratioPowerTwo(value *numeral.Int, bits uint32) (float64, error) {
	text, err := s.base.numbers.String(value)
	if err != nil {
		return 0, err
	}
	integer, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return 0, ErrSLSSearch
	}
	ratio, _ := new(big.Rat).SetFrac(integer, new(big.Int).Lsh(big.NewInt(1), uint(bits))).Float64()
	return ratio, nil
}

func (s *SearchTracker) TopScore(roots []ast.ASTHandle) (float64, error) {
	var sum float64
	for _, root := range roots {
		score, err := s.base.Score(root)
		if err != nil {
			return 0, err
		}
		sum += score
	}
	if err := s.base.SetTopSum(sum); err != nil {
		return 0, err
	}
	return sum, nil
}

func (s *SearchTracker) UnsatAssertion(roots []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(roots) == 1 {
		value, err := s.base.GetValue(roots[0])
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		one, err := s.base.numbers.IsOne(value)
		if err != nil || one {
			return ast.ASTHandle{}, false, err
		}
		return roots[0], true, nil
	}
	position := uint32(0)
	found := false
	if s.base.ucb {
		maxScore := -1.0
		for i, root := range roots {
			value, err := s.base.GetValue(root)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			one, err := s.base.numbers.IsOne(value)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if one {
				continue
			}
			entry, err := s.base.existingScore(root)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			noise, err := s.RandomUint(8)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			q := entry.score + s.base.ucbConstant*math.Sqrt(math.Log(float64(s.base.touched))/float64(entry.touched)) + s.base.ucbNoise*float64(noise)
			if q > maxScore {
				maxScore, position, found = q, uint32(i), true
			}
		}
		if found {
			s.base.touched++
			s.base.scores[roots[position]].touched++
		}
	} else {
		var count uint32
		for i, root := range roots {
			value, err := s.base.GetValue(root)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			one, err := s.base.numbers.IsOne(value)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if one {
				continue
			}
			count++
			random, err := s.RandomUint(16)
			if err != nil {
				return ast.ASTHandle{}, false, err
			}
			if random%count == 0 {
				position, found = uint32(i), true
			}
		}
	}
	if !found {
		return ast.ASTHandle{}, false, nil
	}
	s.lastPosition, s.hasLastPosition = position, true
	return roots[position], true, nil
}

func (s *SearchTracker) UnsatConstants(roots []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if s.base.walksat {
		root, found, err := s.UnsatAssertion(roots)
		if err != nil || !found {
			return nil, err
		}
		return append([]ast.ASTHandle(nil), s.constantsOcc[root]...), nil
	}
	if len(roots) == 1 {
		value, err := s.base.GetValue(roots[0])
		if err != nil {
			return nil, err
		}
		one, err := s.base.numbers.IsOne(value)
		if err != nil {
			return nil, err
		}
		if !one {
			return s.base.Constants()
		}
	}
	seen := make(map[ast.ASTHandle]struct{})
	var answer []ast.ASTHandle
	for _, root := range roots {
		value, err := s.base.GetValue(root)
		if err != nil {
			return nil, err
		}
		one, err := s.base.numbers.IsOne(value)
		if err != nil {
			return nil, err
		}
		if one {
			continue
		}
		for _, decl := range s.constantsOcc[root] {
			if _, found := seen[decl]; !found {
				seen[decl] = struct{}{}
				answer = append(answer, decl)
			}
		}
	}
	return answer, nil
}

func (s *SearchTracker) ConstantsForRoot(root ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	values, found := s.constantsOcc[root]
	if !found {
		return nil, ErrSLSSearch
	}
	return append([]ast.ASTHandle(nil), values...), nil
}

func (s *SearchTracker) NewUnsatAssertion(roots []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := s.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if len(roots) == 1 {
		return ast.ASTHandle{}, false, nil
	}
	var count, position uint32
	found := false
	for i, root := range roots {
		if s.hasLastPosition && uint32(i) == s.lastPosition {
			continue
		}
		value, err := s.base.GetValue(root)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		one, err := s.base.numbers.IsOne(value)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if one {
			continue
		}
		count++
		random, err := s.RandomUint(16)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if random%count == 0 {
			position, found = uint32(i), true
		}
	}
	if !found {
		return ast.ASTHandle{}, false, nil
	}
	return roots[position], true, nil
}

func (s *SearchTracker) IncreaseWeight(root ast.ASTHandle) error {
	if _, found := s.base.weights[root]; !found {
		return ErrSLSSearch
	}
	s.base.weights[root]++
	return nil
}

func (s *SearchTracker) DecreaseWeight(root ast.ASTHandle) error {
	weight, found := s.base.weights[root]
	if !found {
		return ErrSLSSearch
	}
	if weight > s.base.pawsInit {
		weight--
	} else {
		weight = s.base.pawsInit
	}
	s.base.weights[root] = weight
	return nil
}
