package modelturnhttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/modelturn"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider/fake"
)

type recordingRequestBody struct {
	reads  int
	closes int
}

func (body *recordingRequestBody) Read([]byte) (int, error) {
	body.reads++
	return 0, io.EOF
}

func (body *recordingRequestBody) Close() error {
	body.closes++
	return nil
}

func TestNewHandlerRequiresExecutor(t *testing.T) {
	handler, err := NewHandler(nil)
	if handler != nil {
		t.Fatal("NewHandler(nil) returned a nonnil handler")
	}
	if err == nil || err.Error() != "model-turn HTTP executor is required" {
		t.Fatalf("NewHandler(nil) error = %v, want exact fixed error", err)
	}
}

func TestExactTargetRequiresOneOriginFormSpelling(t *testing.T) {
	tests := []struct {
		name   string
		target string
		change func(*http.Request)
		want   bool
	}{
		{name: "exact", target: modelTurnPath, want: true},
		{name: "wrong case", target: "/v1/Model-turns"},
		{name: "trailing slash", target: modelTurnPath + "/"},
		{name: "encoded alias", target: "/v1/%6dodel-turns"},
		{name: "nonempty query", target: modelTurnPath + "?debug=true"},
		{name: "forced query", target: modelTurnPath + "?"},
		{name: "absolute form", target: "http://proxy.example" + modelTurnPath},
		{
			name:   "authority form",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.RequestURI = "proxy.example:443"
				request.URL.Host = "proxy.example:443"
			},
		},
		{
			name:   "parsed path differs from exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.Path = "/v1/different"
			},
		},
		{
			name:   "request URI differs from exact parsed URL",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.RequestURI = "/v1/different"
			},
		},
		{
			name:   "raw path on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.RawPath = "/v1/%6dodel-turns"
			},
		},
		{
			name:   "raw query on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.RawQuery = "debug=true"
			},
		},
		{
			name:   "forced query on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.ForceQuery = true
			},
		},
		{
			name:   "scheme on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.Scheme = "http"
			},
		},
		{
			name:   "host on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.Host = "proxy.example"
			},
		},
		{
			name:   "opaque form on exact request URI",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL.Opaque = "opaque-target"
			},
		},
		{
			name:   "missing parsed URL",
			target: modelTurnPath,
			change: func(request *http.Request) {
				request.URL = nil
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.target, http.NoBody)
			if test.change != nil {
				test.change(request)
			}

			if got := hasExactTarget(request); got != test.want {
				t.Fatalf("hasExactTarget() = %t, want %t", got, test.want)
			}
		})
	}

	if hasExactTarget(nil) {
		t.Fatal("hasExactTarget(nil) = true, want false")
	}
}

func TestHandlerRejectsTransportBeforeBodyReadOrProviderDispatch(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		target     string
		headers    http.Header
		trailers   http.Header
		wantStatus int
		wantBody   string
		wantAllow  string
	}{
		{
			name:   "target wins over method and representation",
			method: http.MethodGet,
			target: "/not-model-turns",
			headers: http.Header{
				"Content-Type": {"text/plain"},
			},
			trailers: http.Header{
				"Content-Encoding": nil,
			},
			wantStatus: http.StatusNotFound,
			wantBody:   notFoundBody,
		},
		{
			name:   "method wins over representation",
			method: http.MethodPut,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {"text/plain"},
			},
			trailers: http.Header{
				"Content-Encoding": nil,
			},
			wantStatus: http.StatusMethodNotAllowed,
			wantBody:   methodNotAllowedBody,
			wantAllow:  http.MethodPost,
		},
		{
			name:   "empty content encoding is present",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Encoding": {""},
				"Content-Type":     {jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "identity content encoding is present",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Encoding": {"identity"},
				"Content-Type":     {jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "noncanonical content encoding key is present",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"content-encoding": {"gzip"},
				"Content-Type":     {jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "content encoding declared as trailer",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType},
			},
			trailers: http.Header{
				"content-encoding": nil,
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:       "content type missing",
			method:     http.MethodPost,
			target:     modelTurnPath,
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "content type repeated",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType, jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "content type repeated across key casing",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType},
				"content-type": {jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "content type also declared as trailer",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType},
			},
			trailers: http.Header{
				"content-type": nil,
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "comma-combined content type",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType + ", " + jsonMediaType},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "duplicate charset parameter",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType + "; charset=utf-8; charset=utf-8"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "RFC 2231 charset parameter",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType + "; charset*=utf-8''utf-8"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "unsupported media type",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {"text/json"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "extra media parameter",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType + "; profile=v1"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
		{
			name:   "wrong charset",
			method: http.MethodPost,
			target: modelTurnPath,
			headers: http.Header{
				"Content-Type": {jsonMediaType + "; charset=iso-8859-1"},
			},
			wantStatus: http.StatusUnsupportedMediaType,
			wantBody:   unsupportedMediaTypeBody,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := newTestFake(t)
			handler := newTestHandler(t, upstream)
			body := &recordingRequestBody{}
			request := httptest.NewRequest(test.method, test.target, body)
			request.Header = test.headers.Clone()
			request.Trailer = test.trailers.Clone()
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			assertHTTPResponse(
				t,
				response,
				test.wantStatus,
				textMediaType,
				test.wantBody,
				test.wantAllow,
			)
			if body.reads != 0 {
				t.Fatalf("request body Read() calls = %d, want 0", body.reads)
			}
			if body.closes != 0 {
				t.Fatalf("request body Close() calls = %d, want server-owned closure", body.closes)
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("zero-dispatch fake was not complete: %v", err)
			}
		})
	}
}

func TestJSONMediaTypeProfile(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "bare", value: "application/json", want: true},
		{name: "bare case insensitive", value: "APPLICATION/JSON", want: true},
		{name: "charset", value: "application/json;charset=utf-8", want: true},
		{
			name:  "quoted charset with case and whitespace",
			value: `Application/Json ; CHARSET = "UTF-8"`,
			want:  true,
		},
		{name: "empty", value: ""},
		{name: "unsupported", value: "text/json"},
		{name: "missing parameter value", value: "application/json; charset"},
		{name: "missing parameter name", value: "application/json; =utf-8"},
		{name: "wrong parameter", value: "application/json; profile=v1"},
		{name: "wrong charset", value: "application/json; charset=ascii"},
		{
			name:  "duplicate identical charset",
			value: "application/json; charset=utf-8; charset=utf-8",
		},
		{name: "RFC 2231 extended", value: "application/json; charset*=utf-8''utf-8"},
		{name: "RFC 2231 continuation", value: "application/json; charset*0=utf-8"},
		{name: "extra parameter", value: "application/json; charset=utf-8; profile=v1"},
		{name: "quoted semicolon", value: `application/json; charset="utf;8"`},
		{name: "combined values", value: "application/json, application/json"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := acceptsJSONMediaType(test.value); got != test.want {
				t.Fatalf("acceptsJSONMediaType(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestDirectHEADResponseIncludesAuthoredMethodBody(t *testing.T) {
	upstream := newTestFake(t)
	handler := newTestHandler(t, upstream)
	body := &recordingRequestBody{}
	request := httptest.NewRequest(http.MethodHead, modelTurnPath, body)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	assertHTTPResponse(
		t,
		response,
		http.StatusMethodNotAllowed,
		textMediaType,
		methodNotAllowedBody,
		http.MethodPost,
	)
	if body.reads != 0 {
		t.Fatalf("request body Read() calls = %d, want 0", body.reads)
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("zero-dispatch fake was not complete: %v", err)
	}
}

func newTestHandler(t *testing.T, invoker provider.Invoker) http.Handler {
	t.Helper()
	executor, err := modelturn.NewExecutor(invoker)
	if err != nil {
		t.Fatalf("modelturn.NewExecutor() returned an error: %v", err)
	}
	handler, err := NewHandler(executor)
	if err != nil {
		t.Fatalf("NewHandler() returned an error: %v", err)
	}
	return handler
}

func newTestFake(t *testing.T, exchanges ...fake.Exchange) *fake.Provider {
	t.Helper()
	upstream, err := fake.New(exchanges...)
	if err != nil {
		t.Fatalf("fake.New() returned an error: %v", err)
	}
	return upstream
}

func assertHTTPResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	wantMediaType string,
	wantBody string,
	wantAllow string,
) {
	t.Helper()
	assertHTTPMetadata(t, response, wantStatus, wantMediaType, wantAllow)
	if got := response.Body.String(); got != wantBody {
		t.Fatalf("body = %q, want %q", got, wantBody)
	}
}

func assertHTTPMetadata(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantStatus int,
	wantMediaType string,
	wantAllow string,
) {
	t.Helper()
	result := response.Result()
	if result.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", result.StatusCode, wantStatus)
	}
	if got := result.Header.Get("Content-Type"); got != wantMediaType {
		t.Fatalf("Content-Type = %q, want %q", got, wantMediaType)
	}
	if got := result.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if got := result.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := result.Header.Get("Allow"); got != wantAllow {
		t.Fatalf("Allow = %q, want %q", got, wantAllow)
	}
	for _, name := range []string{
		"Retry-After",
		"WWW-Authenticate",
		"Access-Control-Allow-Origin",
		"Location",
		"Set-Cookie",
	} {
		if values, present := result.Header[name]; present || len(values) != 0 {
			t.Fatalf("unexpected response header %s = %q", name, values)
		}
	}
}
