// Package modelturnhttp presents FastGate model-turn v1 outcomes over HTTP.
package modelturnhttp

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/modelturn"
)

const (
	modelTurnPath            = "/v1/model-turns"
	jsonMediaType            = "application/json"
	textMediaType            = "text/plain; charset=utf-8"
	notFoundBody             = "not found\n"
	methodNotAllowedBody     = "method not allowed\n"
	unsupportedMediaTypeBody = "unsupported media type\n"
)

type modelTurnHandler struct{ executor *modelturn.Executor }

// NewHandler constructs the model-turn v1 HTTP presentation boundary without mounting a server.
func NewHandler(executor *modelturn.Executor) (http.Handler, error) {
	if executor == nil {
		return nil, errors.New("model-turn HTTP executor is required")
	}
	return modelTurnHandler{executor: executor}, nil
}

func (handler modelTurnHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	response, rejected := transportRejection(request)
	if !rejected {
		outcome, err := handler.executor.Execute(request.Context(), request.Body)
		response = prepareOutcome(outcome, err)
	}
	writePreparedResponse(responseWriter, response)
}

func transportRejection(request *http.Request) (preparedResponse, bool) {
	if !hasExactTarget(request) {
		return prepareTextResponse(http.StatusNotFound, notFoundBody, ""), true
	}
	if request.Method != http.MethodPost {
		return prepareTextResponse(http.StatusMethodNotAllowed, methodNotAllowedBody, http.MethodPost), true
	}
	// Server requests expose declared trailer names before body reads, even with nil values.
	_, hasEncoding := headerFieldValues(request.Header, "Content-Encoding")
	_, hasEncodingTrailer := headerFieldValues(request.Trailer, "Content-Encoding")
	if hasEncoding || hasEncodingTrailer {
		return prepareTextResponse(http.StatusUnsupportedMediaType, unsupportedMediaTypeBody, ""), true
	}
	contentTypes, _ := headerFieldValues(request.Header, "Content-Type")
	_, hasContentTypeTrailer := headerFieldValues(request.Trailer, "Content-Type")
	if hasContentTypeTrailer ||
		len(contentTypes) != 1 || !acceptsJSONMediaType(contentTypes[0]) {
		return prepareTextResponse(http.StatusUnsupportedMediaType, unsupportedMediaTypeBody, ""), true
	}
	return preparedResponse{}, false
}

func hasExactTarget(request *http.Request) bool {
	if request == nil || request.RequestURI != modelTurnPath || request.URL == nil {
		return false
	}
	return request.URL.Path == modelTurnPath &&
		request.URL.RawPath == "" &&
		request.URL.RawQuery == "" &&
		!request.URL.ForceQuery &&
		request.URL.Scheme == "" &&
		request.URL.Host == "" &&
		request.URL.Opaque == ""
}

func headerFieldValues(header http.Header, name string) ([]string, bool) {
	var values []string
	present := false
	for fieldName, fieldValues := range header {
		if strings.EqualFold(fieldName, name) {
			present = true
			values = append(values, fieldValues...)
		}
	}
	return values, present
}

func acceptsJSONMediaType(value string) bool {
	semicolonCount := strings.Count(value, ";")
	if semicolonCount > 1 {
		return false
	}
	if semicolonCount == 1 {
		_, rawParameter, _ := strings.Cut(value, ";")
		rawName, _, hasValue := strings.Cut(rawParameter, "=")
		if !hasValue || !strings.EqualFold(strings.TrimSpace(rawName), "charset") {
			return false
		}
	}

	mediaType, parameters, err := mime.ParseMediaType(value)
	if err != nil || !strings.EqualFold(mediaType, jsonMediaType) {
		return false
	}
	if semicolonCount == 0 {
		return len(parameters) == 0
	}
	if len(parameters) != 1 {
		return false
	}
	for name, parameterValue := range parameters {
		return strings.EqualFold(name, "charset") && strings.EqualFold(parameterValue, "utf-8")
	}
	return false
}

func writePreparedResponse(responseWriter http.ResponseWriter, response preparedResponse) {
	headers := responseWriter.Header()
	headers.Set("Cache-Control", "no-store")
	headers.Set("Content-Type", response.mediaType)
	headers.Set("X-Content-Type-Options", "nosniff")
	if response.allowMethod != "" {
		headers.Set("Allow", response.allowMethod)
	}
	responseWriter.WriteHeader(response.status)
	_, _ = responseWriter.Write(response.body)
}
