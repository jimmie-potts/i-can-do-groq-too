package modelturn

import (
	"context"
	"errors"
	"io"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

// Executor owns admission and one synchronous provider invocation for model-turn v1. It neither
// closes the body nor adds HTTP, retry, panic recovery, or concurrency policy; Invoker rules apply.
type Executor struct {
	invoker provider.Invoker
}

// NewExecutor constructs a model-turn executor around one provider-neutral invoker.
func NewExecutor(invoker provider.Invoker) (*Executor, error) {
	if invoker == nil {
		return nil, errors.New("model-turn invoker is required")
	}
	return &Executor{invoker: invoker}, nil
}

// Execute admits one complete request body and invokes the configured provider at most once.
func (executor *Executor) Execute(ctx context.Context, body io.Reader) (Outcome, error) {
	if ctx == nil {
		return Outcome{}, errors.New("model-turn context is required")
	}
	if body == nil {
		return Outcome{}, errors.New("model-turn body reader is required")
	}

	raw, err := readRequestBody(body)
	if err != nil || !validateStrictDocument(raw) {
		return newUncorrelatedFailure(), nil
	}
	root, ok := decodeObject(raw)
	if !ok {
		return newUncorrelatedFailure(), nil
	}
	requestID, ok := safeRequestID(root)
	if !ok {
		return newUncorrelatedFailure(), nil
	}
	request, ok := decodeWireRequest(root)
	if !ok {
		return newInvalidRequestFailure(requestID), nil
	}
	if len(request.requiredCapabilities) != 0 {
		return newUnsupportedCapabilityFailure(requestID), nil
	}
	if request.modelAlias != supportedAlias {
		return newInvalidRequestFailure(requestID), nil
	}
	return executor.executeAdmitted(ctx, request), nil
}

func (executor *Executor) executeAdmitted(ctx context.Context, request wireRequest) Outcome {
	providerRequest, err := mapProviderRequest(request)
	if err != nil {
		return newInternalFailure(request.requestID)
	}

	result, invokeErr := executor.invoker.Invoke(ctx, providerRequest)
	if provider.ValidateInvocation(ctx, result, invokeErr) != nil {
		return newInternalFailure(request.requestID)
	}
	return newProviderOutcome(request.requestID, result, invokeErr)
}
