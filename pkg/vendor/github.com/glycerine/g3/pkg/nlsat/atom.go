package nlsat

// Source: src/nlsat/nlsat_types.h/.cpp.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	zhash "github.com/glycerine/g3/pkg/util/hash"
)

type AtomKind int32

const (
	AtomEQ AtomKind = iota
	AtomLT
	AtomGT
)
const (
	AtomRootEQ AtomKind = 10 + iota
	AtomRootLT
	AtomRootGT
	AtomRootLE
	AtomRootGE
)

const NullVariable uint32 = ^uint32(0)
const TrueBooleanVariable sat.BoolVar = 0
const TrueLiteral sat.Literal = 0
const FalseLiteral sat.Literal = 1

func atomAssert(file string, line int32, text string, ok bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/nlsat/"+file, line, text)
}

func (k AtomKind) Flip() (AtomKind, error) {
	if err := atomAssert("nlsat_types.h", 71, "k == EQ || k == LT || k == GT", k == AtomEQ || k == AtomLT || k == AtomGT); err != nil {
		return AtomEQ, err
	}
	switch k {
	case AtomLT:
		return AtomGT, nil
	case AtomGT:
		return AtomLT, nil
	default:
		return AtomEQ, nil
	}
}

func (k AtomKind) String() string {
	switch k {
	case AtomEQ:
		return "="
	case AtomLT:
		return "<"
	case AtomGT:
		return ">"
	case AtomRootEQ:
		return "= root"
	case AtomRootLT:
		return "< root"
	case AtomRootGT:
		return "> root"
	case AtomRootLE:
		return "<= root"
	case AtomRootGE:
		return ">= root"
	default:
		return strconv.FormatInt(int64(k), 10)
	}
}

// Atom is the common header. Polynomial references and atom storage belong to
// the solver; changing the atom reference count does not change polynomial refs.
type Atom struct {
	kind    AtomKind
	refs    uint32
	boolVar sat.BoolVar
	maxVar  uint32
}

func (a *Atom) initialize(k AtomKind, maxVar uint32) {
	*a = Atom{kind: k, boolVar: sat.NullBoolVar, maxVar: maxVar}
}
func (a *Atom) IsEquality() bool             { return a.kind == AtomEQ || a.kind == AtomRootEQ }
func (a *Atom) IsInequality() bool           { return a.kind <= AtomGT }
func (a *Atom) IsRoot() bool                 { return a.kind >= AtomRootEQ }
func (a *Atom) Kind() AtomKind               { return a.kind }
func (a *Atom) BooleanVariable() sat.BoolVar { return a.boolVar }
func (a *Atom) MaxVariable() uint32          { return a.maxVar }
func (a *Atom) RefCount() uint32             { return a.refs }
func (a *Atom) IncRef()                      { a.refs++ }
func (a *Atom) DecRef() error {
	if err := atomAssert("nlsat_types.h", 93, "m_ref_count > 0", a.refs > 0); err != nil {
		return err
	}
	a.refs--
	return nil
}

// InequalityFactor represents the source tagged pointer without pointer tags.
// Even records exponent parity, not the sign or degree of the polynomial itself.
type InequalityFactor struct {
	Polynomial polynomial.Polynomial
	Even       bool
}

type InequalityAtom struct {
	Atom
	factors []InequalityFactor
}

// initialize fills caller-owned storage, like the source placement constructor.
// The solver supplies that storage and owns both its allocation and polynomial
// references. No additional polynomial reference is acquired here.
func (a *InequalityAtom) initialize(k AtomKind, ps []polynomial.Polynomial, even []bool, maxVar uint32, storage []InequalityFactor) error {
	if len(ps) != len(even) || len(storage) != len(ps) || uint64(len(ps)) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	a.Atom.initialize(k, maxVar)
	a.factors = storage
	for i, p := range ps {
		a.factors[i] = InequalityFactor{Polynomial: p, Even: even[i]}
	}
	return atomAssert("nlsat_types.cpp", 32, "is_ineq_atom()", a.IsInequality())
}
func (a *InequalityAtom) Size() uint32 { return uint32(len(a.factors)) }
func (a *InequalityAtom) Polynomial(i uint32) (polynomial.Polynomial, error) {
	if err := atomAssert("nlsat_types.h", 106, "i < size()", uint64(i) < uint64(len(a.factors))); err != nil {
		return polynomial.Polynomial{}, err
	}
	if uint64(i) >= uint64(len(a.factors)) {
		return polynomial.Polynomial{}, arena.ErrBounds
	}
	return a.factors[i].Polynomial, nil
}
func (a *InequalityAtom) IsOdd(i uint32) (bool, error) {
	if err := atomAssert("nlsat_types.h", 108, "i < size()", uint64(i) < uint64(len(a.factors))); err != nil {
		return false, err
	}
	if uint64(i) >= uint64(len(a.factors)) {
		return false, arena.ErrBounds
	}
	return !a.factors[i].Even, nil
}
func (a *InequalityAtom) IsEven(i uint32) (bool, error) {
	odd, err := a.IsOdd(i)
	return !odd, err
}
func (a *InequalityAtom) Hash(pm *polynomial.MultivariatePolynomialCore) (uint32, error) {
	if pm == nil || len(a.factors) == 0 {
		return 0, arena.ErrConfig
	}
	var err error
	h := zhash.Composite(uint32(a.kind), a.Size(), func(i uint32) uint32 {
		if err != nil {
			return 0
		}
		var p polynomial.Polynomial
		p, err = a.Polynomial(i)
		if err != nil {
			return 0
		}
		var info polynomial.PolynomialInfo
		info, err = pm.Info(p)
		return info.ID
	})
	return h, err
}
func (a *InequalityAtom) Equal(b *InequalityAtom) bool {
	if a.kind != b.kind || len(a.factors) != len(b.factors) {
		return false
	}
	for i := range a.factors {
		if a.factors[i] != b.factors[i] {
			return false
		}
	}
	return true
}

type RootAtom struct {
	Atom
	x, index   uint32
	polynomial polynomial.Polynomial
}

func (a *RootAtom) initialize(k AtomKind, x, index uint32, p polynomial.Polynomial) error {
	a.Atom.initialize(k, x)
	a.x, a.index, a.polynomial = x, index, p
	return atomAssert("nlsat_types.cpp", 55, "is_root_atom()", a.IsRoot())
}
func (a *RootAtom) Variable() uint32                  { return a.x }
func (a *RootAtom) Index() uint32                     { return a.index }
func (a *RootAtom) Polynomial() polynomial.Polynomial { return a.polynomial }
func (a *RootAtom) Hash(pm *polynomial.MultivariatePolynomialCore) (uint32, error) {
	if pm == nil {
		return 0, arena.ErrConfig
	}
	info, err := pm.Info(a.polynomial)
	if err != nil {
		return 0, err
	}
	return zhash.Mix(a.x, a.index<<2|uint32(a.kind), info.ID), nil
}
func (a *RootAtom) Equal(b *RootAtom) bool {
	return a.kind == b.kind && a.x == b.x && a.index == b.index && a.polynomial == b.polynomial
}

func normalizeSign(s int) int {
	if s < 0 {
		return -1
	}
	if s == 0 {
		return 0
	}
	return 1
}
