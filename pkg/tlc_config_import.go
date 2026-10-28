package apalache

import (
	"fmt"
	"sort"
)

const (
	TLCStateConstraintPrefix  = "CONSTRAINT__si_"
	TLCActionConstraintPrefix = "ACTION_CONSTRAINT__si_"
	TLCInvariantPrefix        = "INVARIANT__si_"
	TLCTemporalPrefix         = "PROPERTY__si_"
	TLCInitDeclaration        = "INIT"
	TLCNextDeclaration        = "NEXT"
	TLCSpecificationDecl      = "SPECIFICATION"
)

type TLCConfigurationError struct{ Message string }

func (err *TLCConfigurationError) Error() string { return err.Message }

// ImportTLCConfig appends the typed synthetic declarations produced by the
// Scala TlcConfigImporter.
func ImportTLCConfig(module *Module, config TLCConfig) (*Module, error) {
	if module == nil {
		return nil, &TLCConfigurationError{Message: "cannot import TLC configuration into a nil module"}
	}
	declarations := module.Declarations()
	assignmentNames := orderedConfigKeys(config.ConstAssignments, config.ConstAssignmentOrder)
	for _, name := range assignmentNames {
		value, err := ConfigConstToExpr(config.ConstAssignments[name])
		if err != nil {
			return nil, &TLCConfigurationError{Message: fmt.Sprintf("constant %s: %v", name, err)}
		}
		declarations = append(declarations, ExpressionToOperatorDefinition(OverridePrefix+name, value))
	}
	replacementNames := orderedConfigKeys(config.ConstReplacements, config.ConstReplacementOrder)
	for _, name := range replacementNames {
		target := config.ConstReplacements[name]
		declaration, found := declarationNamed(module, target)
		if !found {
			return nil, &TLCConfigurationError{Message: fmt.Sprintf("met a replacement %s <- %s, but %s is not found", name, target, target)}
		}
		var body Expr
		switch typed := declaration.(type) {
		case *OperDecl:
			if len(typed.FormalParams()) != 0 {
				return nil, &TLCConfigurationError{Message: fmt.Sprintf("met a replacement %s <- %s, where %s is an operator of %d parameters", name, target, target, len(typed.FormalParams()))}
			}
			operatorType, ok := typed.Tag().Type()
			if !ok {
				return nil, &TLCConfigurationError{Message: fmt.Sprintf("replacement target %s is untyped", target)}
			}
			bodyType, ok := typed.Body().Tag().Type()
			if !ok {
				return nil, &TLCConfigurationError{Message: fmt.Sprintf("replacement target body %s is untyped", target)}
			}
			body = MustOperExpr(Op("OPER_APP"), Typed(bodyType), NewNameExpr(target, Typed(operatorType)))
		default:
			body = NewNameExpr(target, declaration.Tag())
		}
		declarations = append(declarations, ExpressionToOperatorDefinition(OverridePrefix+name, body))
	}
	appendBoolNames := func(prefix string, names []string) {
		for index, name := range names {
			declarations = append(declarations,
				ExpressionToOperatorDefinition(fmt.Sprintf("%s%d", prefix, index), NewNameExpr(name, Typed(BoolT1))))
		}
	}
	appendBoolNames(TLCStateConstraintPrefix, config.StateConstraints)
	appendBoolNames(TLCActionConstraintPrefix, config.ActionConstraints)
	appendBoolNames(TLCInvariantPrefix, config.Invariants)
	appendBoolNames(TLCTemporalPrefix, config.TemporalProperties)
	switch behavior := config.Behavior.(type) {
	case InitNextSpec:
		declarations = append(declarations,
			ExpressionToOperatorDefinition(TLCInitDeclaration, NewNameExpr(behavior.Init, Typed(BoolT1))),
			ExpressionToOperatorDefinition(TLCNextDeclaration, NewNameExpr(behavior.Next, Typed(BoolT1))))
	case TemporalSpec:
		declarations = append(declarations,
			ExpressionToOperatorDefinition(TLCSpecificationDecl, NewNameExpr(behavior.Name, Typed(BoolT1))))
	default:
		return nil, &TLCConfigurationError{Message: "neither INIT and NEXT, nor SPECIFICATION found in the TLC configuration"}
	}
	result := NewModule(module.Name(), declarations)
	result.properties = cloneMap(module.properties)
	return result, nil
}

func orderedConfigKeys[V any](values map[string]V, preferred []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, name := range preferred {
		if _, exists := values[name]; !exists {
			continue
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	remainder := make([]string, 0, len(values)-len(result))
	for name := range values {
		if _, exists := seen[name]; !exists {
			remainder = append(remainder, name)
		}
	}
	sort.Strings(remainder)
	return append(result, remainder...)
}
