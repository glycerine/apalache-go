package smtcore

// Source: src/smt/smt_model_generator.cpp dependency ordering and fresh-value
// source management.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	protomodel "github.com/glycerine/g3/pkg/smtcore/proto_model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type SMTModelGenerator struct {
	manager       *ast.ASTManager
	context       *SMTCheckerContext
	model         *protomodel.ProtoModel
	extraFresh    []*SMTExtraFreshValue
	rootValues    map[*SMTEnode]ast.ASTHandle
	searchFailure SMTFailure // Active kernel's failure when building this candidate.
	trace         *diagnostic.TraceTags
	refs          []ast.ASTRef
	freshIndex    uint32
	closed        bool
}

func NewSMTModelGenerator(manager *ast.ASTManager) (*SMTModelGenerator, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTModelGenerator{manager: manager, rootValues: make(map[*SMTEnode]ast.ASTHandle), freshIndex: 1}, nil
}

func (g *SMTModelGenerator) open() error {
	if g == nil || g.manager == nil || g.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (g *SMTModelGenerator) SetContext(context *SMTCheckerContext) error {
	if err := g.open(); err != nil {
		return err
	}
	if context == nil || context.manager != g.manager || g.context != nil {
		return ast.ErrASTDomain
	}
	g.context = context
	return nil
}

func (g *SMTModelGenerator) InitializeModel(config model.ModelEvaluatorConfig) error {
	if err := g.open(); err != nil {
		return err
	}
	if g.context == nil {
		return ast.ErrASTDomain
	}
	if err := g.sourceAssertion(524, g.model == nil, "!m_model"); err != nil {
		return err
	}
	if err := g.sourceAssertion(51, g.model == nil, "!m_model"); err != nil {
		return err
	}
	if g.model != nil {
		return ast.ErrASTDomain
	}
	value, err := protomodel.NewProtoModel(g.manager, config)
	if err != nil {
		return err
	}
	g.model = value
	g.trace = config.Trace
	return nil
}

func (g *SMTModelGenerator) RegisterFactory(factory protomodel.ValueFactory) error {
	if err := g.open(); err != nil {
		return err
	}
	if g.model == nil {
		return ast.ErrASTDomain
	}
	return g.model.RegisterFactory(factory)
}

func (g *SMTModelGenerator) SomeValue(sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := g.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := g.sourceAssertion(476, g.model != nil, "m_model"); err != nil {
		return ast.ASTHandle{}, err
	}
	if g.model == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return g.model.SomeValue(sort)
}

func (g *SMTModelGenerator) pin(value ast.ASTHandle) error {
	ref, err := g.manager.NewRef(value)
	if err != nil {
		return err
	}
	g.refs = append(g.refs, ref)
	return nil
}

func (g *SMTModelGenerator) MkExtraFreshValue(sort ast.ASTHandle) (*SMTExtraFreshValue, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	infinite, err := g.manager.SortIsInfinite(sort)
	if err != nil {
		return nil, err
	}
	if err = g.sourceAssertion(468, infinite, "s->is_infinite()"); err != nil {
		return nil, err
	}
	if !infinite {
		return nil, ast.ErrASTDomain
	}
	value, err := NewSMTExtraFreshValue(sort, g.freshIndex)
	if err != nil {
		return nil, err
	}
	g.freshIndex++
	g.extraFresh = append(g.extraFresh, value)
	return value, nil
}

func (g *SMTModelGenerator) BuildValues(roots []*SMTEnode, procedures map[*SMTEnode]SMTModelValueProcedure) error {
	if err := g.open(); err != nil {
		return err
	}
	if g.model == nil || len(g.rootValues) != 0 {
		return ast.ErrASTDomain
	}
	ordered, err := g.topSortSources(roots, procedures)
	if err != nil {
		return err
	}
	if err = g.traceSortedSources(ordered, procedures); err != nil {
		return err
	}
	for _, source := range ordered {
		if source.IsFreshValue() {
			fresh, e := source.FreshValue()
			if e != nil {
				return e
			}
			value, e := g.model.FreshValue(fresh.sort)
			if traceErr := g.traceFreshValue(fresh, value); traceErr != nil {
				return traceErr
			}
			if e != nil || value.IsNull() {
				if e != nil {
					return e
				}
				return ast.ErrASTDomain
			}
			if e = g.pin(value); e != nil {
				return e
			}
			if e = fresh.SetValue(value); e != nil {
				return e
			}
			continue
		}
		node, e := source.Enode()
		if e != nil {
			return e
		}
		if e = g.sourceAssertion(344, node.root == node, "n->get_root() == n"); e != nil {
			return e
		}
		procedure := procedures[node]
		if procedure == nil {
			if verifyErr := diagnostic.BorrowProcessDebugContext().VerifyFailure("src/smt/smt_model_generator.cpp", 349, "root2proc.find(n, proc)"); verifyErr != nil {
				return verifyErr
			}
		}
		if e = g.sourceAssertion(350, procedure != nil, "proc"); e != nil {
			return e
		}
		if procedure == nil {
			return ast.ErrASTDomain
		}
		if e = g.traceModel("mg_top_sort", "mk_values", 345, func() bool { return true }, func(out io.Writer) error {
			id, traceErr := node.OwnerID()
			if traceErr == nil {
				_, traceErr = fmt.Fprintf(out, "#%d\n", id)
			}
			return traceErr
		}); e != nil {
			return e
		}
		dependencies := procedure.Dependencies()
		values := make([]ast.ASTHandle, 0, len(dependencies))
		for _, dependency := range dependencies {
			if dependency.IsFreshValue() {
				fresh, depErr := dependency.FreshValue()
				if depErr != nil {
					return depErr
				}
				if depErr = g.traceModel("mg_top_sort", "mk_values", 357, func() bool { return fresh.value.IsNull() }, func(out io.Writer) error {
					id, traceErr := node.OwnerID()
					if traceErr == nil {
						_, traceErr = fmt.Fprintf(out, "#%d -> fresh!%d\n", id, fresh.index)
					}
					return traceErr
				}); depErr != nil {
					return depErr
				}
				value, depErr := fresh.Value()
				if assertErr := g.sourceAssertion(360, !value.IsNull(), "d.get_value()->get_value()"); assertErr != nil {
					return assertErr
				}
				if depErr != nil {
					return depErr
				}
				values = append(values, value)
			} else {
				child, depErr := dependency.Enode()
				if depErr != nil {
					return depErr
				}
				if depErr = g.traceModel("mg_top_sort", "mk_values", 365, func() bool { return true }, func(out io.Writer) error {
					id, traceErr := node.OwnerID()
					if traceErr != nil {
						return traceErr
					}
					if _, traceErr = fmt.Fprintf(out, "#%d (", id); traceErr != nil {
						return traceErr
					}
					if traceErr = g.displayModelExpression(out, node.owner); traceErr != nil {
						return traceErr
					}
					if _, traceErr = io.WriteString(out, "): "); traceErr != nil {
						return traceErr
					}
					if traceErr = g.displayModelExpression(out, child.owner); traceErr != nil {
						return traceErr
					}
					if _, traceErr = io.WriteString(out, " "); traceErr != nil {
						return traceErr
					}
					if traceErr = g.displayModelExpression(out, child.root.owner); traceErr != nil {
						return traceErr
					}
					_, traceErr = io.WriteString(out, "\n")
					return traceErr
				}); depErr != nil {
					return depErr
				}
				child = child.root
				value := g.rootValues[child]
				if assertErr := g.sourceAssertion(369, !value.IsNull(), "val"); assertErr != nil {
					return assertErr
				}
				if value.IsNull() {
					return ast.ErrASTDomain
				}
				values = append(values, value)
			}
		}
		value, e := procedure.MakeValue(g, values)
		if e != nil || value.IsNull() {
			if e != nil {
				return e
			}
			return ast.ErrASTDomain
		}
		if e = g.sourceAssertion(481, g.model != nil, "m_model"); e != nil {
			return e
		}
		if e = g.model.RegisterValue(value); e != nil {
			return e
		}
		if e = g.pin(value); e != nil {
			return e
		}
		g.rootValues[node] = value
	}
	return nil
}

func (g *SMTModelGenerator) discoverValueProcedures(state *SMTInternalizerState) ([]*SMTEnode, map[*SMTEnode]SMTModelValueProcedure, error) {
	if err := g.open(); err != nil {
		return nil, nil, err
	}
	if state == nil || state.manager != g.manager || state.context != g.context || g.model == nil {
		return nil, nil, ast.ErrASTDomain
	}
	for _, theory := range state.theoryOrder {
		if modelTheory, ok := theory.(SMTModelInitializer); ok {
			if err := g.traceModel("model", "init_model", 57, func() bool { return true }, func(out io.Writer) error {
				name := fmt.Sprintf("%T", theory)
				if named, found := theory.(interface{ Name() string }); found {
					name = named.Name()
				}
				_, traceErr := fmt.Fprintf(out, "init_model for theory: %s\n", name)
				return traceErr
			}); err != nil {
				return nil, nil, err
			}
			if err := modelTheory.InitializeModel(g); err != nil {
				return nil, nil, err
			}
		}
	}
	// Reserve existing values after theory factories initialize and before
	// any fallback procedure requests a fresh value (source mk_model order).
	if err := g.registerExistingModelValues(state); err != nil {
		return nil, nil, err
	}
	if err := g.publishBooleanInterpretations(state); err != nil {
		return nil, nil, err
	}
	booleanSort, err := g.manager.MkBoolSort()
	if err != nil {
		return nil, nil, err
	}
	roots := make([]*SMTEnode, 0, len(state.enodeStack))
	procedures := make(map[*SMTEnode]SMTModelValueProcedure)
	for _, node := range state.enodeStack {
		if node == nil || node.root != node || !state.isRelevant(node.owner) {
			continue
		}
		roots = append(roots, node)
		sort, sortErr := g.manager.GetSort(node.owner)
		if sortErr != nil {
			return nil, nil, sortErr
		}
		var procedure SMTModelValueProcedure
		if sort == booleanSort {
			if err := g.sourceAssertion(70, sort == booleanSort, "m_manager.is_bool(p)"); err != nil {
				return nil, nil, err
			}
			value, valueErr := g.manager.MkFalse()
			if g.manager.IsTrue(node.owner) {
				value, valueErr = g.manager.MkTrue()
			} else if !g.manager.IsFalse(node.owner) {
				assignment := g.context.assignments[node.owner]
				if err := g.traceModel("model", "mk_value_procs", 94, func() bool { return assignment == lbool.Undef }, func(out io.Writer) error {
					if traceErr := g.displayModelExpression(out, node.owner); traceErr != nil {
						return traceErr
					}
					_, traceErr := io.WriteString(out, "\n")
					return traceErr
				}); err != nil {
					return nil, nil, err
				}
				if err := g.sourceAssertion(96, assignment != lbool.Undef, "m_context->get_assignment(r) != l_undef"); err != nil {
					return nil, nil, err
				}
				if assignment == lbool.True {
					value, valueErr = g.manager.MkTrue()
				}
			}
			if valueErr != nil {
				return nil, nil, valueErr
			}
			procedure, valueErr = NewSMTExpressionValueProcedure(value)
			if valueErr != nil {
				return nil, nil, valueErr
			}
		} else {
			family, familyErr := g.manager.GetFamilyID(sort)
			if familyErr != nil {
				return nil, nil, familyErr
			}
			theory := state.theories[family]
			if modelTheory, ok := theory.(SMTModelValueTheory); ok {
				if node.TheoryVar(family) != SMTNullTheoryVar {
					procedure, familyErr = modelTheory.ModelValue(node, g)
					if familyErr == nil {
						familyErr = g.sourceAssertion(108, procedure != nil, "proc")
					}
				} else {
					if familyErr = g.traceModel("model", "mk_value_procs", 111, func() bool { return true }, func(out io.Writer) error {
						id, traceErr := node.OwnerID()
						if traceErr == nil {
							_, traceErr = fmt.Fprintf(out, "creating fresh value for #%d\n", id)
						}
						return traceErr
					}); familyErr != nil {
						return nil, nil, familyErr
					}
					fresh, freshErr := g.MkExtraFreshValue(sort)
					if freshErr != nil {
						return nil, nil, freshErr
					}
					procedure, familyErr = NewSMTFreshValueProcedure(fresh)
				}
				if familyErr != nil {
					return nil, nil, familyErr
				}
			} else {
				if familyErr = g.sourceAssertion(128, node.root == node, "r == r->get_root()"); familyErr != nil {
					return nil, nil, familyErr
				}
				modelValue, valueErr := g.manager.IsModelValue(node.owner)
				if valueErr != nil {
					return nil, nil, valueErr
				}
				value := node.owner
				if !modelValue {
					value, valueErr = g.model.FreshValue(sort)
					if valueErr = g.traceModel("model", "mk_model_value", 133, func() bool { return value.IsNull() }, func(out io.Writer) error {
						if traceErr := g.displayModelExpression(out, node.owner); traceErr != nil {
							return traceErr
						}
						if _, traceErr := io.WriteString(out, "\nsort:\n"); traceErr != nil {
							return traceErr
						}
						if traceErr := g.displayModelExpression(out, sort); traceErr != nil {
							return traceErr
						}
						finite, traceErr := g.model.IsFinite(sort)
						if traceErr == nil {
							_, traceErr = fmt.Fprintf(out, "\nis_finite: %t\n", finite)
						}
						return traceErr
					}); valueErr != nil {
						return nil, nil, valueErr
					}
					if valueErr != nil || value.IsNull() {
						if valueErr != nil {
							return nil, nil, valueErr
						}
						return nil, nil, ast.ErrASTDomain
					}
				}
				procedure, valueErr = NewSMTExpressionValueProcedure(value)
				if valueErr == nil {
					valueErr = g.sourceAssertion(117, procedure != nil, "proc")
				}
				if valueErr != nil {
					return nil, nil, valueErr
				}
			}
			if familyErr = g.sourceAssertion(120, procedure != nil, "proc"); familyErr != nil {
				return nil, nil, familyErr
			}
			if procedure == nil {
				return nil, nil, ast.ErrASTDomain
			}
		}
		procedures[node] = procedure
	}
	return roots, procedures, nil
}

func (g *SMTModelGenerator) BuildStateValues(state *SMTInternalizerState) error {
	if err := g.traceModel("model", "mk_model", 525, func() bool { return true }, func(out io.Writer) error {
		if state == nil {
			return ast.ErrASTDomain
		}
		if err := state.DisplayAssignment(out); err != nil {
			return err
		}
		if err := state.DisplayEquivalenceClasses(out); err != nil {
			return err
		}
		if err := state.DisplayExpressionBoolVarMap(out); err != nil {
			return err
		}
		if err := state.DisplayApplicationEnodeMap(out); err != nil {
			return err
		}
		return state.DisplayTheories(out)
	}); err != nil {
		return err
	}
	roots, procedures, err := g.discoverValueProcedures(state)
	if err != nil {
		return err
	}
	if err = g.BuildValues(roots, procedures); err != nil {
		return err
	}
	if err = g.publishConstantInterpretations(state); err != nil {
		return err
	}
	if err = g.publishFunctionInterpretations(state); err != nil {
		return err
	}
	return g.finalizeTheoryModels(state)
}

func (g *SMTModelGenerator) registerExistingModelValues(state *SMTInternalizerState) error {
	if g.model == nil {
		return ast.ErrASTDomain
	}
	for _, node := range state.enodeStack {
		if node == nil || node.root != node || !state.isRelevant(node.owner) {
			continue
		}
		modelValue, err := g.manager.IsModelValue(node.owner)
		if err != nil {
			return err
		}
		if modelValue {
			if err = g.sourceAssertion(481, g.model != nil, "m_model"); err != nil {
				return err
			}
			if err = g.model.RegisterValue(node.owner); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *SMTModelGenerator) uninterpretedConstant(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	kind, err := g.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	count, err := g.manager.GetNumArgs(expression)
	if err != nil || count != 0 {
		return ast.ASTHandle{}, false, err
	}
	family, err := g.manager.GetFamilyID(expression)
	if err != nil || family != ast.NullFamilyID {
		return ast.ASTHandle{}, false, err
	}
	declaration, err := g.manager.GetDecl(expression)
	return declaration, err == nil, err
}

func (g *SMTModelGenerator) publishBooleanInterpretations(state *SMTInternalizerState) error {
	if g.model == nil {
		return ast.ErrASTDomain
	}
	for variable := int32(SMTFirstBoolVar); int(variable) < len(state.boolExpr); variable++ {
		expression := state.boolExpr[variable]
		if expression.IsNull() || !state.isRelevant(expression) {
			continue
		}
		declaration, constant, err := g.uninterpretedConstant(expression)
		if err != nil {
			return err
		}
		if !constant {
			continue
		}
		assignment := state.assignment[NewSMTLiteral(variable, false).Index()]
		// mk_bool_model chooses false for every assignment other than
		// l_true, including an unassigned atom in an incomplete candidate.
		value, err := g.manager.MkFalse()
		if assignment == lbool.True {
			value, err = g.manager.MkTrue()
		}
		if err != nil {
			return err
		}
		if err = g.model.Core().RegisterConstant(declaration, value); err != nil {
			return err
		}
	}
	return nil
}

func (g *SMTModelGenerator) publishConstantInterpretations(state *SMTInternalizerState) error {
	if g.model == nil {
		return ast.ErrASTDomain
	}
	for _, node := range state.enodeStack {
		if node == nil || !state.isRelevant(node.owner) {
			continue
		}
		value := g.rootValues[node.root]
		if value.IsNull() {
			continue
		}
		declaration, constant, err := g.uninterpretedConstant(node.owner)
		if err != nil {
			return err
		}
		if constant {
			if err = g.model.Core().RegisterConstant(declaration, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *SMTModelGenerator) includeFunctionInterpretation(state *SMTInternalizerState, declaration ast.ASTHandle) (bool, error) {
	family, err := g.manager.GetFamilyID(declaration)
	if err != nil {
		return false, err
	}
	if err = g.traceModel("model", "include_func_interp", 409, func() bool { return true }, func(out io.Writer) error {
		if traceErr := g.manager.DisplayDeclarationHead(out, declaration); traceErr != nil {
			return traceErr
		}
		_, traceErr := fmt.Fprintf(out, " %d\n", family)
		return traceErr
	}); err != nil {
		return false, err
	}
	if family == ast.NullFamilyID {
		return true, nil
	}
	if family == g.manager.BasicFamilyID() {
		return false, nil
	}
	theory := state.theories[family]
	if err = g.traceModel("model", "include_func_interp", 413, func() bool { return true }, func(out io.Writer) error {
		if theory == nil {
			_, traceErr := io.WriteString(out, "<nil>\n")
			return traceErr
		}
		name := fmt.Sprintf("%T", theory)
		if named, found := theory.(interface{ Name() string }); found {
			name = named.Name()
		}
		_, traceErr := fmt.Fprintf(out, "%s\n", name)
		return traceErr
	}); err != nil {
		return false, err
	}
	if theory == nil {
		return true, nil
	}
	modelTheory, ok := theory.(SMTModelFunctionTheory)
	return ok && modelTheory.IncludeFunctionInterpretation(declaration), nil
}

func (g *SMTModelGenerator) publishFunctionInterpretations(state *SMTInternalizerState) error {
	if g.model == nil {
		return ast.ErrASTDomain
	}
	for _, node := range state.enodeStack {
		if node == nil || node.NumArgs() == 0 || !node.IsCongruenceRoot() || !state.isRelevant(node.owner) {
			continue
		}
		include, err := g.includeFunctionInterpretation(state, node.declaration)
		if err != nil {
			return err
		}
		if !include {
			continue
		}
		result := g.rootValues[node.root]
		if err = g.sourceAssertion(434, !result.IsNull(), "result"); err != nil {
			return err
		}
		if result.IsNull() {
			return ast.ErrASTDomain
		}
		arguments := make([]ast.ASTHandle, len(node.arguments))
		for index, argument := range node.arguments {
			if argument == nil || argument.root == nil {
				return ast.ErrASTDomain
			}
			arguments[index] = g.rootValues[argument.root]
			if err = g.sourceAssertion(437, !arguments[index].IsNull(), "arg"); err != nil {
				return err
			}
			if arguments[index].IsNull() {
				return ast.ErrASTDomain
			}
		}
		interpretation, err := g.model.Core().FunctionInterpretation(node.declaration)
		if err != nil {
			return err
		}
		if interpretation == nil {
			interpretation, err = model.NewFunctionInterpretation(g.manager, node.NumArgs())
			if err != nil {
				return err
			}
			if err = g.model.Core().RegisterFunction(node.declaration, interpretation); err != nil {
				_ = interpretation.Close()
				return err
			}
		}
		registered, err := g.model.Core().FunctionInterpretation(node.declaration)
		if err != nil {
			return err
		}
		if err = g.sourceAssertion(445, registered != nil, "m_model->has_interpretation(f)"); err != nil {
			return err
		}
		if err = g.sourceAssertion(446, registered == interpretation, "m_model->get_func_interp(f) == fi"); err != nil {
			return err
		}
		if err = g.traceModel("model", "mk_func_interps", 448, func() bool { return true }, func(out io.Writer) error {
			if _, traceErr := io.WriteString(out, "insert new entry for:\n"); traceErr != nil {
				return traceErr
			}
			if traceErr := g.displayModelExpression(out, node.owner); traceErr != nil {
				return traceErr
			}
			if _, traceErr := io.WriteString(out, "\nargs: "); traceErr != nil {
				return traceErr
			}
			for _, argument := range node.arguments {
				id, traceErr := argument.OwnerID()
				if traceErr != nil {
					return traceErr
				}
				if _, traceErr = fmt.Fprintf(out, "#%d ", id); traceErr != nil {
					return traceErr
				}
			}
			id, traceErr := node.OwnerID()
			if traceErr != nil {
				return traceErr
			}
			if _, traceErr = fmt.Fprintf(out, "\nvalue: #%d\n", id); traceErr != nil {
				return traceErr
			}
			if traceErr = g.displayModelExpression(out, result); traceErr != nil {
				return traceErr
			}
			_, traceErr = io.WriteString(out, "\n")
			return traceErr
		}); err != nil {
			return err
		}
		if g.searchFailure == SMTFailureTheory {
			// An incomplete theory can assign identical argument values to
			// distinct congruence roots. Native keeps the first such entry.
			entry, err := interpretation.FindEntry(arguments)
			if err != nil {
				return err
			}
			if entry != nil {
				continue
			}
		}
		if err = interpretation.InsertNewEntry(arguments, result); err != nil {
			return err
		}
	}
	return nil
}

func (g *SMTModelGenerator) finalizeTheoryModels(state *SMTInternalizerState) error {
	for _, theory := range state.theoryOrder {
		if finalizer, ok := theory.(SMTModelFinalizer); ok {
			if err := finalizer.FinalizeModel(g); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *SMTModelGenerator) FinalModel() (*model.Model, error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if g.model == nil {
		return nil, ast.ErrASTDomain
	}
	result, err := g.model.FinalModel()
	if err != nil {
		return nil, err
	}
	g.model = nil
	return result, nil
}

func (g *SMTModelGenerator) Value(node *SMTEnode) (ast.ASTHandle, error) {
	if err := g.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if node == nil || node.root == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	value := g.rootValues[node.root]
	if err := g.sourceAssertion(400, !value.IsNull(), "val"); err != nil {
		return ast.ASTHandle{}, err
	}
	if value.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return value, nil
}

func (g *SMTModelGenerator) Close() error {
	if g == nil || g.manager == nil {
		return ast.ErrASTHandle
	}
	if g.closed {
		return nil
	}
	var result error
	if g.model != nil {
		result = errors.Join(result, g.model.Close())
		g.model = nil
	}
	for index := len(g.refs); index > 0; index-- {
		result = errors.Join(result, g.manager.DelRef(&g.refs[index-1]))
	}
	g.refs = nil
	if result == nil {
		g.closed = true
		g.context = nil
		g.trace = nil
		g.manager = nil
	}
	return result
}

type smtModelSourceKey struct {
	node  *SMTEnode
	fresh *SMTExtraFreshValue
}

func smtModelDependencyKey(dependency SMTModelValueDependency) (smtModelSourceKey, error) {
	if dependency.IsFreshValue() {
		value, err := dependency.FreshValue()
		return smtModelSourceKey{fresh: value}, err
	}
	node, err := dependency.Enode()
	return smtModelSourceKey{node: node}, err
}

func smtModelKeyDependency(key smtModelSourceKey) (SMTModelValueDependency, error) {
	if key.fresh != nil {
		return NewSMTFreshValueDependency(key.fresh)
	}
	return NewSMTEnodeValueDependency(key.node)
}

func (g *SMTModelGenerator) topSortSources(roots []*SMTEnode, procedures map[*SMTEnode]SMTModelValueProcedure) ([]SMTModelValueDependency, error) {
	if g == nil || g.manager == nil {
		return nil, ast.ErrASTHandle
	}
	const (
		white uint8 = iota
		gray
		black
	)
	colors := make(map[smtModelSourceKey]uint8)
	traversedSorts := make(map[ast.ASTHandle]struct{})
	var todo []smtModelSourceKey
	result := make([]SMTModelValueDependency, 0, len(roots)+len(g.extraFresh))
	// Push all white dependencies in producer order. The work stack consumes
	// the last one first, matching native visit_child/process_source.
	pushChild := func(key smtModelSourceKey) (bool, error) {
		switch colors[key] {
		case white:
			todo = append(todo, key)
			return false, nil
		case gray:
			// A true dependency cycle cannot produce the values required by
			// mk_values. Retain the Go domain guard for that invalid input.
			return false, ast.ErrASTDomain
		default:
			return true, nil
		}
	}
	visitChildren := func(key smtModelSourceKey) (bool, error) {
		visited := true
		if key.fresh != nil {
			sort := key.fresh.sort
			if _, done := traversedSorts[sort]; done {
				return true, nil
			}
			for _, root := range roots {
				if root == nil {
					return false, ast.ErrASTDomain
				}
				rootSort, err := g.manager.GetSort(root.owner)
				if err != nil {
					return false, err
				}
				if rootSort != sort {
					continue
				}
				if err = g.sourceAssertion(179, root.root == root, "r == r->get_root()"); err != nil {
					return false, err
				}
				procedure := procedures[root]
				if err = g.sourceAssertion(182, procedure != nil, "proc"); err != nil {
					return false, err
				}
				if root.root != root || procedure == nil {
					return false, ast.ErrASTDomain
				}
				if procedure.IsFresh() {
					continue
				}
				if err = g.sourceAssertion(185, root.root == root, "r == r->get_root()"); err != nil {
					return false, err
				}
				if err = g.traceModel("mg_top_sort", "visit_children", 186, func() bool { return true }, func(out io.Writer) error {
					rootID, traceErr := root.OwnerID()
					if traceErr != nil {
						return traceErr
					}
					if _, traceErr = fmt.Fprintf(out, "fresh!%d -> #%d ", key.fresh.index, rootID); traceErr != nil {
						return traceErr
					}
					if traceErr = g.displayModelExpression(out, rootSort); traceErr != nil {
						return traceErr
					}
					_, traceErr = io.WriteString(out, "\n")
					return traceErr
				}); err != nil {
					return false, err
				}
				done, err := pushChild(smtModelSourceKey{node: root})
				if err != nil {
					return false, err
				}
				if err = g.traceModel("mg_top_sort", "visit_children", 188, func() bool { return true }, func(out io.Writer) error {
					_, traceErr := fmt.Fprintf(out, "visited: %t, todo.size(): %d\n", done, len(todo))
					return traceErr
				}); err != nil {
					return false, err
				}
				visited = visited && done
			}
			// Native marks the sort before processing any newly pushed roots.
			// A second fresh source of this sort must not rescan those roots.
			traversedSorts[sort] = struct{}{}
			return visited, nil
		}
		if err := g.sourceAssertion(194, key.fresh == nil, "!src.is_fresh_value()"); err != nil {
			return false, err
		}
		if err := g.sourceAssertion(197, key.node != nil && key.node.root == key.node, "n == n->get_root()"); err != nil {
			return false, err
		}
		if key.node == nil || key.node.root != key.node {
			return false, ast.ErrASTDomain
		}
		procedure := procedures[key.node]
		if err := g.sourceAssertion(201, procedure != nil, "proc"); err != nil {
			return false, err
		}
		if procedure == nil {
			return false, ast.ErrASTDomain
		}
		for _, dependency := range procedure.Dependencies() {
			child, err := smtModelDependencyKey(dependency)
			if err != nil {
				return false, err
			}
			done, err := pushChild(child)
			if err != nil {
				return false, err
			}
			visited = visited && done
			if err = g.traceDependency(key.node, child, 209); err != nil {
				return false, err
			}
		}
		return visited, nil
	}
	process := func(key smtModelSourceKey) error {
		if err := g.sourceAssertion(229, colors[key] != gray, "color != Grey"); err != nil {
			return err
		}
		if colors[key] == gray {
			return ast.ErrASTDomain
		}
		if colors[key] == black {
			return nil
		}
		if err := g.sourceAssertion(232, colors[key] == white, "color == White"); err != nil {
			return err
		}
		if err := g.traceProcessSource(224, "process source", key, len(todo)); err != nil {
			return err
		}
		todo = append(todo, key)
		for len(todo) != 0 {
			current := todo[len(todo)-1]
			if err := g.traceProcessSource(236, "current source", current, len(todo)); err != nil {
				return err
			}
			switch colors[current] {
			case white:
				colors[current] = gray
				if _, err := visitChildren(current); err != nil {
					return err
				}
			case gray:
				// This second visit is inside native SASSERT, including its effects.
				if smtcoreSourceDebug {
					visited, err := visitChildren(current)
					if err != nil {
						return err
					}
					if err = g.sourceAssertion(246, visited, "visit_children(curr, roots, root2proc, colors, already_traversed, todo)"); err != nil {
						return err
					}
				}
				colors[current] = black
				dependency, err := smtModelKeyDependency(current)
				if err != nil {
					return err
				}
				result = append(result, dependency)
			case black:
				todo = todo[:len(todo)-1]
			default:
				return diagnostic.BorrowProcessDebugContext().UnreachableSource("src/smt/smt_model_generator.cpp", 254)
			}
		}
		if err := g.traceModel("mg_top_sort", "process_source", 257, func() bool { return true }, func(out io.Writer) error {
			_, traceErr := fmt.Fprintf(out, "END process_source, todo.size(): %d\n", len(todo))
			return traceErr
		}); err != nil {
			return err
		}
		return nil
	}
	for _, fresh := range g.extraFresh {
		if err := process(smtModelSourceKey{fresh: fresh}); err != nil {
			return nil, err
		}
	}
	for _, root := range roots {
		procedure := procedures[root]
		if err := g.sourceAssertion(288, procedure != nil, "proc"); err != nil {
			return nil, err
		}
		if procedure == nil {
			return nil, ast.ErrASTDomain
		}
		if procedure.IsFresh() {
			if err := process(smtModelSourceKey{node: root}); err != nil {
				return nil, err
			}
		}
	}
	for _, root := range roots {
		if err := process(smtModelSourceKey{node: root}); err != nil {
			return nil, err
		}
	}
	return result, nil
}
