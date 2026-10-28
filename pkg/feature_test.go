package apalache

import "testing"

func TestFeatureRegistryMatchesScala(t *testing.T) {
	features := AllFeatures()
	if len(features) != 2 || features[0] != RowsFeature || features[1] != ImpreciseRecordsFeature {
		t.Fatalf("features = %#v", features)
	}
	for _, feature := range features {
		if feature.Name() == "" || feature.Description() == "" || feature.String() != feature.Name() {
			t.Errorf("invalid feature %#v", feature)
		}
		if got, ok := FeatureFromString(feature.Name()); !ok || got != feature {
			t.Errorf("FeatureFromString(%q) = %#v, %v", feature.Name(), got, ok)
		}
	}
	features[0] = Feature{}
	if AllFeatures()[0] != RowsFeature {
		t.Fatal("AllFeatures exposed the registry backing slice")
	}
	if _, ok := FeatureFromString("unknown"); ok {
		t.Fatal("unknown feature was accepted")
	}
}

func TestModulePropertiesAndOperatorParametersMatchScala(t *testing.T) {
	properties := []ModuleProperty{
		PropertyTemporalEncoded, PropertyDesugared, PropertyTypeChecked, PropertyConfigured,
		PropertyInlined, PropertyPrimed, PropertyVCGenerated, PropertyTransitionsFound,
		PropertyPreprocessed, PropertyOptimized, PropertyAnalyzed,
	}
	seen := make(map[ModuleProperty]struct{}, len(properties))
	module := NewModule("M", nil)
	for _, property := range properties {
		if _, duplicate := seen[property]; duplicate {
			t.Fatalf("duplicate module property %d", property)
		}
		seen[property] = struct{}{}
		if module.HasProperty(property) {
			t.Fatalf("new module unexpectedly has property %d", property)
		}
		module.AddProperty(property)
		if !module.HasProperty(property) {
			t.Fatalf("property %d was not retained", property)
		}
	}

	if ValueParam("x") != (OperatorParam{Name: "x", Arity: 0}) || ValueParam("x").IsOperator() {
		t.Fatal("value parameter differs from Scala")
	}
	if !(OperatorParam{Name: "F", Arity: 1}).IsOperator() || (OperatorParam{Name: "bad", Arity: -1}).IsOperator() {
		t.Fatal("operator parameter arity predicate differs from Scala")
	}
}
