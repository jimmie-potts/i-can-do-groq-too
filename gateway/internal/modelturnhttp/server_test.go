package modelturnhttp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider/fake"
)

const (
	testClientTimeout    = 2 * time.Second
	maximumTestBodyBytes = 1 << 20
)

type contextTerminationInvoker struct {
	calls atomic.Int32
}

func (invoker *contextTerminationInvoker) Invoke(
	ctx context.Context,
	_ provider.Request,
) (provider.Result, error) {
	invoker.calls.Add(1)
	return provider.Result{}, ctx.Err()
}

type countingResponseWriter struct {
	header           http.Header
	headerCalls      int
	writeHeaderCalls int
	writeCalls       int
	status           int
	body             []byte
	writeErr         error
}

func newCountingResponseWriter(writeErr error) *countingResponseWriter {
	return &countingResponseWriter{header: make(http.Header), writeErr: writeErr}
}

func (writer *countingResponseWriter) Header() http.Header {
	writer.headerCalls++
	return writer.header
}

func (writer *countingResponseWriter) WriteHeader(status int) {
	writer.writeHeaderCalls++
	writer.status = status
}

func (writer *countingResponseWriter) Write(body []byte) (int, error) {
	writer.writeCalls++
	writer.body = append([]byte(nil), body...)
	if writer.writeErr != nil {
		return 0, writer.writeErr
	}
	return len(body), nil
}

type responseWriteErrorTrap struct {
	formatted int
}

func (trap *responseWriteErrorTrap) Error() string {
	trap.formatted++
	panic("response write error was formatted")
}

func TestHandlerAbortsMatchingContextTerminationBeforeWriterAccess(t *testing.T) {
	tests := []struct {
		name   string
		newCtx func() (context.Context, context.CancelFunc)
	}{
		{
			name: "canceled",
			newCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
		},
		{
			name: "deadline exceeded",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(1, 0))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.newCtx()
			defer cancel()
			invoker := &contextTerminationInvoker{}
			handler := newTestHandler(t, invoker)
			document := defaultTestDocument()
			request := httptest.NewRequestWithContext(
				ctx,
				http.MethodPost,
				modelTurnPath,
				bytes.NewReader(marshalTestDocument(t, document)),
			)
			request.Header.Set("Content-Type", jsonMediaType)
			writer := newCountingResponseWriter(nil)

			panicValue, panicked := capturePanic(func() {
				handler.ServeHTTP(writer, request)
			})

			if !panicked || panicValue != http.ErrAbortHandler {
				t.Fatalf("ServeHTTP() panic = (%v, %t), want exact http.ErrAbortHandler", panicValue, panicked)
			}
			if invoker.calls.Load() != 1 {
				t.Fatalf("Invoke() calls = %d, want 1", invoker.calls.Load())
			}
			if writer.headerCalls != 0 || writer.writeHeaderCalls != 0 || writer.writeCalls != 0 {
				t.Fatalf(
					"writer calls = (Header %d, WriteHeader %d, Write %d), want all zero",
					writer.headerCalls,
					writer.writeHeaderCalls,
					writer.writeCalls,
				)
			}
		})
	}
}

func TestRealServerAbortReachesInvokerAndDoesNotBecomeImplicitOK(t *testing.T) {
	invoker := &contextTerminationInvoker{}
	handler := newTestHandler(t, invoker)
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) {
		serverCtx, cancel := context.WithCancel(request.Context())
		cancel()
		handler.ServeHTTP(responseWriter, request.WithContext(serverCtx))
	}))
	defer server.Close()

	client, transport := newLoopbackClient()
	document := defaultTestDocument()
	requestCtx, cancelRequest := context.WithTimeout(context.Background(), testClientTimeout)
	defer cancelRequest()
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		server.URL+modelTurnPath,
		bytes.NewReader(marshalTestDocument(t, document)),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
	}
	request.Header.Set("Content-Type", jsonMediaType)

	response, requestErr := client.Do(request)
	transport.CloseIdleConnections()

	if requestCtx.Err() != nil {
		t.Fatalf("request context ended before server abort was observed: %v", requestCtx.Err())
	}
	if requestErr == nil || response != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("client.Do() = (response %v, error %v), want no HTTP response", response, requestErr)
	}
	if invoker.calls.Load() != 1 {
		t.Fatalf("Invoke() calls = %d, want reached exactly once", invoker.calls.Load())
	}
}

func TestHandlerDoesNotRetryAfterResponseWriteFailure(t *testing.T) {
	document := defaultTestDocument()
	result, err := provider.NewResult("prepared before write", nil)
	if err != nil {
		t.Fatalf("provider.NewResult() returned an error: %v", err)
	}
	exchange, err := fake.ExpectResult(providerRequestForDocument(t, document), result)
	if err != nil {
		t.Fatalf("fake.ExpectResult() returned an error: %v", err)
	}
	upstream := newTestFake(t, exchange)
	handler := newTestHandler(t, upstream)
	request := httptest.NewRequest(
		http.MethodPost,
		modelTurnPath,
		bytes.NewReader(marshalTestDocument(t, document)),
	)
	request.Header.Set("Content-Type", jsonMediaType)
	writeTrap := &responseWriteErrorTrap{}
	writer := newCountingResponseWriter(writeTrap)

	handler.ServeHTTP(writer, request)

	if writer.headerCalls != 1 || writer.writeHeaderCalls != 1 || writer.writeCalls != 1 {
		t.Fatalf(
			"writer calls = (Header %d, WriteHeader %d, Write %d), want exactly one each",
			writer.headerCalls,
			writer.writeHeaderCalls,
			writer.writeCalls,
		)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("written status = %d, want %d", writer.status, http.StatusOK)
	}
	if writeTrap.formatted != 0 {
		t.Fatalf("write error formatting calls = %d, want 0", writeTrap.formatted)
	}
	assertJSONDocument(t, writer.body, map[string]any{
		"version":     modelTurnVersion,
		"kind":        completedKind,
		"request_id":  document.RequestID,
		"output_text": result.OutputText(),
	})
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() returned an error: %v", err)
	}
}

func TestRealServerSuppressesAuthoredHEADBody(t *testing.T) {
	upstream := newTestFake(t)
	handler := newTestHandler(t, upstream)
	server := httptest.NewServer(handler)
	defer server.Close()
	client, transport := newLoopbackClient()

	requestCtx, cancelRequest := context.WithTimeout(context.Background(), testClientTimeout)
	defer cancelRequest()
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodHead,
		server.URL+modelTurnPath,
		nil,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("HEAD request returned an error: %v", err)
	}
	body := readAndCloseResponse(t, response)
	transport.CloseIdleConnections()

	assertServerResponseHeaders(
		t,
		response,
		http.StatusMethodNotAllowed,
		textMediaType,
		http.MethodPost,
	)
	if len(body) != 0 {
		t.Fatalf("HEAD wire body = %q, want empty", body)
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("zero-dispatch fake was not complete: %v", err)
	}
}

func TestRealServerRejectsDeclaredRepresentationTrailersBeforeDispatch(t *testing.T) {
	tests := []struct {
		name        string
		trailerName string
	}{
		{name: "content encoding", trailerName: "Content-Encoding"},
		{name: "second content type", trailerName: "Content-Type"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			handler := newTestHandler(t, testInvokerFunc(func(
				context.Context,
				provider.Request,
			) (provider.Result, error) {
				calls.Add(1)
				return provider.Result{}, nil
			}))
			server := httptest.NewServer(handler)
			defer server.Close()
			client, transport := newLoopbackClient()

			requestCtx, cancelRequest := context.WithTimeout(context.Background(), testClientTimeout)
			defer cancelRequest()
			request, err := http.NewRequestWithContext(
				requestCtx,
				http.MethodPost,
				server.URL+modelTurnPath,
				bytes.NewReader(marshalTestDocument(t, defaultTestDocument())),
			)
			if err != nil {
				t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
			}
			request.Header.Set("Content-Type", jsonMediaType)
			request.ContentLength = -1
			request.Trailer = http.Header{test.trailerName: {"declared-value"}}

			response, err := client.Do(request)
			if err != nil {
				t.Fatalf("request with declared trailer returned an error: %v", err)
			}
			body := readAndCloseResponse(t, response)
			transport.CloseIdleConnections()

			assertServerResponseHeaders(
				t,
				response,
				http.StatusUnsupportedMediaType,
				textMediaType,
				"",
			)
			if got := string(body); got != unsupportedMediaTypeBody {
				t.Fatalf("body = %q, want %q", got, unsupportedMediaTypeBody)
			}
			if calls.Load() != 0 {
				t.Fatalf("Invoke() calls = %d, want 0", calls.Load())
			}
		})
	}
}

func TestSerialChunkedLoopbackModelTurnCleansUpAndCompletesFake(t *testing.T) {
	document := defaultTestDocument()
	document.RequestID = "req-loopback-010"
	usage := &provider.Usage{InputTokens: 5, OutputTokens: 8}
	result, err := provider.NewResult("one loopback completion", usage)
	if err != nil {
		t.Fatalf("provider.NewResult() returned an error: %v", err)
	}
	exchange, err := fake.ExpectResult(providerRequestForDocument(t, document), result)
	if err != nil {
		t.Fatalf("fake.ExpectResult() returned an error: %v", err)
	}
	upstream := newTestFake(t, exchange)
	handler := newTestHandler(t, upstream)
	server := httptest.NewServer(handler)
	defer server.Close()
	client, transport := newLoopbackClient()

	requestCtx, cancelRequest := context.WithTimeout(context.Background(), testClientTimeout)
	defer cancelRequest()
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		server.URL+modelTurnPath,
		bytes.NewReader(marshalTestDocument(t, document)),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() returned an error: %v", err)
	}
	request.Header.Set("Content-Type", jsonMediaType)
	request.ContentLength = -1
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("model-turn request returned an error: %v", err)
	}
	body := readAndCloseResponse(t, response)
	transport.CloseIdleConnections()

	assertServerResponseHeaders(t, response, http.StatusOK, jsonMediaType, "")
	assertJSONDocument(t, body, map[string]any{
		"version":     modelTurnVersion,
		"kind":        completedKind,
		"request_id":  document.RequestID,
		"output_text": result.OutputText(),
		"usage":       expectedUsage(*usage),
	})
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() after response cleanup returned an error: %v", err)
	}
}

func capturePanic(function func()) (panicValue any, panicked bool) {
	defer func() {
		panicValue = recover()
		panicked = panicValue != nil
	}()
	function()
	return nil, false
}

func newLoopbackClient() (*http.Client, *http.Transport) {
	transport := &http.Transport{Proxy: nil}
	return &http.Client{Transport: transport}, transport
}

func readAndCloseResponse(t *testing.T, response *http.Response) []byte {
	t.Helper()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maximumTestBodyBytes+1))
	closeErr := response.Body.Close()
	if readErr != nil {
		t.Fatalf("read response body: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close response body: %v", closeErr)
	}
	if len(body) > maximumTestBodyBytes {
		t.Fatalf("response body exceeded %d bytes", maximumTestBodyBytes)
	}
	return body
}

func assertServerResponseHeaders(
	t *testing.T,
	response *http.Response,
	wantStatus int,
	wantMediaType string,
	wantAllow string,
) {
	t.Helper()
	if response.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", response.StatusCode, wantStatus)
	}
	if got := response.Header.Get("Content-Type"); got != wantMediaType {
		t.Fatalf("Content-Type = %q, want %q", got, wantMediaType)
	}
	if got := response.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := response.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := response.Header.Get("Allow"); got != wantAllow {
		t.Fatalf("Allow = %q, want %q", got, wantAllow)
	}
}
