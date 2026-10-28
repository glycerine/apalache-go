package apalache

// CheckerException is the root of failures tied to a problematic TLA+
// expression. NullExpr may be used when no concrete expression is available.
type CheckerException struct {
	message   string
	causeExpr Expr
}

func NewCheckerException(message string, cause Expr) *CheckerException {
	if cause == nil {
		cause = NullExpr()
	}
	return &CheckerException{message: message, causeExpr: cause}
}
func (err *CheckerException) Error() string   { return err.message }
func (err *CheckerException) CauseExpr() Expr { return err.causeExpr }

type RewriterException struct{ *CheckerException }

func NewRewriterException(message string, cause Expr) *RewriterException {
	return &RewriterException{NewCheckerException(message, cause)}
}
func (err *RewriterException) Unwrap() error { return err.CheckerException }

type RewriterKnownLimitationError struct{ *CheckerException }

func NewRewriterKnownLimitationError(message string, cause Expr) *RewriterKnownLimitationError {
	return &RewriterKnownLimitationError{NewCheckerException(message, cause)}
}
func (err *RewriterKnownLimitationError) Unwrap() error { return err.CheckerException }

type NoRuleException struct{ *RewriterException }

func NewNoRuleException(message string, cause Expr) *NoRuleException {
	return &NoRuleException{NewRewriterException(message, cause)}
}
func (err *NoRuleException) Unwrap() error { return err.RewriterException }

type SmtEncodingException struct{ *CheckerException }

func NewSmtEncodingException(message string, cause Expr) *SmtEncodingException {
	return &SmtEncodingException{NewCheckerException(message, cause)}
}
func (err *SmtEncodingException) Unwrap() error { return err.CheckerException }

type InvalidTlaExException struct{ *CheckerException }

func NewInvalidTlaExException(message string, cause Expr) *InvalidTlaExException {
	return &InvalidTlaExException{NewCheckerException(message, cause)}
}
func (err *InvalidTlaExException) Unwrap() error { return err.CheckerException }

type InternalCheckerError struct{ *CheckerException }

func NewInternalCheckerError(message string, cause Expr) *InternalCheckerError {
	return &InternalCheckerError{NewCheckerException(message, cause)}
}
func (err *InternalCheckerError) Unwrap() error { return err.CheckerException }
