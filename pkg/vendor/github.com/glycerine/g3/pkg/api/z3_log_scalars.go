package api

import "strconv"

func (log *Z3LogWriter) ResetArguments() error { return log.writeRecord("R\n") }

func (log *Z3LogWriter) Pointer(pointer uint64) error {
	return log.writeRecord("P " + z3LogPointer(pointer) + "\n")
}

func (log *Z3LogWriter) Int64(value int64) error {
	return log.writeRecord("I " + strconv.FormatInt(value, 10) + "\n")
}

func (log *Z3LogWriter) Uint64(value uint64) error {
	return log.writeRecord("U " + strconv.FormatUint(value, 10) + "\n")
}

func (log *Z3LogWriter) Double(value float64) error {
	// std::ostream's default precision is six significant digits.
	return log.writeRecord("D " + strconv.FormatFloat(value, 'g', 6, 64) + "\n")
}

func (log *Z3LogWriter) String(value string) error {
	return log.writeRecord("S \"" + z3LogEscape(value) + "\"\n")
}

func z3LogPointer(pointer uint64) string {
	if pointer == 0 {
		return "0"
	}
	return "0x" + strconv.FormatUint(pointer, 16)
}
