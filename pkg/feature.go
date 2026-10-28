package apalache

type Feature struct {
	name        string
	description string
}

func (feature Feature) Name() string        { return feature.name }
func (feature Feature) Description() string { return feature.description }
func (feature Feature) String() string      { return feature.name }

var (
	RowsFeature = Feature{
		name:        "rows",
		description: "enable row typing as explained in ADR-014 (default in versions after 0.29.0)",
	}
	ImpreciseRecordsFeature = Feature{
		name:        "no-rows",
		description: "disable row types, make records imprecise (default in versions prior to 0.29.0)",
	}
	allFeatures = []Feature{RowsFeature, ImpreciseRecordsFeature}
)

func AllFeatures() []Feature { return append([]Feature(nil), allFeatures...) }

func FeatureFromString(name string) (Feature, bool) {
	for _, feature := range allFeatures {
		if feature.name == name {
			return feature, true
		}
	}
	return Feature{}, false
}
