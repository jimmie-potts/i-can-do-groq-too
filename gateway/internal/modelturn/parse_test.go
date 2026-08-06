package modelturn

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

type parseRecordingReader struct {
	data             []byte
	offset           int
	maxChunk         int
	terminalWithData error
	offered          []int
	deliveredBefore  []int
}

func (reader *parseRecordingReader) Read(destination []byte) (int, error) {
	reader.offered = append(reader.offered, len(destination))
	reader.deliveredBefore = append(reader.deliveredBefore, reader.offset)
	if reader.offset == len(reader.data) {
		return 0, io.EOF
	}

	available := len(reader.data) - reader.offset
	count := min(len(destination), available)
	if reader.maxChunk > 0 {
		count = min(count, reader.maxChunk)
	}
	copy(destination, reader.data[reader.offset:reader.offset+count])
	reader.offset += count
	if reader.offset == len(reader.data) && reader.terminalWithData != nil {
		return count, reader.terminalWithData
	}
	return count, nil
}

type parseSingleReadReader struct {
	data  []byte
	err   error
	calls int
}

func (reader *parseSingleReadReader) Read(destination []byte) (int, error) {
	reader.calls++
	if reader.calls > 1 {
		return 0, io.EOF
	}
	return copy(destination, reader.data), reader.err
}

type parseNoProgressReader struct {
	calls int
}

func (reader *parseNoProgressReader) Read([]byte) (int, error) {
	reader.calls++
	return 0, nil
}

type parseDelayedEOFReader struct {
	zeroReads int
	calls     int
	data      []byte
}

type parseResetNoProgressReader struct {
	calls int
}

func (reader *parseResetNoProgressReader) Read(destination []byte) (int, error) {
	reader.calls++
	switch {
	case reader.calls <= maxConsecutiveNoReads-1:
		return 0, nil
	case reader.calls == maxConsecutiveNoReads:
		return copy(destination, "n"), nil
	case reader.calls < 2*maxConsecutiveNoReads:
		return 0, nil
	default:
		return copy(destination, "ull"), io.EOF
	}
}

func (reader *parseDelayedEOFReader) Read(destination []byte) (int, error) {
	reader.calls++
	if reader.calls <= reader.zeroReads {
		return 0, nil
	}
	return copy(destination, reader.data), io.EOF
}

type parseInvalidCountReader struct {
	count func(int) int
	err   error
	calls int
}

func (reader *parseInvalidCountReader) Read(destination []byte) (int, error) {
	reader.calls++
	return reader.count(len(destination)), reader.err
}

type parseNeverInvoker struct{}

func (parseNeverInvoker) Invoke(context.Context, provider.Request) (provider.Result, error) {
	panic("provider invocation must not start during parser rejection")
}

type parseCountingInvoker struct {
	calls  int
	result provider.Result
}

func (invoker *parseCountingInvoker) Invoke(
	context.Context,
	provider.Request,
) (provider.Result, error) {
	invoker.calls++
	return invoker.result, nil
}

func TestReadRequestBodyEnforcesLimitPlusOneAndOfferedSizes(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "exact limit", size: MaxRequestBodyBytes},
		{name: "one byte over", size: MaxRequestBodyBytes + 1, wantErr: errRequestBodyTooLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := parsePaddedValidDocument(t, test.size)
			reader := &parseRecordingReader{
				data:             document,
				maxChunk:         12_345,
				terminalWithData: io.EOF,
			}

			got, err := readRequestBody(reader)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("readRequestBody() error = %v, want %v", err, test.wantErr)
			}
			parseAssertOfferedSizes(t, reader)
			if reader.offset != test.size {
				t.Fatalf("reader delivered %d bytes, want %d", reader.offset, test.size)
			}
			if test.wantErr != nil {
				if got != nil {
					t.Fatalf("readRequestBody() retained %d bytes on rejection, want nil", len(got))
				}
				return
			}
			if !bytes.Equal(got, document) {
				t.Fatal("readRequestBody() changed the exact-limit document")
			}
			if cap(got) > MaxRequestBodyBytes+1 {
				t.Fatalf("readRequestBody() retained capacity %d, want at most limit plus one", cap(got))
			}
			if !validateStrictDocument(got) {
				t.Fatal("exact-limit otherwise valid document failed strict parsing")
			}
		})
	}
}

func TestReadRequestBodyCountsOverflowDataBeforeNonEOFError(t *testing.T) {
	readerError := errors.New("PARSE-ORDERING-SECRET")
	reader := &parseRecordingReader{
		data:             parsePaddedValidDocument(t, MaxRequestBodyBytes+1),
		maxChunk:         12_345,
		terminalWithData: readerError,
	}

	body, err := readRequestBody(reader)

	if !errors.Is(err, errRequestBodyTooLarge) {
		t.Fatalf("readRequestBody() error = %v, want overflow before terminal reader error", err)
	}
	if body != nil {
		t.Fatalf("readRequestBody() body length = %d, want discarded overflow", len(body))
	}
	if reader.offset != MaxRequestBodyBytes+1 {
		t.Fatalf("reader delivered %d bytes, want limit plus one", reader.offset)
	}
}

func TestExecuteAdmitsExactLimitAndRejectsOneByteOver(t *testing.T) {
	result, err := provider.NewResult("limit result", nil)
	if err != nil {
		t.Fatalf("provider.NewResult() returned an error: %v", err)
	}
	tests := []struct {
		name      string
		size      int
		wantCalls int
	}{
		{name: "exact limit", size: MaxRequestBodyBytes, wantCalls: 1},
		{name: "one byte over", size: MaxRequestBodyBytes + 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invoker := &parseCountingInvoker{result: result}
			executor, err := NewExecutor(invoker)
			if err != nil {
				t.Fatalf("NewExecutor() returned an error: %v", err)
			}
			reader := &parseRecordingReader{
				data:             parsePaddedValidDocument(t, test.size),
				maxChunk:         12_345,
				terminalWithData: io.EOF,
			}

			outcome, executeErr := executor.Execute(context.Background(), reader)

			if executeErr != nil {
				t.Fatalf("Execute() returned an error: %v", executeErr)
			}
			if invoker.calls != test.wantCalls {
				t.Fatalf("Invoke() calls = %d, want %d", invoker.calls, test.wantCalls)
			}
			if test.wantCalls == 1 {
				gotResult, gotErr, ok := outcome.ProviderOutcome()
				if !ok || gotResult != result || gotErr != nil {
					t.Fatal("exact-limit body did not preserve its provider result")
				}
				return
			}
			body, ok := outcome.FailureBody()
			if !ok || string(body) != uncorrelatedFailureBody {
				t.Fatalf("FailureBody() = (%q, %t), want fixed uncorrelated failure", body, ok)
			}
		})
	}
}

func TestReadRequestBodyCountsDataReturnedWithTerminalErrors(t *testing.T) {
	readerError := errors.New("PARSE-READER-ERROR-MUST-NOT-ESCAPE")
	tests := []struct {
		name     string
		readErr  error
		wantBody string
		wantErr  error
	}{
		{name: "data with EOF completes body", readErr: io.EOF, wantBody: "null"},
		{name: "data with non-EOF error is discarded", readErr: readerError, wantErr: readerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &parseSingleReadReader{data: []byte("null"), err: test.readErr}

			got, err := readRequestBody(reader)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("readRequestBody() error = %v, want %v", err, test.wantErr)
			}
			if string(got) != test.wantBody {
				t.Fatalf("readRequestBody() body = %q, want %q", got, test.wantBody)
			}
			if reader.calls != 1 {
				t.Fatalf("Read() calls = %d, want 1", reader.calls)
			}
		})
	}
}

func TestExecuteHidesNonEOFReaderErrorAndDiscardsPrefix(t *testing.T) {
	const readerSecret = "PARSE-READER-ERROR-MUST-NOT-ESCAPE"
	readerError := errors.New(readerSecret)
	executor, err := NewExecutor(parseNeverInvoker{})
	if err != nil {
		t.Fatalf("NewExecutor() returned an error: %v", err)
	}
	reader := &parseSingleReadReader{
		data: []byte(`{"version":"v1","kind":"model_turn.request","request_id":"unsafe-prefix"}`),
		err:  readerError,
	}

	outcome, err := executor.Execute(context.Background(), reader)

	if err != nil {
		t.Fatalf("Execute() returned an ordinary error: %v", err)
	}
	body, ok := outcome.FailureBody()
	if !ok || string(body) != "invalid request\n" {
		t.Fatalf("FailureBody() = (%q, %t), want canonical uncorrelated body", body, ok)
	}
	if strings.Contains(string(body), readerSecret) {
		t.Fatal("FailureBody() exposed the raw reader error")
	}
	if _, ok := outcome.RequestID(); ok {
		t.Fatal("RequestID() recovered an ID from a discarded prefix")
	}
	if _, ok := outcome.FailureCode(); ok {
		t.Fatal("FailureCode() reported a protocol code for an uncorrelated failure")
	}
}

func TestReadRequestBodyStopsAfterOneHundredNoProgressReads(t *testing.T) {
	reader := &parseNoProgressReader{}

	body, err := readRequestBody(reader)

	if !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("readRequestBody() error = %v, want io.ErrNoProgress", err)
	}
	if body != nil {
		t.Fatalf("readRequestBody() body = %q, want nil", body)
	}
	if reader.calls != maxConsecutiveNoReads {
		t.Fatalf("Read() calls = %d, want %d", reader.calls, maxConsecutiveNoReads)
	}
}

func TestReadRequestBodyAllowsNinetyNineNoProgressReads(t *testing.T) {
	reader := &parseDelayedEOFReader{
		zeroReads: maxConsecutiveNoReads - 1,
		data:      []byte("null"),
	}

	body, err := readRequestBody(reader)

	if err != nil {
		t.Fatalf("readRequestBody() returned an error after 99 stalls: %v", err)
	}
	if string(body) != "null" {
		t.Fatalf("readRequestBody() body = %q, want null", body)
	}
	if reader.calls != maxConsecutiveNoReads {
		t.Fatalf("Read() calls = %d, want %d", reader.calls, maxConsecutiveNoReads)
	}
}

func TestReadRequestBodyResetsNoProgressCountAfterData(t *testing.T) {
	reader := &parseResetNoProgressReader{}

	body, err := readRequestBody(reader)

	if err != nil {
		t.Fatalf("readRequestBody() returned an error after separated stalls: %v", err)
	}
	if string(body) != "null" {
		t.Fatalf("readRequestBody() body = %q, want null", body)
	}
	if reader.calls != 2*maxConsecutiveNoReads {
		t.Fatalf("Read() calls = %d, want %d", reader.calls, 2*maxConsecutiveNoReads)
	}
}

func TestExecuteTurnsNoProgressIntoUncorrelatedFailure(t *testing.T) {
	executor, err := NewExecutor(parseNeverInvoker{})
	if err != nil {
		t.Fatalf("NewExecutor() returned an error: %v", err)
	}
	reader := &parseNoProgressReader{}

	outcome, executeErr := executor.Execute(context.Background(), reader)

	if executeErr != nil {
		t.Fatalf("Execute() returned an ordinary error: %v", executeErr)
	}
	body, ok := outcome.FailureBody()
	if !ok || string(body) != uncorrelatedFailureBody {
		t.Fatalf("FailureBody() = (%q, %t), want fixed uncorrelated failure", body, ok)
	}
	if _, ok := outcome.RequestID(); ok {
		t.Fatal("RequestID() reported correlation after no-progress failure")
	}
	if _, ok := outcome.FailureCode(); ok {
		t.Fatal("FailureCode() reported a protocol code after no-progress failure")
	}
	if reader.calls != maxConsecutiveNoReads {
		t.Fatalf("Read() calls = %d, want %d", reader.calls, maxConsecutiveNoReads)
	}
}

func TestReadRequestBodyRejectsInvalidReaderCountsBeforeTerminalErrors(t *testing.T) {
	tests := []struct {
		name  string
		count func(int) int
		err   error
	}{
		{name: "negative count", count: func(int) int { return -1 }},
		{name: "count above offered buffer", count: func(size int) int { return size + 1 }},
		{
			name:  "count above offered buffer with EOF",
			count: func(size int) int { return size + 1 },
			err:   io.EOF,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &parseInvalidCountReader{count: test.count, err: test.err}

			body, err := readRequestBody(reader)

			if !errors.Is(err, errReaderContract) {
				t.Fatalf("readRequestBody() error = %v, want invalid-count error", err)
			}
			if body != nil {
				t.Fatalf("readRequestBody() body = %q, want nil", body)
			}
			if reader.calls != 1 {
				t.Fatalf("Read() calls = %d, want 1", reader.calls)
			}
		})
	}
}

func TestValidateStrictDocumentRejectsDuplicateDecodedNamesAtEveryDepth(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "same spelling at root", raw: `{"name":1,"name":2}`},
		{name: "escaped collision at root", raw: `{"name":1,"n\u0061me":2}`},
		{
			name: "escaped collision nested in array and object",
			raw:  `{"outer":[{"name":1,"\u006eame":2}]}`,
		},
		{
			name: "surrogate pair collides with literal scalar",
			raw:  `{"\ud83d\ude00":1,"😀":2}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if validateStrictDocument([]byte(test.raw)) {
				t.Fatalf("validateStrictDocument(%s) = true, want duplicate rejection", test.raw)
			}
		})
	}
}

func TestValidateStrictDocumentEnforcesUTF8AndSurrogatePairs(t *testing.T) {
	invalidUTF8 := []byte{'"', 0xff, '"'}
	tests := []struct {
		name string
		raw  []byte
		want bool
	}{
		{name: "invalid UTF-8", raw: invalidUTF8},
		{name: "lone high surrogate", raw: []byte(`"\ud800"`)},
		{name: "lone low surrogate", raw: []byte(`"\udc00"`)},
		{name: "high surrogate followed by high surrogate", raw: []byte(`"\ud800\ud801"`)},
		{name: "valid surrogate pair", raw: []byte(`"\ud83d\ude00"`), want: true},
		{name: "literal scalar", raw: []byte(`"😀"`), want: true},
		{name: "escaped backslash before surrogate text", raw: []byte(`"\\ud800"`), want: true},
		{name: "odd escaped slash leaves lone surrogate", raw: []byte(`"\\\ud800"`)},
		{name: "two escaped backslashes leave surrogate text", raw: []byte(`"\\\\ud800"`), want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateStrictDocument(test.raw); got != test.want {
				t.Fatalf("validateStrictDocument(%q) = %t, want %t", test.raw, got, test.want)
			}
		})
	}
}

func TestValidateStrictDocumentRequiresOneCompleteValue(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "one value", raw: `{"value":true}`, want: true},
		{name: "trailing whitespace", raw: "{} \r\n\t", want: true},
		{name: "second value", raw: `{} []`},
		{name: "trailing non-whitespace", raw: `{}x`},
		{name: "empty", raw: ``},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateStrictDocument([]byte(test.raw)); got != test.want {
				t.Fatalf("validateStrictDocument(%q) = %t, want %t", test.raw, got, test.want)
			}
		})
	}
}

func TestValidateStrictDocumentUsesJSONNumberSyntaxWithoutFloatConversion(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "integer", raw: `0`, want: true},
		{name: "fraction and exponent", raw: `-12.5e+8`, want: true},
		{name: "huge exponent remains valid JSON syntax", raw: `1e999999`, want: true},
		{name: "leading zero", raw: `01`},
		{name: "leading plus", raw: `+1`},
		{name: "leading decimal point", raw: `.1`},
		{name: "trailing decimal point", raw: `1.`},
		{name: "missing exponent digits", raw: `1e`},
		{name: "NaN", raw: `NaN`},
		{name: "infinity", raw: `Infinity`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateStrictDocument([]byte(test.raw)); got != test.want {
				t.Fatalf("validateStrictDocument(%q) = %t, want %t", test.raw, got, test.want)
			}
		})
	}
}

func TestValidateStrictDocumentCountsRootAsDepthOne(t *testing.T) {
	tests := []struct {
		name  string
		depth int
		root  string
		want  bool
	}{
		{name: "sixteen nested arrays", depth: 16, want: true},
		{name: "seventeen nested arrays", depth: 17},
		{name: "root object plus fifteen arrays", depth: 15, root: "object", want: true},
		{name: "root object plus sixteen arrays", depth: 16, root: "object"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := strings.Repeat("[", test.depth) + "0" + strings.Repeat("]", test.depth)
			if test.root == "object" {
				raw = `{"nested":` + raw + `}`
			}
			if got := validateStrictDocument([]byte(raw)); got != test.want {
				t.Fatalf("validateStrictDocument() at requested depth = %t, want %t", got, test.want)
			}
		})
	}
}

func parsePaddedValidDocument(t *testing.T, size int) []byte {
	t.Helper()
	base := []byte(
		`{"version":"v1","kind":"model_turn.request","request_id":"limit-test",` +
			`"model_alias":"learning-text","conversation":[{"role":"user","content":"bounded"}],` +
			`"instructions":[],"required_capabilities":[]}`,
	)
	if size < len(base) {
		t.Fatalf("requested padded size %d is below base document size %d", size, len(base))
	}
	document := make([]byte, size)
	copy(document, base)
	for index := len(base); index < len(document); index++ {
		document[index] = ' '
	}
	return document
}

func parseAssertOfferedSizes(t *testing.T, reader *parseRecordingReader) {
	t.Helper()
	if len(reader.offered) == 0 || len(reader.offered) != len(reader.deliveredBefore) {
		t.Fatalf(
			"recorded offered sizes = %d and offsets = %d, want equal nonzero counts",
			len(reader.offered),
			len(reader.deliveredBefore),
		)
	}
	for index, offered := range reader.offered {
		remaining := MaxRequestBodyBytes + 1 - reader.deliveredBefore[index]
		want := min(readBufferBytes, remaining)
		if offered != want {
			t.Fatalf(
				"Read() call %d offered %d bytes at offset %d, want %d",
				index+1,
				offered,
				reader.deliveredBefore[index],
				want,
			)
		}
	}
}
