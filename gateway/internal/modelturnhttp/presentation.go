package modelturnhttp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/modelturn"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

const (
	modelTurnVersion          = "v1"
	completedKind             = "model_turn.completed"
	failedKind                = "model_turn.failed"
	internalServerErrorBody   = "internal server error\n"
	invalidRequestCode        = "invalid_request"
	unsupportedCapabilityCode = "unsupported_capability"
	internalErrorCode         = "internal_error"
)

type preparedResponse struct {
	status      int
	mediaType   string
	body        []byte
	allowMethod string
}

type usageResponse struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type completedResponse struct {
	Version    string         `json:"version"`
	Kind       string         `json:"kind"`
	RequestID  string         `json:"request_id"`
	OutputText string         `json:"output_text"`
	Usage      *usageResponse `json:"usage,omitempty"`
}

type failedResponse struct {
	Version   string              `json:"version"`
	Kind      string              `json:"kind"`
	RequestID string              `json:"request_id"`
	Error     failedResponseError `json:"error"`
	Usage     *usageResponse      `json:"usage,omitempty"`
}

type failedResponseError struct {
	Code      provider.FailureCode `json:"code"`
	Message   string               `json:"message"`
	Retryable bool                 `json:"retryable"`
}

func prepareOutcome(outcome modelturn.Outcome, executeErr error) preparedResponse {
	if executeErr != nil {
		return prepareInternalResponse()
	}

	requestID, hasRequestID := outcome.RequestID()
	failureBody, hasFailureBody := outcome.FailureBody()
	failureCode, hasFailureCode := outcome.FailureCode()
	result, providerErr, hasProviderOutcome := outcome.ProviderOutcome()

	switch {
	case hasFailureBody && !hasRequestID && !hasFailureCode && !hasProviderOutcome:
		return prepareBytesResponse(http.StatusBadRequest, textMediaType, failureBody)
	case hasFailureBody && hasRequestID && hasFailureCode && !hasProviderOutcome:
		status, ok := admissionFailureStatus(failureCode)
		if !ok || requestID == "" {
			return prepareInternalResponse()
		}
		return prepareBytesResponse(status, jsonMediaType, failureBody)
	case !hasFailureBody && hasRequestID && !hasFailureCode && hasProviderOutcome:
		if requestID == "" {
			return prepareInternalResponse()
		}
		return prepareProviderOutcome(requestID, result, providerErr)
	default:
		return prepareInternalResponse()
	}
}

func admissionFailureStatus(code string) (int, bool) {
	switch code {
	case invalidRequestCode:
		return http.StatusBadRequest, true
	case unsupportedCapabilityCode:
		return http.StatusUnprocessableEntity, true
	case internalErrorCode:
		return http.StatusInternalServerError, true
	default:
		return 0, false
	}
}

func prepareProviderOutcome(requestID string, result provider.Result, providerErr error) preparedResponse {
	if providerErr == nil {
		return prepareCompletedResponse(requestID, result)
	}
	if providerErr == context.Canceled || providerErr == context.DeadlineExceeded {
		panic(http.ErrAbortHandler)
	}
	failure, ok := providerErr.(*provider.Failure)
	if !ok || failure == nil {
		return prepareInternalResponse()
	}
	return prepareProviderFailureResponse(requestID, failure)
}

func prepareCompletedResponse(requestID string, result provider.Result) preparedResponse {
	usage, hasUsage := result.Usage()
	document := completedResponse{
		Version:    modelTurnVersion,
		Kind:       completedKind,
		RequestID:  requestID,
		OutputText: result.OutputText(),
		Usage:      prepareUsage(usage, hasUsage),
	}
	body, err := json.Marshal(document)
	return prepareJSONResponse(http.StatusOK, body, err)
}

func prepareProviderFailureResponse(requestID string, failure *provider.Failure) preparedResponse {
	status, message, ok := providerFailurePresentation(failure.Code())
	if !ok {
		return prepareInternalResponse()
	}
	usage, hasUsage := failure.Usage()
	document := failedResponse{
		Version:   modelTurnVersion,
		Kind:      failedKind,
		RequestID: requestID,
		Error: failedResponseError{
			Code:      failure.Code(),
			Message:   message,
			Retryable: failure.Retryable(),
		},
		Usage: prepareUsage(usage, hasUsage),
	}
	body, err := json.Marshal(document)
	return prepareJSONResponse(status, body, err)
}

func providerFailurePresentation(code provider.FailureCode) (int, string, bool) {
	switch code {
	case provider.FailureAuthenticationFailed:
		return http.StatusBadGateway, "FastGate could not authenticate to the upstream.", true
	case provider.FailureRateLimited:
		return http.StatusTooManyRequests, "The upstream rate limit was reached.", true
	case provider.FailureRequestRejected:
		return http.StatusUnprocessableEntity, "The upstream rejected the request.", true
	case provider.FailureUnavailable:
		return http.StatusServiceUnavailable, "The upstream is unavailable.", true
	case provider.FailureInvalidResponse:
		return http.StatusBadGateway, "The upstream returned an invalid response.", true
	case provider.FailureUnsupportedUpstreamOutput:
		return http.StatusBadGateway,
			"The upstream returned output that model-turn v1 does not support.", true
	case provider.FailureInternal:
		return http.StatusInternalServerError, "The request could not be processed.", true
	default:
		return 0, "", false
	}
}

func prepareUsage(usage provider.Usage, present bool) *usageResponse {
	if !present {
		return nil
	}
	return &usageResponse{InputTokens: usage.InputTokens, OutputTokens: usage.OutputTokens}
}

func prepareJSONResponse(status int, body []byte, marshalErr error) preparedResponse {
	if marshalErr != nil {
		return prepareInternalResponse()
	}
	return prepareBytesResponse(status, jsonMediaType, body)
}

func prepareTextResponse(status int, body string, allowMethod string) preparedResponse {
	response := prepareBytesResponse(status, textMediaType, []byte(body))
	response.allowMethod = allowMethod
	return response
}

func prepareInternalResponse() preparedResponse {
	return prepareTextResponse(http.StatusInternalServerError, internalServerErrorBody, "")
}

func prepareBytesResponse(status int, mediaType string, body []byte) preparedResponse {
	return preparedResponse{status: status, mediaType: mediaType, body: body}
}
