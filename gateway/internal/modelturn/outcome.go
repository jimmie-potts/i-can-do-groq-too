// Package modelturn admits one FastGate model-turn v1 request before provider dispatch.
package modelturn

import "github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"

const (
	uncorrelatedFailureBody = "invalid request\n"

	invalidRequestCode        = "invalid_request"
	unsupportedCapabilityCode = "unsupported_capability"
	internalErrorCode         = "internal_error"

	invalidRequestMessage        = "The request is invalid."
	unsupportedCapabilityMessage = "The required capability is not supported."
	internalErrorMessage         = "The request could not be processed."
)

type outcomeKind uint8

const (
	outcomeUncorrelatedFailure outcomeKind = iota + 1
	outcomeAdmissionFailure
	outcomeProvider
	outcomeInternalFailure
)

// Outcome is one closed result of model-turn admission and execution. Its zero value is invalid. A
// value returned with nil ordinary error contains exactly one uncorrelated failure, correlated
// admission failure, validated provider outcome, or correlated internal failure.
type Outcome struct {
	kind          outcomeKind
	requestID     string
	failureCode   string
	result        provider.Result
	providerError error
}

// RequestID returns the safely admitted correlation ID when the outcome is correlated.
func (outcome Outcome) RequestID() (string, bool) {
	switch outcome.kind {
	case outcomeAdmissionFailure, outcomeProvider, outcomeInternalFailure:
		return outcome.requestID, true
	default:
		return "", false
	}
}

// FailureBody returns a new copy of the canonical failure payload when this is a failure outcome.
func (outcome Outcome) FailureBody() ([]byte, bool) {
	switch outcome.kind {
	case outcomeUncorrelatedFailure:
		return []byte(uncorrelatedFailureBody), true
	case outcomeAdmissionFailure, outcomeInternalFailure:
		return correlatedFailureBody(outcome.requestID, outcome.failureCode), true
	default:
		return nil, false
	}
}

// FailureCode returns the model-turn error code for a correlated failure outcome.
func (outcome Outcome) FailureCode() (string, bool) {
	switch outcome.kind {
	case outcomeAdmissionFailure, outcomeInternalFailure:
		return outcome.failureCode, true
	default:
		return "", false
	}
}

// ProviderOutcome returns the exact validated provider result/error alternative.
func (outcome Outcome) ProviderOutcome() (provider.Result, error, bool) {
	if outcome.kind != outcomeProvider {
		return provider.Result{}, nil, false
	}
	return outcome.result, outcome.providerError, true
}

func newUncorrelatedFailure() Outcome { return Outcome{kind: outcomeUncorrelatedFailure} }

func newInvalidRequestFailure(requestID string) Outcome {
	return newFailure(outcomeAdmissionFailure, requestID, invalidRequestCode)
}

func newUnsupportedCapabilityFailure(requestID string) Outcome {
	return newFailure(outcomeAdmissionFailure, requestID, unsupportedCapabilityCode)
}

func newInternalFailure(requestID string) Outcome {
	return newFailure(outcomeInternalFailure, requestID, internalErrorCode)
}

func newFailure(kind outcomeKind, requestID string, code string) Outcome {
	return Outcome{
		kind:        kind,
		requestID:   requestID,
		failureCode: code,
	}
}

func newProviderOutcome(requestID string, result provider.Result, err error) Outcome {
	return Outcome{
		kind:          outcomeProvider,
		requestID:     requestID,
		result:        result,
		providerError: err,
	}
}

func correlatedFailureBody(requestID string, code string) []byte {
	message := internalErrorMessage
	switch code {
	case invalidRequestCode:
		message = invalidRequestMessage
	case unsupportedCapabilityCode:
		message = unsupportedCapabilityMessage
	}
	// requestID has passed the ASCII identifier rule, while code and message are fixed constants.
	// None can require JSON escaping, so concatenation keeps the exact compact field order visible.
	return []byte(
		`{"version":"v1","kind":"model_turn.failed","request_id":"` +
			requestID +
			`","error":{"code":"` +
			code +
			`","message":"` +
			message +
			`","retryable":false}}`,
	)
}
