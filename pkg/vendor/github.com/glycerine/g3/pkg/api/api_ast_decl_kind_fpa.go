package api

// Source: src/api/api_ast.cpp Z3_get_decl_kind floating-point switch;
// src/ast/fpa_decl_plugin.h fpa_op_kind.

import "github.com/glycerine/g3/pkg/ast"

const (
	APIDeclarationFPARMNearestTiesToEven APIDeclarationKind = 450 + iota
	APIDeclarationFPARMNearestTiesToAway
	APIDeclarationFPARMTowardPositive
	APIDeclarationFPARMTowardNegative
	APIDeclarationFPARMTowardZero
	APIDeclarationFPANumeral
	APIDeclarationFPAPlusInfinity
	APIDeclarationFPAMinusInfinity
	APIDeclarationFPANaN
	APIDeclarationFPAPlusZero
	APIDeclarationFPAMinusZero
	APIDeclarationFPAAdd
	APIDeclarationFPASubtract
	APIDeclarationFPANegate
	APIDeclarationFPAMultiply
	APIDeclarationFPADivide
	APIDeclarationFPARemainder
	APIDeclarationFPAAbsolute
	APIDeclarationFPAMinimum
	APIDeclarationFPAMaximum
	APIDeclarationFPAFusedMultiplyAdd
	APIDeclarationFPASquareRoot
	APIDeclarationFPARoundToIntegral
	APIDeclarationFPAEqual
	APIDeclarationFPALessThan
	APIDeclarationFPAGreaterThan
	APIDeclarationFPALessEqual
	APIDeclarationFPAGreaterEqual
	APIDeclarationFPAIsNaN
	APIDeclarationFPAIsInfinite
	APIDeclarationFPAIsZero
	APIDeclarationFPAIsNormal
	APIDeclarationFPAIsSubnormal
	APIDeclarationFPAIsNegative
	APIDeclarationFPAIsPositive
	APIDeclarationFPAConstruct
	APIDeclarationFPAToFloatingPoint
	APIDeclarationFPAToFloatingPointUnsigned
	APIDeclarationFPAToUnsignedBitVector
	APIDeclarationFPAToSignedBitVector
	APIDeclarationFPAToReal
	APIDeclarationFPAToIEEEBitVector
	APIDeclarationFPABitVectorWrap
	APIDeclarationFPABitVectorToRoundingMode
)

// FloatingPointDeclarationKind classifies a floating-point declaration or app.
func (c *SMTContext) FloatingPointDeclarationKind(value ast.ASTHandle) (APIDeclarationKind, error) {
	manager, family, operation, err := c.apiDeclarationFamilyOperation(value)
	if err != nil {
		return APIDeclarationInternal, err
	}
	floatingPoint, err := manager.ExistingFloatingPointFamilyID()
	if err != nil {
		return APIDeclarationInternal, err
	}
	if floatingPoint == ast.NullFamilyID || family != floatingPoint {
		return APIDeclarationInternal, ast.ErrASTDomain
	}
	return apiFloatingPointDeclarationKind(operation), nil
}

func apiFloatingPointDeclarationKind(operation int32) APIDeclarationKind {
	if kind := apiFPARoundingDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiFPAValueDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiFPAArithmeticDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiFPARelationDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	if kind := apiFPAConversionDeclarationKind(operation); kind != APIDeclarationInternal {
		return kind
	}
	return apiFPAExtensionDeclarationKind(operation)
}
