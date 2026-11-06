package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind basic proof operation switch.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationProofUndefined APIDeclarationKind = 100 + iota
	APIDeclarationProofTrue
	APIDeclarationProofAsserted
	APIDeclarationProofGoal
	APIDeclarationProofModusPonens
	APIDeclarationProofReflexivity
	APIDeclarationProofSymmetry
	APIDeclarationProofTransitivity
	APIDeclarationProofTransitivityStar
	APIDeclarationProofMonotonicity
	APIDeclarationProofQuantifierIntroduction
	APIDeclarationProofDistributivity
	APIDeclarationProofAndElimination
	APIDeclarationProofNotOrElimination
	APIDeclarationProofRewrite
	APIDeclarationProofRewriteStar
	APIDeclarationProofPullQuantifier
	APIDeclarationProofPushQuantifier
	APIDeclarationProofEliminateUnusedVariables
	APIDeclarationProofDER
	APIDeclarationProofQuantifierInstantiation
	APIDeclarationProofHypothesis
	APIDeclarationProofLemma
	APIDeclarationProofUnitResolution
	APIDeclarationProofIffTrue
	APIDeclarationProofIffFalse
	APIDeclarationProofCommutativity
	APIDeclarationProofDefinitionAxiom
	APIDeclarationProofDefinitionIntroduction
	APIDeclarationProofApplyDefinition
	APIDeclarationProofIffOEq
	APIDeclarationProofNNFPositive
	APIDeclarationProofNNFNegative
	APIDeclarationProofSkolemize
	APIDeclarationProofModusPonensOEq
	APIDeclarationProofTheoryLemma
	APIDeclarationProofHyperResolve
)

func apiProofDeclarationKind(operation int32) APIDeclarationKind {
	switch operation {
	case ast.ASTPrUndef:
		return APIDeclarationProofUndefined
	case ast.ASTPrTrue:
		return APIDeclarationProofTrue
	case ast.ASTPrAsserted:
		return APIDeclarationProofAsserted
	case ast.ASTPrGoal:
		return APIDeclarationProofGoal
	case ast.ASTPrModusPonens:
		return APIDeclarationProofModusPonens
	case ast.ASTPrReflexivity:
		return APIDeclarationProofReflexivity
	case ast.ASTPrSymmetry:
		return APIDeclarationProofSymmetry
	case ast.ASTPrTransitivity:
		return APIDeclarationProofTransitivity
	case ast.ASTPrTransitivityStar:
		return APIDeclarationProofTransitivityStar
	case ast.ASTPrMonotonicity:
		return APIDeclarationProofMonotonicity
	case ast.ASTPrQuantIntro:
		return APIDeclarationProofQuantifierIntroduction
	case ast.ASTPrDistributivity:
		return APIDeclarationProofDistributivity
	case ast.ASTPrAndElim:
		return APIDeclarationProofAndElimination
	case ast.ASTPrNotOrElim:
		return APIDeclarationProofNotOrElimination
	case ast.ASTPrRewrite:
		return APIDeclarationProofRewrite
	case ast.ASTPrRewriteStar:
		return APIDeclarationProofRewriteStar
	case ast.ASTPrPullQuant:
		return APIDeclarationProofPullQuantifier
	case ast.ASTPrPushQuant:
		return APIDeclarationProofPushQuantifier
	case ast.ASTPrElimUnusedVars:
		return APIDeclarationProofEliminateUnusedVariables
	case ast.ASTPrDER:
		return APIDeclarationProofDER
	case ast.ASTPrQuantInst:
		return APIDeclarationProofQuantifierInstantiation
	case ast.ASTPrHypothesis:
		return APIDeclarationProofHypothesis
	case ast.ASTPrLemma:
		return APIDeclarationProofLemma
	case ast.ASTPrUnitResolution:
		return APIDeclarationProofUnitResolution
	case ast.ASTPrIffTrue:
		return APIDeclarationProofIffTrue
	case ast.ASTPrIffFalse:
		return APIDeclarationProofIffFalse
	case ast.ASTPrCommutativity:
		return APIDeclarationProofCommutativity
	case ast.ASTPrDefAxiom:
		return APIDeclarationProofDefinitionAxiom
	case ast.ASTPrDefIntro:
		return APIDeclarationProofDefinitionIntroduction
	case ast.ASTPrApplyDef:
		return APIDeclarationProofApplyDefinition
	case ast.ASTPrIffOEq:
		return APIDeclarationProofIffOEq
	case ast.ASTPrNNFPos:
		return APIDeclarationProofNNFPositive
	case ast.ASTPrNNFNeg:
		return APIDeclarationProofNNFNegative
	case ast.ASTPrSkolemize:
		return APIDeclarationProofSkolemize
	case ast.ASTPrModusPonensOEq:
		return APIDeclarationProofModusPonensOEq
	case ast.ASTPrTheoryLemma:
		return APIDeclarationProofTheoryLemma
	case ast.ASTPrHyperResolve:
		return APIDeclarationProofHyperResolve
	default:
		return APIDeclarationInternal
	}
}
