package fp

// Source: src/muz/spacer/spacer_dl_interface.cpp query setup and dispatch.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *PropositionalEngine) preprocessHornRules(query ast.ASTHandle, allowSubsumptionResultTransforms, allowInlineResultTransforms bool) (_ *hornPreprocessResult, resultErr error) {
	return e.preprocessHornRulesWithSliceProof(query, allowSubsumptionResultTransforms, allowInlineResultTransforms, false)
}

func (e *PropositionalEngine) preprocessHornRulesWithSliceProof(query ast.ASTHandle, allowSubsumptionResultTransforms, allowInlineResultTransforms, allowSliceProof bool) (_ *hornPreprocessResult, resultErr error) {
	return e.preprocessHornRulesWithQueryRule(query, nil, allowSubsumptionResultTransforms, allowInlineResultTransforms, allowSliceProof, true)
}

func (e *PropositionalEngine) preprocessHornRulesWithQueryRule(query ast.ASTHandle, queryRule *base.Rule, allowSubsumptionResultTransforms, allowInlineResultTransforms, allowSliceProof, engineTransforms bool) (_ *hornPreprocessResult, resultErr error) {
	return e.preprocessHornRulesWithStages(query, queryRule, allowSubsumptionResultTransforms, allowInlineResultTransforms, allowSliceProof, engineTransforms, engineTransforms)
}

// BMC shares default transformations and slicing with Spacer, but its source
// query path does not run Spacer's optional coalescing and unfolding stage.
func (e *PropositionalEngine) preprocessHornRulesWithStages(query ast.ASTHandle, queryRule *base.Rule, allowSubsumptionResultTransforms, allowInlineResultTransforms, allowSliceProof, sliceStage, unfoldStage bool) (_ *hornPreprocessResult, resultErr error) {
	var slicedPredicates map[ast.ASTHandle]ast.ASTHandle
	var slicedColumns map[ast.ASTHandle][]bool
	var sliceProof *transforms.SliceProofConverter
	prepared, err := base.NewRuleSet(e.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, prepared.Close())
			if sliceProof != nil {
				resultErr = errors.Join(resultErr, sliceProof.Close())
			}
		}
	}()
	if err = prepared.AddRules(e.set); err != nil {
		return nil, err
	}
	if queryRule != nil {
		if err = prepared.Add(queryRule); err != nil {
			return nil, err
		}
	}
	// rule_manager::mk_query registers the synthetic/direct query predicate as
	// an output before apply_default_transformation runs.
	if err = prepared.SetOutputPredicate(query); err != nil {
		return nil, err
	}
	valid, err := prepared.Finalize()
	if err != nil || !valid {
		if err == nil {
			err = fmt.Errorf("muz/fp: negation is not stratified")
		}
		return nil, err
	}
	normalization, err := e.ruleNormalizationParams()
	if err != nil {
		return nil, err
	}
	transformer := base.NewRuleTransformer()
	initialCone := transforms.NewConePluginWithRelations(e.relations, 0)
	if err = transformer.Register(initialCone); err != nil {
		return nil, errors.Join(err, initialCone.Close(), transformer.Close())
	}
	firstSimplifier, makeErr := transforms.NewInterpTailSimplifierPlugin(e.manager, e.rules, e.params, e.globals, 40000)
	if makeErr != nil {
		return nil, errors.Join(makeErr, transformer.Close())
	}
	firstSimplifier.SetNormalizationParams(normalization)
	if err = transformer.Register(firstSimplifier); err != nil {
		return nil, errors.Join(err, firstSimplifier.Close(), transformer.Close())
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	useSubsumption, err := parameters.Bool("datalog.subsumption", true)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	if !allowSubsumptionResultTransforms {
		useSubsumption = false
	}
	useChecker, err := parameters.Bool("xform.subsumption_checker", true)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	inlineEager, err := parameters.Bool("xform.inline_eager", true)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	inlineLinear, err := parameters.Bool("xform.inline_linear", true)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	inlineBranching, err := parameters.Bool("xform.inline_linear_branch", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	// PDR exposes relation models and inductive certificates. Preserve stages
	// whose result converters are not retained while sharing convertible ones.
	if !allowInlineResultTransforms {
		inlineEager = false
		inlineLinear = false
	}
	unfoldRounds, err := parameters.UInt("xform.unfold_rules", 0)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	coalesceRules, err := parameters.Bool("xform.coalesce_rules", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	generateProofTrace, err := parameters.Bool("generate_proof_trace", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	generateProofTrace = generateProofTrace || e.forceProofTrace
	sliceRules, err := parameters.Bool("xform.slice", true)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	transformArrays, err := parameters.Bool("xform.transform_arrays", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	instantiateQuantifiers, err := parameters.Bool("xform.instantiate_quantifiers", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	quantifyArrays, err := parameters.Bool("xform.quantify_arrays", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	instantiateArrays, err := parameters.Bool("xform.instantiate_arrays", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	instantiateArraysEnforce, err := parameters.Bool("xform.instantiate_arrays.enforce", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	instantiateArraysQuantifiers, err := parameters.UInt("xform.instantiate_arrays.nb_quantifier", 1)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	arrayBlast, err := parameters.Bool("xform.array_blast", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	bitBlast, err := parameters.Bool("xform.bit_blast", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	scale, err := parameters.Bool("xform.scale", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	magic, err := parameters.Bool("xform.magic", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	karr, err := parameters.Bool("xform.karr", false)
	if err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	var inliners []*transforms.RuleInlinerPlugin
	var subsumers []*transforms.SubsumptionCheckerPlugin
	newInliner := func(priority uint32) *transforms.RuleInlinerPlugin {
		plugin := transforms.NewRuleInlinerPluginWithServices(e.manager, e.rules, []ast.ASTHandle{query}, inlineEager, inlineLinear, e.params, e.globals, priority)
		plugin.SetNormalizationParams(normalization)
		plugin.SetLinearBranching(inlineBranching)
		inliners = append(inliners, plugin)
		return plugin
	}
	newSubsumption := func(priority uint32) *transforms.SubsumptionCheckerPlugin {
		plugin := transforms.NewSubsumptionCheckerPlugin(useChecker, priority)
		subsumers = append(subsumers, plugin)
		return plugin
	}
	if useSubsumption {
		if err = transformer.Register(newSubsumption(35005)); err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
	}
	if instantiateQuantifiers {
		if err = transformer.Register(transforms.NewQuantifierInstantiationPlugin(e.manager, e.rules, 37000)); err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
	}
	var scaler *transforms.ScalePlugin
	var scaledPredicates map[ast.ASTHandle]ast.ASTHandle
	if scale {
		scaler = transforms.NewScalePlugin(e.manager, e.rules, 36030)
		if err = transformer.Register(scaler); err != nil {
			return nil, errors.Join(err, scaler.Close(), transformer.Close())
		}
	}
	var magicTransformer *transforms.MagicSymbolicPlugin
	var magicPredicates map[ast.ASTHandle]ast.ASTHandle
	var karrTransformer *transforms.KarrInvariantsPlugin
	var karrInvariants map[ast.ASTHandle]rel.KarrMatrix
	if magic {
		magicTransformer = transforms.NewMagicSymbolicPlugin(e.manager, e.rules, 36020)
		if err = transformer.Register(magicTransformer); err != nil {
			return nil, errors.Join(err, magicTransformer.Close(), transformer.Close())
		}
	}
	if karr {
		karrTransformer = transforms.NewKarrInvariantsPlugin(e.manager, e.rules, 36010)
		if err = transformer.Register(karrTransformer); err != nil {
			return nil, errors.Join(err, karrTransformer.Close(), transformer.Close())
		}
	}
	var quantifierAbstraction *transforms.QuantifierAbstractionPlugin
	var quantifiedPredicates map[ast.ASTHandle]ast.ASTHandle
	if quantifyArrays {
		quantifierAbstraction = transforms.NewQuantifierAbstractionPlugin(e.manager, e.rules, 38000)
		if err = transformer.Register(quantifierAbstraction); err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
	}
	if arrayBlast && !quantifyArrays {
		blast, makeErr := transforms.NewArrayBlastPlugin(e.manager, e.rules, e.params, e.globals, 35999)
		if makeErr != nil {
			return nil, errors.Join(makeErr, transformer.Close())
		}
		blast.SetNormalizationParams(normalization)
		if err = transformer.Register(blast); err != nil {
			return nil, errors.Join(err, blast.Close(), transformer.Close())
		}
	}
	var arrayInstantiation *transforms.ArrayInstantiationPlugin
	var instantiatedPredicates map[ast.ASTHandle]ast.ASTHandle
	if instantiateArrays {
		arrayInstantiation = transforms.NewArrayInstantiationPlugin(e.manager, e.rules, instantiateArraysQuantifiers, instantiateArraysEnforce, 34999)
		if err = transformer.Register(arrayInstantiation); err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
	}
	if err = transformer.Register(newInliner(35000)); err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	var bitBlaster *transforms.BitBlastPlugin
	var bitBlastedPredicates map[ast.ASTHandle]ast.ASTHandle
	if bitBlast {
		bitBlaster, err = transforms.NewBitBlastPlugin(e.manager, e.rules, e.params, e.globals, 35000)
		if err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
		bitBlaster.SetNormalizationParams(normalization)
		if err = transformer.Register(bitBlaster); err != nil {
			return nil, errors.Join(err, bitBlaster.Close(), transformer.Close())
		}
	}
	if transformArrays {
		arrayRewrite, makeErr := transforms.NewArrayEqRewritePlugin(e.manager, e.rules, e.params, e.globals, 34998)
		if makeErr != nil {
			return nil, errors.Join(makeErr, transformer.Close())
		}
		if err = transformer.Register(arrayRewrite); err != nil {
			return nil, errors.Join(err, arrayRewrite.Close(), transformer.Close())
		}
	}
	laterCone := transforms.NewConePluginWithRelations(e.relations, 34990)
	if err = transformer.Register(laterCone); err != nil {
		return nil, errors.Join(err, laterCone.Close(), transformer.Close())
	}
	secondSimplifier, makeErr := transforms.NewInterpTailSimplifierPlugin(e.manager, e.rules, e.params, e.globals, 34980)
	if makeErr != nil {
		return nil, errors.Join(makeErr, transformer.Close())
	}
	secondSimplifier.SetNormalizationParams(normalization)
	if err = transformer.Register(secondSimplifier); err != nil {
		return nil, errors.Join(err, secondSimplifier.Close(), transformer.Close())
	}
	if useSubsumption {
		if err = transformer.Register(newSubsumption(34975)); err != nil {
			return nil, errors.Join(err, transformer.Close())
		}
	}
	if err = transformer.Register(newInliner(34970)); err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	finalCone := transforms.NewConePluginWithRelations(e.relations, 34960)
	if err = transformer.Register(finalCone); err != nil {
		return nil, errors.Join(err, finalCone.Close(), transformer.Close())
	}
	thirdSimplifier, makeErr := transforms.NewInterpTailSimplifierPlugin(e.manager, e.rules, e.params, e.globals, 34950)
	if makeErr != nil {
		return nil, errors.Join(makeErr, transformer.Close())
	}
	thirdSimplifier.SetNormalizationParams(normalization)
	if err = transformer.Register(thirdSimplifier); err != nil {
		return nil, errors.Join(err, thirdSimplifier.Close(), transformer.Close())
	}
	if useSubsumption {
		for _, stage := range []struct {
			priority uint32
			inlining bool
		}{
			{priority: 34940},
			{priority: 34930, inlining: true},
			{priority: 34920},
			{priority: 34910, inlining: true},
			{priority: 34900},
			{priority: 34890, inlining: true},
			{priority: 34880},
		} {
			var plugin base.RuleTransformPlugin
			if stage.inlining {
				plugin = newInliner(stage.priority)
			} else {
				plugin = newSubsumption(stage.priority)
			}
			if err = transformer.Register(plugin); err != nil {
				return nil, errors.Join(err, plugin.Close(), transformer.Close())
			}
		}
	} else if err = transformer.Register(newInliner(34930)); err != nil {
		return nil, errors.Join(err, transformer.Close())
	}
	_, err = transformer.Apply(prepared)
	var initialConeSteps, laterConeSteps, finalConeSteps []transforms.ConeModelStep
	var inlineModelRules [][]transforms.InlineModelRule
	var subsumptionModelSteps [][]transforms.ConeModelStep
	var modelConversions []hornModelConversion
	keepModelConversions := false
	defer func() {
		if !keepModelConversions {
			for _, rules := range inlineModelRules {
				resultErr = errors.Join(resultErr, transforms.CloseInlineModelRules(rules))
			}
			for _, steps := range subsumptionModelSteps {
				resultErr = errors.Join(resultErr, transforms.CloseConeModelSteps(steps))
			}
			resultErr = errors.Join(resultErr,
				transforms.CloseConeModelSteps(initialConeSteps),
				transforms.CloseConeModelSteps(laterConeSteps),
				transforms.CloseConeModelSteps(finalConeSteps),
				closeHornModelConversions(modelConversions))
		}
	}()
	if err == nil {
		initialConeSteps, err = initialCone.TakeModelSteps()
	}
	if err == nil {
		laterConeSteps, err = laterCone.TakeModelSteps()
	}
	if err == nil {
		finalConeSteps, err = finalCone.TakeModelSteps()
	}
	if err == nil {
		inlineModelRules = make([][]transforms.InlineModelRule, len(inliners))
		for index, inliner := range inliners {
			inlineModelRules[index], err = inliner.TakeModelRules()
			if err != nil {
				break
			}
		}
	}
	if err == nil {
		subsumptionModelSteps = make([][]transforms.ConeModelStep, len(subsumers))
		for index, subsumer := range subsumers {
			subsumptionModelSteps[index], err = subsumer.TakeModelSteps()
			if err != nil {
				break
			}
		}
	}
	if err == nil && quantifierAbstraction != nil {
		quantifiedPredicates = quantifierAbstraction.FreshPredicates()
		for _, declaration := range quantifiedPredicates {
			state, stateErr := e.predicateState(declaration)
			if stateErr != nil {
				err = stateErr
				break
			}
			e.predicates[declaration] = state
		}
	}
	if err == nil && arrayInstantiation != nil {
		instantiatedPredicates = arrayInstantiation.FreshPredicates()
		for _, declaration := range instantiatedPredicates {
			state, stateErr := e.predicateState(declaration)
			if stateErr != nil {
				err = fmt.Errorf("muz/fp: array-instantiated predicate state: %w", stateErr)
				break
			}
			e.predicates[declaration] = state
		}
	}
	if err == nil && bitBlaster != nil {
		bitBlastedPredicates = bitBlaster.FreshPredicates()
		for _, declaration := range bitBlastedPredicates {
			state, stateErr := e.predicateState(declaration)
			if stateErr != nil {
				err = fmt.Errorf("muz/fp: bit-blasted predicate state: %w", stateErr)
				break
			}
			e.predicates[declaration] = state
		}
	}
	if err == nil && scaler != nil {
		scaledPredicates = scaler.FreshPredicates()
		for _, declaration := range scaledPredicates {
			state, stateErr := e.predicateState(declaration)
			if stateErr != nil {
				err = fmt.Errorf("muz/fp: scaled predicate state: %w", stateErr)
				break
			}
			e.predicates[declaration] = state
		}
	}
	if err == nil && magicTransformer != nil {
		magicPredicates = magicTransformer.AnswerPredicates()
		for _, declaration := range magicPredicates {
			state, stateErr := e.predicateState(declaration)
			if stateErr != nil {
				err = fmt.Errorf("muz/fp: magic answer predicate state: %w", stateErr)
				break
			}
			e.predicates[declaration] = state
		}
	}
	if err == nil && karrTransformer != nil {
		karrInvariants = karrTransformer.ModelInvariants()
	}
	err = errors.Join(err, transformer.Close())
	if err != nil {
		return nil, fmt.Errorf("muz/fp: Spacer rule preprocessing: %w", err)
	}
	// Proof-producing backends opt in only when they apply the converter.
	if sliceStage && sliceRules && (!generateProofTrace || allowSliceProof) {
		slicer := base.NewRuleTransformer()
		plugin := transforms.NewRuleSlicePlugin(e.manager, e.rules, 1)
		plugin.SetGenerateProofTrace(generateProofTrace)
		if err = slicer.Register(plugin); err != nil {
			return nil, errors.Join(err, plugin.Close(), slicer.Close())
		}
		_, err = slicer.Apply(prepared)
		if err == nil {
			slicedPredicates, slicedColumns = plugin.PredicateSlices()
			if generateProofTrace {
				sliceProof = plugin.TakeProofConverter()
				if sliceProof != nil {
					err = errors.Join(err, sliceProof.SetNormalizationServices(e.params, e.globals))
					sliceProof.SetNormalizationParams(normalization)
				}
			}
		}
		err = errors.Join(err, slicer.Close())
		if err != nil {
			return nil, fmt.Errorf("muz/fp: Spacer rule slicing: %w", err)
		}
	}
	if unfoldStage && unfoldRounds > 0 && coalesceRules {
		coalescer := base.NewRuleTransformer()
		plugin := transforms.NewRuleCoalescePluginWithProofTrace(e.manager, e.rules, 50, generateProofTrace)
		if err = coalescer.Register(plugin); err != nil {
			return nil, errors.Join(err, plugin.Close(), coalescer.Close())
		}
		_, err = coalescer.Apply(prepared)
		err = errors.Join(err, coalescer.Close())
		if err != nil {
			return nil, fmt.Errorf("muz/fp: Spacer rule coalescing: %w", err)
		}
	}
	for round := uint32(0); unfoldStage && round < unfoldRounds; round++ {
		unfolder := base.NewRuleTransformer()
		plugin := transforms.NewRuleUnfoldPlugin(e.manager, e.rules, e.params, e.globals, 100)
		plugin.SetNormalizationParams(normalization)
		if err = unfolder.Register(plugin); err != nil {
			return nil, errors.Join(err, plugin.Close(), unfolder.Close())
		}
		_, err = unfolder.Apply(prepared)
		err = errors.Join(err, unfolder.Close())
		if err != nil {
			return nil, fmt.Errorf("muz/fp: Spacer rule unfolding round %d: %w", round+1, err)
		}
	}
	keep = true
	appendConeSteps := func(steps []transforms.ConeModelStep) {
		for _, step := range steps {
			modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertCone, cone: step})
		}
	}
	appendConeSteps(initialConeSteps)
	initialConeSteps = nil
	if len(quantifiedPredicates) != 0 {
		modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertQuantifier, predicates: quantifiedPredicates})
	}
	if len(scaledPredicates) != 0 {
		modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertScale, predicates: scaledPredicates})
	}
	if len(karrInvariants) != 0 {
		modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertKarr, karr: karrInvariants})
	}
	appendInlineRules := func(index int) {
		if index < len(inlineModelRules) && len(inlineModelRules[index]) != 0 {
			modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertInline, inline: inlineModelRules[index]})
			inlineModelRules[index] = nil
		}
	}
	appendSubsumptionSteps := func(index int) {
		if index >= len(subsumptionModelSteps) {
			return
		}
		appendConeSteps(subsumptionModelSteps[index])
		subsumptionModelSteps[index] = nil
	}
	appendSubsumptionSteps(0)
	appendInlineRules(0)
	if len(bitBlastedPredicates) != 0 {
		modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertBitBlast, predicates: bitBlastedPredicates})
	}
	appendConeSteps(laterConeSteps)
	laterConeSteps = nil
	appendSubsumptionSteps(1)
	appendInlineRules(1)
	appendConeSteps(finalConeSteps)
	finalConeSteps = nil
	for index := 2; index < len(subsumptionModelSteps) || index < len(inlineModelRules); index++ {
		appendSubsumptionSteps(index)
		appendInlineRules(index)
	}
	if len(slicedPredicates) != 0 {
		modelConversions = append(modelConversions, hornModelConversion{kind: hornConvertSlice, predicates: slicedPredicates, columns: slicedColumns})
	}
	keepModelConversions = true
	return &hornPreprocessResult{
		rules:                  prepared,
		slicedPredicates:       slicedPredicates,
		slicedColumns:          slicedColumns,
		quantifiedPredicates:   quantifiedPredicates,
		instantiatedPredicates: instantiatedPredicates,
		bitBlastedPredicates:   bitBlastedPredicates,
		scaledPredicates:       scaledPredicates,
		magicPredicates:        magicPredicates,
		karrInvariants:         karrInvariants,
		modelConversions:       modelConversions,
		sliceProof:             sliceProof,
	}, nil
}

func (e *PropositionalEngine) querySpacer(declaration ast.ASTHandle) (_ lbool.LBool, resultErr error) {
	return e.querySpacerAtLevel(declaration, 0, 2)
}

func (e *PropositionalEngine) querySpacerFromLevel(declaration ast.ASTHandle, fromLevel uint32) (_ lbool.LBool, resultErr error) {
	return e.querySpacerAtLevel(declaration, fromLevel, 1)
}

func (e *PropositionalEngine) querySpacerAtLevel(declaration ast.ASTHandle, fromLevel, emptyModelVerbosity uint32) (_ lbool.LBool, resultErr error) {
	if e.params == nil || e.globals == nil {
		return lbool.Undef, fmt.Errorf("muz/fp: Spacer requires parameter and global services")
	}
	if e.pdr != nil {
		if err := e.closePDRForReplacement(); err != nil {
			return lbool.Undef, err
		}
	}
	finalized, err := e.set.IsFinalized()
	if err != nil {
		return lbool.Undef, err
	}
	if !finalized {
		valid, x := e.set.Finalize()
		if x != nil {
			return lbool.Undef, x
		}
		if !valid {
			return lbool.Undef, ErrUnstratifiedNegation
		}
	}
	// Spacer uses apply_default_transformation, not rel_context's filter/
	// simple-join pipeline; its predicate transformers handle full Horn tails.
	if err = e.validateFragment(); err != nil {
		return lbool.Undef, err
	}
	parameters, err := e.fixedpointParamView()
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, parameters.Close()) }()
	proofTrace, err := parameters.Bool("generate_proof_trace", false)
	if err != nil {
		return lbool.Undef, err
	}
	queryDeclaration := declaration
	var queryRule *base.Rule
	if !e.auxiliary[declaration] {
		queryDeclaration, queryRule, err = e.makeSpacerRelationQuery(declaration, proofTrace || e.forceProofTrace)
		if err != nil {
			return lbool.Undef, err
		}
		defer func() { resultErr = errors.Join(resultErr, queryRule.Close()) }()
	}
	// Source check_reset sees the synthetic query rule as well as the stored
	// program. A fresh query head cannot be subsumed by the prior query rule;
	// inheriting frames here can mix old predicates with freshly sliced ones.
	resetRules, err := e.shouldResetSpacerRules()
	if err != nil {
		return lbool.Undef, err
	}
	if queryRule != nil && e.spacerRules != nil && e.spacerQuery != queryDeclaration {
		resetRules = true
	}
	snapshot, err := snapshotHornRules(e.set)
	if err != nil {
		return lbool.Undef, err
	}
	installedSnapshot := false
	defer func() {
		if !installedSnapshot {
			resultErr = errors.Join(resultErr, snapshot.Close())
		}
	}()
	// Spacer's overall proof hook is empty in this source version; reachable
	// answers and unreachable models use its trace/model conversion paths.
	preprocessed, err := e.preprocessHornRulesWithQueryRule(queryDeclaration, queryRule, true, true, true, true)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, preprocessed.Close()) }()
	prepared := preprocessed.rules
	if err = closeHornModelConversions(e.spacerConversions); err != nil {
		return lbool.Undef, err
	}
	e.spacerConversions = nil
	e.spacerPredicates = preprocessed.slicedPredicates
	e.spacerSlices = preprocessed.slicedColumns
	e.spacerQuantified = preprocessed.quantifiedPredicates
	e.spacerInstantiated = preprocessed.instantiatedPredicates
	e.spacerBitBlasted = preprocessed.bitBlastedPredicates
	e.spacerScaled = preprocessed.scaledPredicates
	e.spacerMagic = preprocessed.magicPredicates
	e.spacerKarr = preprocessed.karrInvariants
	e.spacerConversions = preprocessed.takeModelConversions()
	queryDeclaration, deriveErr := prepared.DerivedPredicate(queryDeclaration)
	if deriveErr != nil {
		return lbool.Undef, fmt.Errorf("muz/fp: resolve preprocessed Spacer query: %w", deriveErr)
	}
	maximumLevels, err := parameters.UInt("spacer.max_level", ^uint32(0))
	if err != nil {
		return lbool.Undef, err
	}
	context, err := e.spacerContext()
	if err != nil {
		return lbool.Undef, err
	}
	closeContext := func() error {
		if e.spacerRules == nil {
			initialized, stateErr := context.Initialized()
			if stateErr == nil && !initialized {
				return nil
			}
			if stateErr != nil {
				return errors.Join(stateErr, e.closeSpacerForReplacement())
			}
		}
		return e.closeSpacerForReplacement()
	}
	if resetRules {
		if err = context.Reset(); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
	}

	background, err := e.manager.RefGet(e.background)
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	if err = context.SetBackground(background); err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	// Source init_rules creates transformers only for predicates in the
	// transformed program. Registry-only predicates may have been eliminated;
	// giving them unconstrained interpretations would corrupt model conversion.
	if err = context.UpdateRules(prepared); err != nil {
		return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: initialize preprocessed Spacer rules: %w", err), closeContext())
	}
	if err = context.SetQuery(queryDeclaration); err != nil {
		return lbool.Undef, errors.Join(fmt.Errorf("muz/fp: set preprocessed Spacer query: %w", err), closeContext())
	}
	preparedRuleCount, err := prepared.Len()
	if err != nil {
		return lbool.Undef, errors.Join(err, closeContext())
	}
	previousSnapshot := e.spacerRules
	e.spacerRules = snapshot
	installedSnapshot = true
	if previousSnapshot != nil {
		if err = previousSnapshot.Close(); err != nil {
			return lbool.Undef, errors.Join(err, closeContext())
		}
	}
	e.spacer = context
	e.spacerRevision = e.revision
	e.spacerOriginalQuery = declaration
	e.spacerQuery = queryDeclaration
	if preparedRuleCount == 0 {
		if err = context.SetEmptyProgramUnsat(); err != nil {
			return lbool.Undef, err
		}
		if err = e.writeSpacerEmptyProgramModel(emptyModelVerbosity); err != nil {
			return lbool.Undef, err
		}
		return lbool.False, nil
	}
	return context.Solve(fromLevel, maximumLevels)
}
