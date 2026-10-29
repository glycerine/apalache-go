package smtcore

// Source: src/smt/theory_bv.h and src/smt/theory_bv.cpp bit-vector
// variables, numeral bits, bit2bool atoms, and equality expansion.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

const (
	smtBVOpNumeral  int32 = 0
	smtBVOpBit2Bool int32 = 55
)

type smtBVScope struct {
	bits                        int
	atoms, occurrences, private int
	unsupported                 bool
}

type smtBVStatistics struct {
	conflicts, diseqStatic, diseqDynamic uint64
	bit2core, theoryToCoreEq, eqDynamic  uint64
}

// SMTBitVectorTheory retains the source theory's little-endian Boolean-bit
// representation. Operations are added incrementally; any reached operation
// without a bit definition makes final-check report incompleteness.
type SMTBitVectorTheory struct {
	*SMTTheoryBase
	bits               map[ast.ASTHandle][]ast.ASTHandle
	bitAtoms           map[int32][]smtBVBitOccurrence
	bitAtomTrail       []int32
	bitOccurrenceTrail []smtBVBitOccurrenceTrail
	bitLiterals        [][]SMTLiteral
	privateFind        []SMTTheoryVar
	privateSize        []uint32
	privateNext        []SMTTheoryVar
	privateTrail       []smtBVPrivateTrail
	zeroOneBits        [][]smtBVZeroOneBit
	mergeAux           [2][]SMTTheoryVar
	watchedPosition    []uint32
	propagationQueue   []smtBVBitOccurrence
	bitOwners          map[ast.ASTHandle]ast.ASTRef
	bitTrail           []ast.ASTHandle
	comparisonAtoms    map[int32]SMTLiteral
	fixedVariables     map[smtBVFixedKey]SMTTheoryVar
	temporary          []ast.ASTRef
	scopes             []smtBVScope
	unsupported        bool
	statistics         smtBVStatistics
	trace              *diagnostic.TraceTags
	config             SMTBVState
	bitBlasterParams   bit_blaster.SMTBitBlasterParams
}

func (t *SMTBitVectorTheory) ConfigureBitVector(config SMTBVState) error {
	if t == nil {
		return ast.ErrASTDomain
	}
	t.config = config
	return nil
}

func NewSMTBitVectorTheory(family ast.FamilyID) (*SMTBitVectorTheory, error) {
	base, err := NewSMTTheoryBase(family)
	if err != nil {
		return nil, err
	}
	return &SMTBitVectorTheory{
		SMTTheoryBase:   base,
		bits:            make(map[ast.ASTHandle][]ast.ASTHandle),
		bitAtoms:        make(map[int32][]smtBVBitOccurrence),
		bitOwners:       make(map[ast.ASTHandle]ast.ASTRef),
		comparisonAtoms: make(map[int32]SMTLiteral),
		fixedVariables:  make(map[smtBVFixedKey]SMTTheoryVar),
		config:          SMTBVState{Mode: 1, HiDiv0: true, Reflect: true, BlastMaxSize: 2147483647, EnableInt2BV: true},
	}, nil
}

func (t *SMTBitVectorTheory) operation(expression ast.ASTHandle) (int32, bool, error) {
	state, err := t.State()
	if err != nil {
		return 0, false, err
	}
	kind, err := state.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := state.manager.GetFamilyID(expression)
	if err != nil || family != t.FamilyID() {
		return 0, false, err
	}
	op, err := state.manager.GetDeclKind(expression)
	return op, err == nil, err
}

func (t *SMTBitVectorTheory) symbolicBits(expression ast.ASTHandle, width uint32) ([]ast.ASTHandle, error) {
	state, _ := t.State()
	result := make([]ast.ASTHandle, width)
	for i := range result {
		bit, err := state.manager.MkBitVectorBit2Bool(uint32(i), expression)
		if err != nil {
			return nil, err
		}
		result[i] = bit
	}
	return result, nil
}

func (t *SMTBitVectorTheory) numeralBits(expression ast.ASTHandle, width uint32) ([]ast.ASTHandle, error) {
	state, _ := t.State()
	text, matched, err := state.manager.BitVectorNumeralText(expression)
	if err != nil || !matched {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok || value.Sign() < 0 {
		return nil, ast.ErrASTDomain
	}
	result := make([]ast.ASTHandle, width)
	for i := range result {
		result[i], err = state.manager.MkBoolVal(value.Bit(i) != 0)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (t *SMTBitVectorTheory) releaseTemporary() (result error) {
	state, err := t.State()
	if err != nil {
		return err
	}
	for i := range t.temporary {
		result = errors.Join(result, state.manager.DelRef(&t.temporary[i]))
	}
	t.temporary = t.temporary[:0]
	return result
}

// The concrete source bit-blaster owns each returned output reference until
// it is transferred into the theory's temporary roots.
func (t *SMTBitVectorTheory) nativeBitBlaster() (*bit_blaster.Engine, error) {
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	config, err := bit_blaster.NewConfig(state.manager, t.FamilyID(), t.bitBlasterParams)
	if err != nil {
		return nil, err
	}
	return bit_blaster.NewEngine(config)
}

func (t *SMTBitVectorTheory) adoptBitBlasterRefs(refs []ast.ASTRef) ([]ast.ASTHandle, error) {
	t.temporary = append(t.temporary, refs...)
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	bits := make([]ast.ASTHandle, len(refs))
	for i, ref := range refs {
		bits[i], err = state.manager.RefGet(ref)
		if err != nil {
			return nil, err
		}
	}
	return bits, nil
}

func (t *SMTBitVectorTheory) addBits(left, right []ast.ASTHandle) ([]ast.ASTHandle, error) {
	engine, err := t.nativeBitBlaster()
	if err != nil {
		return nil, err
	}
	refs, err := engine.MkAdder(left, right)
	if err != nil {
		return nil, err
	}
	return t.adoptBitBlasterRefs(refs)
}

func (t *SMTBitVectorTheory) subtractBits(left, right []ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	engine, err := t.nativeBitBlaster()
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	refs, carry, err := engine.MkSubtracter(left, right)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	bits, err := t.adoptBitBlasterRefs(append(refs, carry))
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	return bits[:len(bits)-1], bits[len(bits)-1], nil
}

func (t *SMTBitVectorTheory) parameter(expression ast.ASTHandle, index int) (value uint32, result error) {
	state, _ := t.State()
	declaration, err := state.manager.GetDecl(expression)
	if err != nil {
		return 0, err
	}
	parameters, err := state.manager.GetDeclParametersCopy(declaration)
	if err != nil {
		return 0, err
	}
	defer func() {
		for i := range parameters {
			result = errors.Join(result, state.manager.DelParameter(&parameters[i]))
		}
	}()
	if index < 0 || index >= len(parameters) {
		return 0, ast.ErrASTDomain
	}
	integer, err := state.manager.ParameterInt(parameters[index])
	if err != nil || integer < 0 {
		if err != nil {
			return 0, err
		}
		return 0, ast.ErrASTDomain
	}
	return uint32(integer), nil
}

func (t *SMTBitVectorTheory) publishBridge(variable int32, defined SMTLiteral) error {
	state, _ := t.State()
	literal := NewSMTLiteral(variable, false)
	if err := state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal, defined.Negated()}); err != nil {
		return err
	}
	return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), defined})
}

func (t *SMTBitVectorTheory) bridgeAtom(expression, definition ast.ASTHandle, lazy bool) (handled bool, err error) {
	state, _ := t.State()
	// Source internalize_le keeps its expr_ref definition alive across
	// recursive internalization. A nested BV callback may drain temporary
	// gates, so this lexical root must survive independently of that trail.
	root, err := state.manager.NewRef(definition)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, state.manager.DelRef(&root)) }()
	if err := state.Internalize(definition, true); err != nil {
		return false, fmt.Errorf("internalize bit-vector atom definition: %w", errors.Join(err, t.releaseTemporary()))
	}
	if err := t.releaseTemporary(); err != nil {
		return false, fmt.Errorf("release bit-vector atom definition: %w", err)
	}
	variable, found := state.boolVar[expression]
	if !found {
		variable, err = state.MkBoolVar(expression)
		if err != nil {
			return false, fmt.Errorf("create bit-vector atom variable: %w", err)
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
	// Native le_atom retains the canonical definition literal, including its
	// sign, until atom retirement. Relevance is scoped independently.
	if lazy {
		if _, exists := t.comparisonAtoms[variable]; !exists {
			defined, literalErr := state.Literal(definition)
			if literalErr != nil {
				return false, literalErr
			}
			t.comparisonAtoms[variable] = defined
			t.privateTrail = append(t.privateTrail, smtBVPrivateTrail{kind: smtBVNewComparison, boolean: variable})
		}
		if t.config.LazyLE && state.relevancy != nil && state.relevancy.enabled {
			return true, nil
		}
	}
	defined, err := state.Literal(definition)
	if err != nil {
		return false, err
	}
	if err = t.publishBridge(variable, defined); err != nil {
		return false, err
	}
	return true, nil
}

func (t *SMTBitVectorTheory) bitsFor(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	if bits, found := t.bits[expression]; found {
		return bits, nil
	}
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	width, err := state.manager.BitVectorSize(expression)
	if err != nil {
		return nil, err
	}
	op, own, err := t.operation(expression)
	if err != nil {
		return nil, err
	}
	var bits []ast.ASTHandle
	if own && op == smtBVOpNumeral {
		bits, err = t.numeralBits(expression, width)
	} else if own && op == 57 {
		bits, err = t.symbolicBits(expression, width)
	} else if own {
		var handled bool
		bits, handled, err = t.operationBits(expression, op)
		if err == nil && !handled {
			bits, err = t.symbolicBits(expression, width)
			// The source leaves its five division-by-zero value functions
			// uninterpreted and includes them in the generated model.
			if op < 12 || op > 16 {
				t.unsupported = true
			}
		}
	} else {
		bits, err = t.symbolicBits(expression, width)
	}
	if err != nil {
		return nil, errors.Join(err, t.releaseTemporary())
	}
	if err = t.publishBitOwner(expression, bits); err != nil {
		return nil, err
	}
	return bits, nil
}

// Keep the AST representation used by Go consumers alive independently of
// recursively drained engine outputs. Native literal vectors remain authoritative.
func (t *SMTBitVectorTheory) publishBitOwner(expression ast.ASTHandle, bits []ast.ASTHandle) error {
	state := t.state
	assembled, err := state.manager.MkBitVectorFromBits(bits)
	if err != nil {
		return errors.Join(err, t.releaseTemporary())
	}
	owner, err := state.manager.NewRef(assembled)
	if err != nil {
		return errors.Join(err, t.releaseTemporary())
	}
	if err = t.releaseTemporary(); err != nil {
		return errors.Join(err, state.manager.DelRef(&owner))
	}
	t.bits[expression] = bits
	t.bitOwners[expression] = owner
	t.bitTrail = append(t.bitTrail, expression)
	return nil
}

func (t *SMTBitVectorTheory) bit2BoolIndex(expression ast.ASTHandle) (uint32, error) {
	state, _ := t.State()
	declaration, err := state.manager.GetDecl(expression)
	if err != nil {
		return 0, err
	}
	parameters, err := state.manager.GetDeclParametersCopy(declaration)
	if err != nil {
		return 0, err
	}
	defer func() {
		for i := range parameters {
			_ = state.manager.DelParameter(&parameters[i])
		}
	}()
	if len(parameters) != 1 {
		return 0, ast.ErrASTDomain
	}
	value, err := state.manager.ParameterInt(parameters[0])
	if err != nil || value < 0 {
		if err != nil {
			return 0, err
		}
		return 0, ast.ErrASTDomain
	}
	return uint32(value), nil
}

func (t *SMTBitVectorTheory) InternalizeAtom(expression ast.ASTHandle, gateContext bool) (handled bool, result error) {
	defer func() {
		if result != nil {
			result = errors.Join(result, t.releaseTemporary())
		}
	}()
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	if err = t.traceBVExpression("bv", "internalize_atom", 1052, "internalizing atom: ", expression); err != nil {
		return false, err
	}
	if err = bvAssert(1053, "atom->get_family_id() == get_family_id()", func() (bool, error) {
		family, familyErr := t.state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), familyErr
	}); err != nil {
		return false, err
	}
	if approximate, approximateErr := t.approximateTerm(expression); approximateErr != nil || approximate {
		return false, approximateErr
	}
	if !retainedBVAtomOperation(op) {
		return false, diagnostic.BorrowProcessDebugContext().UnreachableSource("src/smt/theory_bv.cpp", 1067)
	}
	if op != smtBVOpBit2Bool {
		if op == 59 || op == 60 {
			return t.internalizeBooleanGate3(expression, op, gateContext)
		}
		if op != 22 && op != 23 && op != 52 && op != 53 && op != 54 {
			t.unsupported = true
			return false, nil
		}
		call := smtBVExpressionCall{theory: t}
		defer func() { result = errors.Join(result, call.close()) }()
		arguments, argErr := call.argumentBits(expression, false)
		if argErr != nil {
			return false, argErr
		}
		definition, argErr := t.nativeAtomDefinition(op, arguments)
		if argErr != nil {
			return false, argErr
		}
		return t.bridgeAtom(expression, definition, true)
	}
	return t.internalizeBitProjection(expression)
}

// Source approximate_term checks both argument and result widths.
func (t *SMTBitVectorTheory) approximateTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	for i := uint32(0); i <= count; i++ {
		candidate := expression
		if i < count {
			candidate, err = state.manager.GetArg(expression, i)
			if err != nil {
				return false, err
			}
		}
		isBV, candidateErr := state.manager.IsBitVector(candidate)
		if candidateErr != nil {
			return false, candidateErr
		}
		if isBV {
			width, widthErr := state.manager.BitVectorSize(candidate)
			if widthErr != nil {
				return false, widthErr
			}
			if width > t.config.BlastMaxSize {
				if !t.unsupported {
					if err = t.traceBVExpression("bv", "approximate_term", 1087, "found large size bit-vector:\n", expression); err != nil {
						return false, err
					}
				}
				t.unsupported = true
				return true, nil
			}
		}
	}
	return false, nil
}

func (t *SMTBitVectorTheory) InternalizeTerm(expression ast.ASTHandle) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	op, own, err := t.operation(expression)
	if err != nil || !own {
		return false, err
	}
	if err = bvAssert(858, "term->get_family_id() == get_family_id()", func() (bool, error) {
		family, familyErr := state.manager.GetFamilyID(expression)
		return family == t.FamilyID(), familyErr
	}); err != nil {
		return false, err
	}
	if err = t.traceBVExpression("bv", "internalize_term", 859, "internalizing term: ", expression); err != nil {
		return false, err
	}
	if approximate, err := t.approximateTerm(expression); err != nil || approximate {
		return false, err
	}
	if (op == 57 || op == 58) && !t.config.EnableInt2BV {
		return false, nil
	}
	// The five div0 declarations use the core uninterpreted-function path,
	// including congruence closure. Their BV sort constraint creates bits.
	if op >= 12 && op <= 16 {
		return false, nil
	}
	if !retainedBVTermOperation(op) {
		if err = t.traceBVExpression("bv_op", "internalize_term", 911, "unsupported operator: ", expression); err != nil {
			return false, err
		}
		return false, diagnostic.BorrowProcessDebugContext().UnreachableSource("src/smt/theory_bv.cpp", 912)
	}
	if op == 58 {
		if err = t.traceBVExpression("bv", "internalize_bv2int", 557, "", expression); err != nil {
			return false, err
		}
	}
	count, err := state.manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	if err = t.assertTermOperatorShape(expression, op, count); err != nil {
		return false, err
	}
	switch op {
	case smtBVOpNumeral:
		if err = bvAssert(518, "!get_context().e_internalized(n)", func() (bool, error) { return state.context.enodes[expression] == nil, nil }); err != nil {
			return false, err
		}
	case 58:
		if err = bvAssert(555, "!get_context().e_internalized(n)", func() (bool, error) { return state.context.enodes[expression] == nil, nil }); err != nil {
			return false, err
		}
	case 57:
		if err = bvAssert(623, "!get_context().e_internalized(n)", func() (bool, error) { return state.context.enodes[expression] == nil, nil }); err != nil {
			return false, err
		}
		if err = bvAssert(624, "n->get_num_args() == 1", func() (bool, error) { return count == 1, nil }); err != nil {
			return false, err
		}
	}
	for i := uint32(0); i < count; i++ {
		argument, argErr := state.manager.GetArg(expression, i)
		if argErr != nil {
			return false, argErr
		}
		// Source process_args internalizes every argument in term context.
		// MKBV's Boolean arguments need enodes too when BV reflection is on.
		if argErr = state.Internalize(argument, false); argErr != nil {
			return false, argErr
		}
	}
	node := state.context.enodes[expression]
	if node == nil {
		node, err = state.MkEnode(expression, !t.config.Reflect, false, t.config.CC)
		if err != nil {
			return false, err
		}
	}
	if !t.IsAttachedToVariable(node) {
		if _, err = t.CreateVariable(node); err != nil {
			return false, err
		}
	}
	if err = bvAssert(156, "e->get_th_var(get_id()) != null_theory_var", func() (bool, error) {
		return node.TheoryVar(t.FamilyID()) != SMTNullTheoryVar, nil
	}); err != nil {
		return false, err
	}
	if op == 58 {
		if state.relevancy == nil || !state.relevancy.enabled {
			if err = t.assertBVToIntAxiom(expression); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	bits, err := t.bitsFor(expression)
	if err != nil {
		return false, err
	}
	width, err := state.manager.BitVectorSize(expression)
	if err != nil {
		return false, err
	}
	if op == smtBVOpNumeral {
		if err = bvAssert(529, "bits.size() == sz", func() (bool, error) { return uint64(len(bits)) == uint64(width), nil }); err != nil {
			return false, err
		}
	} else if op != 37 && op != 40 && op != 57 {
		if err = bvAssert(311, "v != null_theory_var", func() (bool, error) {
			return node.TheoryVar(t.FamilyID()) != SMTNullTheoryVar, nil
		}); err != nil {
			return false, err
		}
		if err = bvAssert(313, "get_bv_size(n) == sz", func() (bool, error) { return uint64(len(bits)) == uint64(width), nil }); err != nil {
			return false, err
		}
	}
	for index, bit := range bits {
		if err = state.Internalize(bit, true); err != nil {
			return false, fmt.Errorf("bit-vector bit %d: %w", index, err)
		}
		literal, literalErr := state.Literal(bit)
		if literalErr != nil {
			return false, literalErr
		}
		if op == smtBVOpNumeral && literal != SMTTrueLiteral {
			if err = bvAssert(537, "m.is_false(l)", func() (bool, error) { return literal == SMTFalseLiteral, nil }); err != nil {
				return false, err
			}
		}
		// Native concat/extract splice existing literals directly, numerals use
		// internalize_num, and int2bv uses mk_bits. Other retained term paths
		// reach init_bits and its per-bit source trace.
		if op != smtBVOpNumeral && op != 37 && op != 40 && op != 57 {
			if err = t.traceBVInitBit(expression, uint32(index), literal); err != nil {
				return false, err
			}
		}
		if err = t.registerBitLiteral(expression, uint32(index), literal); err != nil {
			return false, err
		}
	}
	if op == 57 {
		if state.relevancy == nil || !state.relevancy.enabled {
			if err = t.assertIntToBVAxiom(expression); err != nil {
				return false, err
			}
		}
		// Native mk_bits does not run init_bits/find_wpos.
		return true, nil
	}
	// Source init_bits calls find_wpos immediately. All bits may already be
	// fixed when a new compound term reuses an existing circuit.
	if err = t.findWatchedPosition(state.context.enodes[expression].TheoryVar(t.FamilyID())); err != nil {
		return false, err
	}
	return true, nil
}

// Source theory_bv inherits the base no-op internalize_eq_eh.
// Native class merging and bit propagation handle asserted equalities.
func (t *SMTBitVectorTheory) InternalizeEquality(_ ast.ASTHandle, _ int32) error {
	_, err := t.State()
	return err
}

func (t *SMTBitVectorTheory) ApplySortConstraint(node *SMTEnode, sort ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	family, err := state.manager.GetFamilyID(sort)
	if err != nil || family != t.FamilyID() || t.IsAttachedToVariable(node) {
		return err
	}
	approximate, err := t.approximateTerm(node.owner)
	if err != nil || approximate {
		return err
	}
	if _, err = t.CreateVariable(node); err != nil {
		return err
	}
	// Source apply_sort_cnstr calls mk_bits immediately after mk_var.
	// In particular UF arguments need actual Boolean variables before
	// relevancy/assignment can trigger fixed-value equality propagation.
	bits, err := t.bitsFor(node.owner)
	if err != nil {
		return err
	}
	for index, bit := range bits {
		if err = state.Internalize(bit, true); err != nil {
			return err
		}
		literal, readErr := state.Literal(bit)
		if readErr != nil {
			return readErr
		}
		if err = t.registerBitLiteral(node.owner, uint32(index), literal); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTBitVectorTheory) Relevant(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil || state.relevancy == nil {
		return err
	}
	if err = t.traceBVExpression("bv", "relevant_eh", 1285, "relevant: ", expression); err != nil {
		return err
	}
	if t.config.EnableInt2BV {
		op, own, opErr := t.operation(expression)
		if opErr != nil {
			return opErr
		}
		if own && (op == 57 || op == 58) {
			argument, argErr := state.manager.GetArg(expression, 0)
			if argErr != nil {
				return argErr
			}
			if argErr = state.relevancy.MarkRelevant(argument); argErr != nil {
				return argErr
			}
			if op == 58 {
				return t.assertBVToIntAxiom(expression)
			}
			return t.assertIntToBVAxiom(expression)
		}
	}
	isBool, err := state.isBoolean(expression)
	if err != nil {
		return err
	}
	if isBool {
		if variable, exists := state.boolVar[expression]; exists {
			if defined, found := t.comparisonAtoms[variable]; found {
				if err = t.markLiteralRelevant(defined); err != nil {
					return err
				}
				if t.config.LazyLE {
					return t.publishBridge(variable, defined)
				}
			}
		}
		return nil
	}
	node := state.context.enodes[expression]
	if node == nil {
		return nil
	}
	variable := node.TheoryVar(t.FamilyID())
	if variable == SMTNullTheoryVar {
		return nil
	}
	if variable < 0 || int(variable) >= len(t.bitLiterals) {
		return ast.ErrASTDomain
	}
	for _, bit := range t.bitLiterals[variable] {
		if err = t.markLiteralRelevant(bit); err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTBitVectorTheory) NewEquality(left, right SMTTheoryVar) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.traceBVNewEquality(left, right); err != nil {
		return err
	}
	return t.mergePrivateClasses(left, right)
}

func (t *SMTBitVectorTheory) NewDisequality(left, right SMTTheoryVar) error {
	leftNode, err := t.Enode(left)
	if err != nil {
		return err
	}
	rightNode, err := t.Enode(right)
	if err != nil {
		return err
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	isBV, err := state.manager.IsBitVector(leftNode.owner)
	if err != nil || !isBV {
		return err
	}
	leftBits, rightBits := t.bitLiterals[left], t.bitLiterals[right]
	if err = bvAssert(1115, "m_bits[v1].size() == m_bits[v2].size()", func() (bool, error) { return len(leftBits) == len(rightBits), nil }); err != nil {
		return err
	}
	if len(leftBits) == 0 || len(leftBits) != len(rightBits) {
		return ast.ErrASTDomain
	}
	leftWidth, err := state.manager.BitVectorSize(leftNode.owner)
	if err != nil {
		return err
	}
	rightWidth, err := state.manager.BitVectorSize(rightNode.owner)
	if err != nil {
		return err
	}
	if err = bvAssert(1121, "get_bv_size(v1) == get_bv_size(v2)", func() (bool, error) { return leftWidth == rightWidth, nil }); err != nil {
		return err
	}
	if leftWidth != rightWidth {
		return ast.ErrASTDomain
	}
	equality, err := t.MkEqualityLiteral(leftNode.owner, rightNode.owner, true)
	if err != nil {
		return err
	}
	for i := range leftBits {
		if leftBits[i] == rightBits[i].Negated() {
			return nil
		}
	}
	clause := []SMTLiteral{equality}
	for i := range leftBits {
		literal, err := t.internalizeBitDifference(leftBits[i], rightBits[i])
		if err != nil {
			return errors.Join(err, t.releaseTemporary())
		}
		clause = append(clause, literal)
	}
	if err = t.releaseTemporary(); err != nil {
		return err
	}
	t.statistics.diseqDynamic++
	return state.addTheoryAxiom(t.FamilyID(), clause)
}

func (t *SMTBitVectorTheory) Assign(variable int32, isTrue bool) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if err := t.traceBVAssignment(variable, isTrue); err != nil {
		return err
	}
	entries, exists := t.bitAtoms[variable]
	if !exists {
		return nil
	}
	t.propagationQueue = t.propagationQueue[:0]
	for i := len(entries) - 1; i >= 0; i-- {
		t.propagationQueue = append(t.propagationQueue, entries[i])
	}
	if err := t.traceBVPropagationQueueSize(len(t.propagationQueue)); err != nil {
		return err
	}
	return t.propagateBits()
}

func (t *SMTBitVectorTheory) PushScope() error {
	if err := t.SMTTheoryBase.PushScope(); err != nil {
		return err
	}
	t.scopes = append(t.scopes, smtBVScope{
		bits:  len(t.bitTrail),
		atoms: len(t.bitAtomTrail), occurrences: len(t.bitOccurrenceTrail), private: len(t.privateTrail),
		unsupported: t.unsupported,
	})
	return nil
}

func (t *SMTBitVectorTheory) PopScopes(count uint32) error {
	if count == 0 || count > uint32(len(t.scopes)) {
		return ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	if err = t.traceBVScopePop(count); err != nil {
		return err
	}
	level := len(t.scopes) - int(count)
	scope := t.scopes[level]
	if err = t.restorePrivateTrail(scope); err != nil {
		return err
	}
	for _, expression := range t.bitTrail[scope.bits:] {
		delete(t.bits, expression)
		owner := t.bitOwners[expression]
		if err := state.manager.DelRef(&owner); err != nil {
			return err
		}
		delete(t.bitOwners, expression)
	}
	t.bitTrail = t.bitTrail[:scope.bits]

	t.unsupported = scope.unsupported
	t.scopes = t.scopes[:level]
	if err = t.SMTTheoryBase.PopScopes(count); err != nil {
		return err
	}
	t.bitLiterals = t.bitLiterals[:len(t.variables)]
	t.watchedPosition = t.watchedPosition[:len(t.variables)]
	t.zeroOneBits = t.zeroOneBits[:len(t.variables)]
	return nil
}

func (t *SMTBitVectorTheory) FinalCheck() (SMTFinalCheckStatus, error) {
	if err := t.checkBVInvariant(); err != nil {
		return SMTFinalCheckDone, err
	}
	if t.unsupported {
		return SMTFinalCheckGiveUp, nil
	}
	return SMTFinalCheckDone, nil
}

func (*SMTBitVectorTheory) Name() string { return "bit-vector" }

func (t *SMTBitVectorTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint64("bv conflicts", t.statistics.conflicts)
	target.UpdateUint64("bv diseqs", t.statistics.diseqStatic)
	target.UpdateUint64("bv dynamic diseqs", t.statistics.diseqDynamic)
	target.UpdateUint64("bv bit2core", t.statistics.bit2core)
	target.UpdateUint64("bv->core eq", t.statistics.theoryToCoreEq)
	target.UpdateUint64("bv dynamic eqs", t.statistics.eqDynamic)
	return nil
}

func (t *SMTBitVectorTheory) ResetStatistics() {
	if t != nil {
		t.statistics = smtBVStatistics{}
	}
}

func (t *SMTBitVectorTheory) Close() (result error) {
	if t == nil {
		return ast.ErrASTHandle
	}
	state, err := t.State()
	if err != nil {
		return err
	}
	result = errors.Join(result, t.releaseTemporary())
	for expression, owner := range t.bitOwners {
		result = errors.Join(result, state.manager.DelRef(&owner))
		delete(t.bitOwners, expression)
		delete(t.bits, expression)
	}
	t.bitAtoms = nil
	t.bitAtomTrail = nil
	t.bitOccurrenceTrail = nil
	t.bitLiterals = nil
	t.privateFind, t.privateNext, t.privateSize = nil, nil, nil
	t.privateTrail, t.zeroOneBits, t.watchedPosition, t.propagationQueue = nil, nil, nil, nil
	t.mergeAux = [2][]SMTTheoryVar{}
	t.fixedVariables = nil
	t.comparisonAtoms = nil
	t.bitTrail = t.bitTrail[:0]
	t.scopes = t.scopes[:0]
	return result
}

func (t *SMTBitVectorTheory) InitializeModel(generator *SMTModelGenerator) error {
	state, err := t.State()
	if err != nil || generator == nil || generator.manager != state.manager || generator.model == nil {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	factory, err := protomodel.NewBitVectorFactory(generator.manager)
	if err != nil {
		return err
	}
	return generator.RegisterFactory(factory)
}

// IncludeFunctionInterpretation retains the source model hook for the five
// unspecified division-by-zero functions.
func (t *SMTBitVectorTheory) IncludeFunctionInterpretation(declaration ast.ASTHandle) bool {
	state, err := t.State()
	if err != nil {
		return false
	}
	family, err := state.manager.GetFamilyID(declaration)
	if err != nil || family != t.FamilyID() {
		return false
	}
	op, err := state.manager.GetDeclKind(declaration)
	return err == nil && op >= 12 && op <= 16
}

func (t *SMTBitVectorTheory) ModelValue(node *SMTEnode, generator *SMTModelGenerator) (SMTModelValueProcedure, error) {
	if node == nil || generator == nil || generator.model == nil {
		return nil, ast.ErrASTDomain
	}
	state, err := t.State()
	if err != nil {
		return nil, err
	}
	// Source mk_value always evaluates get_fixed_value, including when the
	// runtime assertion switch is off. C actions resume with that native prefix.
	variable := node.TheoryVar(t.FamilyID())
	if err = bvAssert(1596, "v != null_theory_var", func() (bool, error) { return variable != SMTNullTheoryVar, nil }); err != nil {
		return nil, err
	}
	// Invalid foreign/stale variable slots have no defined source access.
	if !t.IsAttachedToVariable(node) {
		return nil, ast.ErrASTDomain
	}
	value, fixed, err := t.fixedLiteralValue(variable)
	if err != nil {
		return nil, err
	}
	if err = bvAssert(1601, "r", func() (bool, error) { return fixed, nil }); err != nil {
		return nil, err
	}
	width, err := state.manager.BitVectorSize(node.owner)
	if err != nil {
		return nil, err
	}
	factory, ok := generator.model.Factory(t.FamilyID()).(*protomodel.BitVectorFactory)
	if !ok || factory == nil {
		return nil, ast.ErrASTDomain
	}
	numeral, err := factory.MkNumValue(new(big.Rat).SetInt(value), width)
	if err != nil {
		return nil, err
	}
	return NewSMTExpressionValueProcedure(numeral)
}

var _ SMTTheoryInternalizer = (*SMTBitVectorTheory)(nil)
