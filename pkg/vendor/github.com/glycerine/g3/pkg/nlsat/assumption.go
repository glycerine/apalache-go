package nlsat

// Source: src/nlsat/nlsat_types.h:36-37; nlsat_solver.cpp:56-64,1354-1416;
// src/nlsat/tactic/goal2nlsat.cpp:251; nlsat_tactic.cpp:178-184.

import "github.com/glycerine/g3/pkg/sat"

// AssumptionAttachment is an opaque frontend attachment identity. The frontend
// keeps its concrete dependency mapping and references; NLSAT neither knows nor
// owns that payload. A nonzero-sized token gives distinct Go allocations distinct
// identities, without importing AST into the arithmetic solver.
type AssumptionAttachment struct{ identity byte }

// Assumption is the retained void* value domain: a frontend attachment or the
// identity of an actual input literal. Equal literals at different input positions
// are distinct assumptions. Zero is the null assumption. Values are comparable.
type Assumption struct {
	attachment *AssumptionAttachment
	literal    *sat.Literal
}

func AttachedAssumption(a *AssumptionAttachment) Assumption { return Assumption{attachment: a} }
func literalAssumption(l *sat.Literal) Assumption           { return Assumption{literal: l} }
func (a Assumption) IsNull() bool                           { return a == Assumption{} }
func (a Assumption) Attachment() *AssumptionAttachment      { return a.attachment }

// InputLiteral returns the borrowed original element, not a copied literal.
// The owning check call keeps that input storage alive until core extraction and
// temporary-clause collection have finished.
func (a Assumption) InputLiteral() *sat.Literal { return a.literal }
