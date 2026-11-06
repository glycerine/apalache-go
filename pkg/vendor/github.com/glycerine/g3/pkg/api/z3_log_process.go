package api

// Source process-global facade: src/api/api_log.cpp. Porting is currently
// single-goroutine, so this deliberately mirrors the source global without a
// synchronization layer.

var processZ3Log *Z3LogWriter
var processZ3LogInAPI bool

func enterZ3APILog() *Z3LogWriter {
	if processZ3Log == nil || processZ3LogInAPI {
		return nil
	}
	processZ3LogInAPI = true
	return processZ3Log
}
func leaveZ3APILog(log *Z3LogWriter) {
	if log != nil {
		processZ3LogInAPI = false
	}
}

func Z3OpenLog(filename string) (bool, error) {
	return Z3OpenLogWithBuildDate(filename, "")
}

func Z3OpenLogWithBuildDate(filename, buildDate string) (bool, error) {
	_ = Z3CloseLog()
	log, err := OpenZ3LogWithBuildDate(filename, buildDate)
	if err != nil {
		return false, err
	}
	processZ3Log = log
	return true, nil
}

func Z3AppendLog(message string) error {
	if processZ3Log == nil {
		return nil
	}
	return processZ3Log.Message(message)
}

func Z3CloseLog() error {
	if processZ3Log == nil {
		return nil
	}
	log := processZ3Log
	processZ3Log = nil
	return log.Close()
}
