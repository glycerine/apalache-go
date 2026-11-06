package realclosure

// Source: src/math/realclosure/realclosure.cpp:5763-5788.

import "strconv"

func rankedRootName(root *algebraicRoot, pretty bool) string {
	if root.kind != realExtensionAlgebraic {
		if pretty && root.prettyName != "" {
			return root.prettyName
		}
		if root.name != "" {
			return root.name
		}
	}
	if pretty {
		return "&alpha;<sub>" + strconv.Itoa(root.index) + "</sub>"
	}
	return "r!" + strconv.Itoa(root.index)
}

func nonzeroRankedCoefficients(value *realValue) int {
	if value == nil || value.isRational {
		return 0
	}
	count := 0
	for _, coefficient := range value.functionNumerator.coefficients {
		if coefficient != nil {
			count++
		}
	}
	return count
}

func rankedCoefficientNeedsParentheses(value *realValue) bool {
	return value != nil && !value.isRational && nonzeroRankedCoefficients(value) > 1
}
