package api

import "strconv"

func (log *Z3LogWriter) Call(id uint32) error {
	return log.writeRecord("C " + strconv.FormatUint(uint64(id), 10) + "\n")
}

// Result, Out, and ArrayOut are SetR, SetO, and SetAO from generated
// api_log_macros.h. They preserve the native pointer identities used as
// replay heap keys.
func (log *Z3LogWriter) Result(pointer uint64) error {
	return log.writeRecord("= " + z3LogPointer(pointer) + "\n")
}

func (log *Z3LogWriter) Out(pointer uint64, position uint32) error {
	return log.writeRecord("* " + z3LogPointer(pointer) + " " + strconv.FormatUint(uint64(position), 10) + "\n")
}

func (log *Z3LogWriter) ArrayOut(pointer uint64, position, index uint32) error {
	return log.writeRecord("@ " + z3LogPointer(pointer) + " " + strconv.FormatUint(uint64(position), 10) + " " + strconv.FormatUint(uint64(index), 10) + "\n")
}

func (log *Z3LogWriter) Message(message string) error {
	return log.writeRecord("M \"" + z3LogEscape(message) + "\"\n")
}
