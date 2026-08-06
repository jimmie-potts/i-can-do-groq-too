package modelturnhttp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/modelturn"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

type closeTrackingReader struct {
	*bytes.Reader
	closes int
}

func (reader *closeTrackingReader) Close() error {
	reader.closes++
	return nil
}

type readFailureTrap struct{}

func (readFailureTrap) Error() string {
	panic("request read error was formatted")
}

type failingRequestReader struct {
	reads int
}

func (reader *failingRequestReader) Read([]byte) (int, error) {
	reader.reads++
	return 0, readFailureTrap{}
}

type ordinaryErrorTrap struct {
	formatted int
}

func (trap *ordinaryErrorTrap) Error() string {
	trap.formatted++
	panic("ordinary executor error was formatted")
}

func TestHandlerPresentsUncorrelatedAdmissionFailures(t *testing.T) {
	tests := []struct {
		name string
		body io.Reader
	}{
		{name: "malformed", body: strings.NewReader(`{"request_id":"request-010",`)},
		{name: "unsafe identifier", body: strings.NewReader(`{"request_id":"-unsafe"}`)},
		{name: "read failure", body: &failingRequestReader{}},
		{name: "oversized", body: bytes.NewReader(make([]byte, (8<<20)+1))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := newTestFake(t)
			handler := newTestHandler(t, upstream)
			request := httptest.NewRequest(http.MethodPost, modelTurnPath, test.body)
			request.Header.Set("Content-Type", jsonMediaType)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertHTTPResponse(
				t,
				response,
				http.StatusBadRequest,
				textMediaType,
				"invalid request\n",
				"",
			)
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("zero-dispatch fake was not complete: %v", err)
			}
		})
	}
}

func TestHandlerPresentsCorrelatedAdmissionFailuresWithoutParsingTheirBodies(t *testing.T) {
	tests := []struct {
		name        string
		change      func(*testRequestDocument)
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{
			name: "wrong version",
			change: func(document *testRequestDocument) {
				document.Version = "v2"
			},
			wantStatus:  http.StatusBadRequest,
			wantCode:    invalidRequestCode,
			wantMessage: "The request is invalid.",
		},
		{
			name: "empty conversation",
			change: func(document *testRequestDocument) {
				document.Conversation = []testRequestMessage{}
			},
			wantStatus:  http.StatusBadRequest,
			wantCode:    invalidRequestCode,
			wantMessage: "The request is invalid.",
		},
		{
			name: "unknown alias",
			change: func(document *testRequestDocument) {
				document.ModelAlias = "unknown-model"
			},
			wantStatus:  http.StatusBadRequest,
			wantCode:    invalidRequestCode,
			wantMessage: "The request is invalid.",
		},
		{
			name: "capability precedes unknown alias",
			change: func(document *testRequestDocument) {
				document.ModelAlias = "unknown-model"
				document.RequiredCapabilities = []string{"tool_calls"}
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantCode:    unsupportedCapabilityCode,
			wantMessage: "The required capability is not supported.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := defaultTestDocument()
			test.change(&document)
			upstream := newTestFake(t)
			handler := newTestHandler(t, upstream)
			request := httptest.NewRequest(
				http.MethodPost,
				modelTurnPath,
				bytes.NewReader(marshalTestDocument(t, document)),
			)
			request.Header.Set("Content-Type", jsonMediaType)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertHTTPResponse(
				t,
				response,
				test.wantStatus,
				jsonMediaType,
				expectedAdmissionFailure(
					document.RequestID,
					test.wantCode,
					test.wantMessage,
				),
				"",
			)
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("zero-dispatch fake was not complete: %v", err)
			}
		})
	}
}

func TestHandlerPresentsInvalidProviderAlternativeAsCorrelatedInternalFailure(t *testing.T) {
	document := defaultTestDocument()
	invocations := 0
	invoker := testInvokerFunc(func(
		context.Context,
		provider.Request,
	) (provider.Result, error) {
		invocations++
		return provider.Result{}, errors.New("raw provider error")
	})
	handler := newTestHandler(t, invoker)
	request := httptest.NewRequest(
		http.MethodPost,
		modelTurnPath,
		bytes.NewReader(marshalTestDocument(t, document)),
	)
	request.Header.Set("Content-Type", jsonMediaType)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertHTTPResponse(
		t,
		response,
		http.StatusInternalServerError,
		jsonMediaType,
		expectedAdmissionFailure(
			document.RequestID,
			internalErrorCode,
			"The request could not be processed.",
		),
		"",
	)
	if invocations != 1 {
		t.Fatalf("Invoke() calls = %d, want 1", invocations)
	}
}

func TestHandlerFallsBackSafelyForOrdinaryExecutorAndInvalidOutcomeState(t *testing.T) {
	t.Run("ordinary executor error", func(t *testing.T) {
		upstream := newTestFake(t)
		handler := newTestHandler(t, upstream)
		request := httptest.NewRequest(http.MethodPost, modelTurnPath, http.NoBody)
		request.Header.Set("Content-Type", jsonMediaType)
		request.Body = nil
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		assertHTTPResponse(
			t,
			response,
			http.StatusInternalServerError,
			textMediaType,
			internalServerErrorBody,
			"",
		)
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("zero-dispatch fake was not complete: %v", err)
		}
	})

	t.Run("invalid zero outcome", func(t *testing.T) {
		response := prepareOutcome(modelturn.Outcome{}, nil)
		if response.status != http.StatusInternalServerError ||
			response.mediaType != textMediaType ||
			string(response.body) != internalServerErrorBody {
			t.Fatalf("prepareOutcome(zero) = %+v, want fixed internal response", response)
		}
	})

	t.Run("ordinary error is not formatted", func(t *testing.T) {
		trap := &ordinaryErrorTrap{}
		response := prepareOutcome(modelturn.Outcome{}, trap)
		if response.status != http.StatusInternalServerError || trap.formatted != 0 {
			t.Fatalf(
				"prepareOutcome(error) = status %d, formatted %d; want 500 and zero formatting",
				response.status,
				trap.formatted,
			)
		}
	})
}

func TestHandlerPassesBodyDirectlyWithoutClosingOrInspectingContentLength(t *testing.T) {
	document := defaultTestDocument()
	document.ModelAlias = "unknown-model"
	raw := marshalTestDocument(t, document)
	body := &closeTrackingReader{Reader: bytes.NewReader(raw)}
	upstream := newTestFake(t)
	handler := newTestHandler(t, upstream)
	request := httptest.NewRequest(http.MethodPost, modelTurnPath, body)
	request.Header.Set("Content-Type", jsonMediaType)
	request.ContentLength = 1
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertHTTPResponse(
		t,
		response,
		http.StatusBadRequest,
		jsonMediaType,
		expectedAdmissionFailure(
			document.RequestID,
			invalidRequestCode,
			"The request is invalid.",
		),
		"",
	)
	if body.Len() != 0 {
		t.Fatalf("request body has %d unread bytes, want complete executor consumption", body.Len())
	}
	if body.closes != 0 {
		t.Fatalf("request body Close() calls = %d, want server-owned closure", body.closes)
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("zero-dispatch fake was not complete: %v", err)
	}
}
