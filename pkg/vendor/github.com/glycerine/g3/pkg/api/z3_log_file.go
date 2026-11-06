package api

import (
	"os"
	"strconv"

	"github.com/glycerine/g3/pkg/util/version"
)

// OpenZ3Log creates a log and writes its version header. Go has no faithful
// equivalent of the C++ translation unit's __DATE__, so the default header
// contains the version tuple only.
func OpenZ3Log(filename string) (*Z3LogWriter, error) {
	return OpenZ3LogWithBuildDate(filename, "")
}

// OpenZ3LogWithBuildDate lets binary packagers retain the native header's
// build-date suffix without coupling the logger to a generated registry.
func OpenZ3LogWithBuildDate(filename, buildDate string) (*Z3LogWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	log, err := newOwnedZ3LogWriter(file, file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if err := log.writeVersion(buildDate); err != nil {
		_ = log.Close()
		return nil, err
	}
	return log, nil
}

func (log *Z3LogWriter) writeVersion(buildDate string) error {
	number := strconv.FormatUint(uint64(version.Major), 10) + "." +
		strconv.FormatUint(uint64(version.Minor), 10) + "." +
		strconv.FormatUint(uint64(version.Build), 10) + "." +
		strconv.FormatUint(uint64(version.Revision), 10)
	if buildDate != "" {
		number += " " + z3LogCString(buildDate)
	}
	return log.writeRecord("V \"" + z3LogEscape(number) + "\"\n")
}
