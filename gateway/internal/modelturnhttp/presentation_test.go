package modelturnhttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider/fake"
)

type marshalErrorTrap struct {
	formatted int
}

func (trap *marshalErrorTrap) Error() string {
	trap.formatted++
	panic("marshal error was formatted")
}

func TestHandlerPresentsCompletedResultsAndUsagePresence(t *testing.T) {
	tests := []struct {
		name  string
		usage *provider.Usage
	}{
		{name: "usage absent"},
		{name: "usage observed zero", usage: &provider.Usage{}},
		{
			name:  "usage nonzero",
			usage: &provider.Usage{InputTokens: 13, OutputTokens: 21},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := defaultTestDocument()
			document.RequestID = "req.completed:010"
			output := "line one\n\"quoted\" café & <tag>"
			result, err := provider.NewResult(output, test.usage)
			if err != nil {
				t.Fatalf("provider.NewResult() returned an error: %v", err)
			}
			exchange, err := fake.ExpectResult(providerRequestForDocument(t, document), result)
			if err != nil {
				t.Fatalf("fake.ExpectResult() returned an error: %v", err)
			}
			upstream := newTestFake(t, exchange)
			response := serveTestDocument(t, newTestHandler(t, upstream), document)

			assertHTTPMetadata(t, response, http.StatusOK, jsonMediaType, "")
			want := map[string]any{
				"version":     modelTurnVersion,
				"kind":        completedKind,
				"request_id":  document.RequestID,
				"output_text": output,
			}
			if test.usage != nil {
				want["usage"] = expectedUsage(*test.usage)
			}
			assertJSONDocument(t, response.Body.Bytes(), want)
			assertCompactJSON(t, response.Body.Bytes())
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestHandlerPresentsEveryNormalizedProviderFailure(t *testing.T) {
	tests := []struct {
		name      string
		code      provider.FailureCode
		status    int
		message   string
		retryable bool
		usage     *provider.Usage
	}{
		{
			name:    "authentication failed",
			code:    provider.FailureAuthenticationFailed,
			status:  http.StatusBadGateway,
			message: "FastGate could not authenticate to the upstream.",
		},
		{
			name:      "rate limited",
			code:      provider.FailureRateLimited,
			status:    http.StatusTooManyRequests,
			message:   "The upstream rate limit was reached.",
			retryable: true,
			usage:     &provider.Usage{},
		},
		{
			name:    "request rejected",
			code:    provider.FailureRequestRejected,
			status:  http.StatusUnprocessableEntity,
			message: "The upstream rejected the request.",
			usage:   &provider.Usage{InputTokens: 8, OutputTokens: 1},
		},
		{
			name:      "unavailable",
			code:      provider.FailureUnavailable,
			status:    http.StatusServiceUnavailable,
			message:   "The upstream is unavailable.",
			retryable: true,
		},
		{
			name:    "invalid response",
			code:    provider.FailureInvalidResponse,
			status:  http.StatusBadGateway,
			message: "The upstream returned an invalid response.",
			usage:   &provider.Usage{},
		},
		{
			name:    "unsupported upstream output",
			code:    provider.FailureUnsupportedUpstreamOutput,
			status:  http.StatusBadGateway,
			message: "The upstream returned output that model-turn v1 does not support.",
			usage:   &provider.Usage{InputTokens: 47, OutputTokens: 3},
		},
		{
			name:      "internal",
			code:      provider.FailureInternal,
			status:    http.StatusInternalServerError,
			message:   "The request could not be processed.",
			retryable: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := defaultTestDocument()
			document.RequestID = "req.failure:010"
			failure, err := provider.NewFailure(test.code, test.retryable, test.usage)
			if err != nil {
				t.Fatalf("provider.NewFailure() returned an error: %v", err)
			}
			exchange, err := fake.ExpectFailure(providerRequestForDocument(t, document), failure)
			if err != nil {
				t.Fatalf("fake.ExpectFailure() returned an error: %v", err)
			}
			upstream := newTestFake(t, exchange)
			response := serveTestDocument(t, newTestHandler(t, upstream), document)

			assertHTTPMetadata(t, response, test.status, jsonMediaType, "")
			want := map[string]any{
				"version":    modelTurnVersion,
				"kind":       failedKind,
				"request_id": document.RequestID,
				"error": map[string]any{
					"code":      string(test.code),
					"message":   test.message,
					"retryable": test.retryable,
				},
			}
			if test.usage != nil {
				want["usage"] = expectedUsage(*test.usage)
			}
			assertJSONDocument(t, response.Body.Bytes(), want)
			assertCompactJSON(t, response.Body.Bytes())
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestHandlerResponsesMatchCanonicalModelTurnFixtures(t *testing.T) {
	t.Run("minimal completed", func(t *testing.T) {
		document := defaultTestDocument()
		document.RequestID = "req-1"
		result, err := provider.NewResult("One model turn has one bounded outcome.", nil)
		if err != nil {
			t.Fatalf("provider.NewResult() returned an error: %v", err)
		}
		exchange, err := fake.ExpectResult(providerRequestForDocument(t, document), result)
		if err != nil {
			t.Fatalf("fake.ExpectResult() returned an error: %v", err)
		}
		upstream := newTestFake(t, exchange)
		response := serveTestDocument(t, newTestHandler(t, upstream), document)

		assertJSONMatchesFixture(t, response.Body.Bytes(), "minimal-completed.json")
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("VerifyComplete() returned an error: %v", err)
		}
	})

	t.Run("unsupported upstream output", func(t *testing.T) {
		document := defaultTestDocument()
		document.RequestID = "req-upstream-tool-output"
		usage := &provider.Usage{InputTokens: 47, OutputTokens: 3}
		failure, err := provider.NewFailure(
			provider.FailureUnsupportedUpstreamOutput,
			false,
			usage,
		)
		if err != nil {
			t.Fatalf("provider.NewFailure() returned an error: %v", err)
		}
		exchange, err := fake.ExpectFailure(providerRequestForDocument(t, document), failure)
		if err != nil {
			t.Fatalf("fake.ExpectFailure() returned an error: %v", err)
		}
		upstream := newTestFake(t, exchange)
		response := serveTestDocument(t, newTestHandler(t, upstream), document)

		assertJSONMatchesFixture(
			t,
			response.Body.Bytes(),
			"failed-unsupported-upstream-output.json",
		)
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("VerifyComplete() returned an error: %v", err)
		}
	})
}

func TestMarshalFailureFallsBackBeforeResponseCommitWithoutFormatting(t *testing.T) {
	trap := &marshalErrorTrap{}
	response := prepareJSONResponse(
		http.StatusOK,
		[]byte("UNSAFE-MARSHAL-BODY"),
		trap,
	)

	if response.status != http.StatusInternalServerError ||
		response.mediaType != textMediaType ||
		string(response.body) != internalServerErrorBody {
		t.Fatalf("prepareJSONResponse(error) = %+v, want fixed internal response", response)
	}
	if trap.formatted != 0 {
		t.Fatalf("marshal error formatting calls = %d, want 0", trap.formatted)
	}
}

func serveTestDocument(
	t *testing.T,
	handler http.Handler,
	document testRequestDocument,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(
		http.MethodPost,
		modelTurnPath,
		bytes.NewReader(marshalTestDocument(t, document)),
	)
	request.Header.Set("Content-Type", jsonMediaType)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func expectedUsage(usage provider.Usage) map[string]any {
	return map[string]any{
		"input_tokens":  json.Number(strconv.FormatInt(usage.InputTokens, 10)),
		"output_tokens": json.Number(strconv.FormatInt(usage.OutputTokens, 10)),
	}
}

func assertJSONDocument(t *testing.T, raw []byte, want map[string]any) {
	t.Helper()
	got := decodeJSONDocument(t, raw)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON document = %#v, want %#v", got, want)
	}
}

func assertCompactJSON(t *testing.T, raw []byte) {
	t.Helper()
	if len(raw) == 0 || raw[len(raw)-1] == '\n' {
		t.Fatalf("JSON body has invalid final framing: %q", raw)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		t.Fatalf("json.Compact() returned an error: %v", err)
	}
	if !bytes.Equal(compact.Bytes(), raw) {
		t.Fatalf("JSON body is not compact: %q", raw)
	}
}

func assertJSONMatchesFixture(t *testing.T, got []byte, fixtureName string) {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join(testContractRoot(t), "fixtures", "valid", fixtureName))
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixtureName, err)
	}
	gotDocument := decodeJSONDocument(t, got)
	wantDocument := decodeJSONDocument(t, fixture)
	if !reflect.DeepEqual(gotDocument, wantDocument) {
		t.Fatalf("response document = %#v, want fixture %#v", gotDocument, wantDocument)
	}
}

func decodeJSONDocument(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var document map[string]any
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode JSON document: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("JSON document has invalid trailing content: %v", err)
	}
	return document
}

func testContractRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test filename")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "contracts", "model-turn", "v1")
}
