package solver

// Source: src/solver/smt_logics.h; src/solver/smt_logics.cpp.

func logicIn(logic string, values ...string) bool {
	for _, value := range values {
		if logic == value {
			return true
		}
	}
	return false
}

func LogicIsAll(logic string) bool { return logic == "ALL" }
func LogicHasFD(logic string) bool { return logic == "QF_FD" }

func LogicHasRealsOnly(logic string) bool {
	return logicIn(logic, "QF_RDL", "QF_LRA", "UFLRA", "LRA", "RDL", "QF_NRA", "QF_UFNRA", "QF_UFLRA")
}

func LogicHasArithmetic(logic string) bool {
	return logicIn(logic,
		"QF_LRA", "QF_LIA", "QF_RDL", "QF_IDL", "QF_AUFLIA", "QF_ALIA", "QF_AUFLIRA", "QF_AUFNIA", "QF_AUFNIRA", "QF_ANIA", "QF_LIRA", "QF_UFLIA", "QF_UFLRA", "QF_UFIDL", "QF_UFRDL", "QF_NIA", "QF_NRA", "QF_NIRA", "QF_UFNRA", "QF_UFNIA", "QF_UFNIRA", "QF_BVRE", "ALIA", "AUFLIA", "AUFLIRA", "AUFNIA", "AUFNIRA", "UFLIA", "UFLRA", "UFNRA", "UFNIRA", "NIA", "NRA", "UFNIA", "LIA", "LRA", "UFIDL", "QF_FP", "QF_FPBV", "QF_BVFP", "QF_S", "ALL", "QF_FD", "HORN", "QF_FPLRA")
}

func LogicHasBitVector(logic string) bool {
	return logicIn(logic, "UFBV", "AUFBV", "ABV", "BV", "QF_BV", "QF_UFBV", "QF_ABV", "QF_AUFBV", "QF_BVRE", "QF_FPBV", "QF_BVFP", "ALL", "QF_FD", "HORN")
}

func LogicHasArray(logic string) bool {
	return logicIn(logic, "QF_AX", "QF_AUFLIA", "QF_ANIA", "QF_ALIA", "QF_AUFLIRA", "QF_AUFNIA", "QF_AUFNIRA", "ALIA", "AUFLIA", "AUFLIRA", "AUFNIA", "AUFNIRA", "AUFBV", "ABV", "ALL", "QF_ABV", "QF_AUFBV", "HORN")
}

func LogicHasSequence(logic string) bool { return logicIn(logic, "QF_BVRE", "QF_S", "ALL") }
func LogicHasString(logic string) bool   { return logicIn(logic, "QF_S", "ALL") }
func LogicHasFPA(logic string) bool {
	return logicIn(logic, "QF_FP", "QF_FPBV", "QF_BVFP", "QF_FPLRA", "ALL")
}
func LogicHasUF(logic string) bool       { return logicIn(logic, "QF_UF", "UF", "QF_DT") }
func LogicHasHorn(logic string) bool     { return logic == "HORN" }
func LogicHasPB(logic string) bool       { return logicIn(logic, "QF_FD", "ALL") }
func LogicHasDatatype(logic string) bool { return logicIn(logic, "QF_FD", "ALL", "QF_DT") }

func SupportedLogic(logic string) bool {
	return LogicHasUF(logic) || LogicIsAll(logic) || LogicHasFD(logic) || LogicHasArithmetic(logic) || LogicHasBitVector(logic) || LogicHasArray(logic) || LogicHasSequence(logic) || LogicHasString(logic) || LogicHasHorn(logic) || LogicHasFPA(logic)
}
