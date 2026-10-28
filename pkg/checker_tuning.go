package apalache

import (
	"fmt"
	"regexp"
	"sort"
)

// CheckerTuning is the validated subset of Scala search tuning whose
// semantics are implemented by the Go checker. Regular expressions use full
// matches, as java.util.regex.Matcher.matches does in Scala.
type CheckerTuning struct {
	transitionFilter *regexp.Regexp
	invariantFilter  *regexp.Regexp
	// Explicit path enumeration has no joined transition relation, so the
	// Scala before/after modes have identical behavior after validation.
	invariantMode string
}

// ParseCheckerTuning validates checker tuning accepted by the Go backend.
// Solver-specific Z3 options are deliberately rejected until g3 exposes a
// compatible parameter API rather than being silently ignored.
func ParseCheckerTuning(values map[string]string) (CheckerTuning, error) {
	result := CheckerTuning{invariantMode: "before"}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		switch key {
		case "search.transitionFilter":
			if value == "" {
				continue
			}
			compiled, err := regexp.Compile("^(?:" + value + ")$")
			if err != nil {
				return CheckerTuning{}, fmt.Errorf("invalid regex %s: %w", value, err)
			}
			result.transitionFilter = compiled
		case "search.invariantFilter":
			if value == "" {
				continue
			}
			compiled, err := regexp.Compile("^(?:" + value + ")$")
			if err != nil {
				return CheckerTuning{}, fmt.Errorf("invalid regex %s: %w", value, err)
			}
			result.invariantFilter = compiled
		case "search.invariant.mode":
			if value != "before" && value != "after" {
				return CheckerTuning{}, fmt.Errorf("invalid value %s for %s, expected one of before, after", value, key)
			}
			result.invariantMode = value
		default:
			return CheckerTuning{}, fmt.Errorf("unexpected tuning field %s", key)
		}
	}
	return result, nil
}

func (tuning CheckerTuning) allowsTransition(step, number int) bool {
	return tuning.transitionFilter == nil || tuning.transitionFilter.MatchString(fmt.Sprintf("%d->%d", step, number))
}

func (tuning CheckerTuning) allowsInvariant(step int, kind string, number int) bool {
	return tuning.invariantFilter == nil || tuning.invariantFilter.MatchString(fmt.Sprintf("%d->%s%d", step, kind, number))
}
