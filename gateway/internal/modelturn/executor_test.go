package modelturn

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider/fake"
)

type executionDocument struct {
	Version              string                 `json:"version"`
	Kind                 string                 `json:"kind"`
	RequestID            string                 `json:"request_id"`
	ModelAlias           string                 `json:"model_alias"`
	Conversation         []executionMessage     `json:"conversation"`
	Instructions         []executionInstruction `json:"instructions"`
	RequiredCapabilities []string               `json:"required_capabilities"`
}

type executionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type executionInstruction struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

type executionCloseTrackingReader struct {
	*bytes.Reader
	closes int
}

func (reader *executionCloseTrackingReader) Close() error {
	reader.closes++
	return nil
}

func TestExecutorRejectsSemanticPolicyBeforeDispatch(t *testing.T) {
	const (
		promptSecret      = "PROMPT-SEMANTIC-SECRET"
		instructionSecret = "INSTRUCTION-SEMANTIC-SECRET"
		aliasSecret       = "UNKNOWN-ALIAS-SEMANTIC-SECRET"
	)
	tests := []struct {
		name         string
		alias        string
		capabilities []string
		wantCode     string
		wantMessage  string
	}{
		{
			name:         "declared tool capability",
			alias:        supportedAlias,
			capabilities: []string{toolCallsCapability},
			wantCode:     unsupportedCapabilityCode,
			wantMessage:  unsupportedCapabilityMessage,
		},
		{
			name:         "capability precedes unknown alias",
			alias:        aliasSecret,
			capabilities: []string{toolCallsCapability},
			wantCode:     unsupportedCapabilityCode,
			wantMessage:  unsupportedCapabilityMessage,
		},
		{
			name:         "unknown alias",
			alias:        aliasSecret,
			capabilities: []string{},
			wantCode:     invalidRequestCode,
			wantMessage:  invalidRequestMessage,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := executionFake(t)
			executor := executionExecutor(t, upstream)
			body := executionBody(t, func(document *executionDocument) {
				document.ModelAlias = test.alias
				document.Conversation[0].Content = promptSecret
				document.Instructions = []executionInstruction{
					{Source: "policy", Content: instructionSecret},
				}
				document.RequiredCapabilities = test.capabilities
			})

			outcome, err := executor.Execute(context.Background(), strings.NewReader(string(body)))

			if err != nil {
				t.Fatalf("Execute() returned an error: %v", err)
			}
			assertExecutionFailure(
				t,
				outcome,
				"request-009",
				test.wantCode,
				test.wantMessage,
			)
			failureBody, _ := outcome.FailureBody()
			for _, secret := range []string{promptSecret, instructionSecret, aliasSecret} {
				if strings.Contains(string(failureBody), secret) {
					t.Fatalf("failure body exposed sentinel %q", secret)
				}
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("zero-dispatch fake was not complete: %v", err)
			}
		})
	}
}

func TestUnsupportedCapabilityMatchesCanonicalFailureFixture(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join(
		requestFixtureContractRoot(t),
		"fixtures",
		"valid",
		"failed-unsupported-capability.json",
	))
	if err != nil {
		t.Fatalf("read canonical unsupported-capability fixture: %v", err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, fixture); err != nil {
		t.Fatalf("compact canonical unsupported-capability fixture: %v", err)
	}
	upstream := executionFake(t)
	executor := executionExecutor(t, upstream)
	body := executionBody(t, func(document *executionDocument) {
		document.RequestID = "req.learning:2"
		document.RequiredCapabilities = []string{toolCallsCapability}
	})

	outcome, executeErr := executor.Execute(context.Background(), bytes.NewReader(body))

	if executeErr != nil {
		t.Fatalf("Execute() returned an error: %v", executeErr)
	}
	got, ok := outcome.FailureBody()
	if !ok || !bytes.Equal(got, compact.Bytes()) {
		t.Fatalf("FailureBody() = (%q, %t), want compact canonical fixture %q", got, ok, compact.Bytes())
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("zero-dispatch fake was not complete: %v", err)
	}
}

func TestExecutorCorrelatesOnlyAfterTheWholeStrictDocumentIsSafe(t *testing.T) {
	invalidUTF8 := append(
		[]byte(`{"request_id":"req-utf8","content":"`),
		0xff,
	)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	correlatedWrongVersion := executionBody(t, func(document *executionDocument) {
		document.Version = "v2"
		document.Conversation[0].Content = "PROMPT-SCHEMA-SECRET"
	})
	correlatedUnknownCapability := executionBody(t, func(document *executionDocument) {
		document.RequiredCapabilities = []string{"UNKNOWN-CAPABILITY-SCHEMA-SECRET"}
	})
	tests := []struct {
		name       string
		raw        []byte
		correlated bool
	}{
		{name: "wrong version", raw: correlatedWrongVersion, correlated: true},
		{name: "unknown capability", raw: correlatedUnknownCapability, correlated: true},
		{name: "malformed prefix", raw: []byte(`{"request_id":"req-prefix",`)},
		{name: "duplicate identifier", raw: []byte(`{"request_id":"req-first","request_id":"req-second"}`)},
		{name: "nested duplicate", raw: []byte(`{"request_id":"req-nested","nested":{"x":1,"x":2}}`)},
		{name: "invalid UTF-8", raw: invalidUTF8},
		{name: "lone surrogate", raw: []byte(`{"request_id":"req-surrogate","content":"\udc00"}`)},
		{name: "second value", raw: []byte(`{"request_id":"req-first"} {"request_id":"req-second"}`)},
		{name: "non-object root", raw: []byte(`["req-array"]`)},
		{name: "missing identifier", raw: []byte(`{"version":"v1"}`)},
		{name: "invalid identifier", raw: []byte(`{"request_id":"-unsafe"}`)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := executionFake(t)
			executor := executionExecutor(t, upstream)

			outcome, err := executor.Execute(context.Background(), bytes.NewReader(test.raw))

			if err != nil {
				t.Fatalf("Execute() returned an error: %v", err)
			}
			if test.correlated {
				assertExecutionFailure(
					t,
					outcome,
					"request-009",
					invalidRequestCode,
					invalidRequestMessage,
				)
			} else {
				body, ok := outcome.FailureBody()
				if !ok || string(body) != uncorrelatedFailureBody {
					t.Fatalf("FailureBody() = (%q, %t), want fixed uncorrelated failure", body, ok)
				}
				if _, ok := outcome.RequestID(); ok {
					t.Fatal("RequestID() trusted an unsafe identifier")
				}
				if _, ok := outcome.FailureCode(); ok {
					t.Fatal("FailureCode() reported a protocol code without safe correlation")
				}
			}
			failureBody, _ := outcome.FailureBody()
			if bytes.Contains(failureBody, []byte("SECRET")) {
				t.Fatal("failure body exposed schema-invalid request content")
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("zero-dispatch fake was not complete: %v", err)
			}
		})
	}
}

func TestExecutorConsumesButDoesNotCloseBody(t *testing.T) {
	upstream := executionFake(t)
	executor := executionExecutor(t, upstream)
	body := executionBody(t, func(document *executionDocument) {
		document.ModelAlias = "unknown-but-valid"
	})
	reader := &executionCloseTrackingReader{Reader: bytes.NewReader(body)}

	_, err := executor.Execute(context.Background(), reader)

	if err != nil {
		t.Fatalf("Execute() returned an error: %v", err)
	}
	if reader.Len() != 0 {
		t.Fatalf("body has %d unread bytes, want complete consumption", reader.Len())
	}
	if reader.closes != 0 {
		t.Fatalf("body Close() calls = %d, want transport-owned closure", reader.closes)
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("zero-dispatch fake was not complete: %v", err)
	}
}

func TestExecutorMapsOneAdmittedRequestAndPreservesResult(t *testing.T) {
	document := executionDefaultDocument()
	document.Conversation = []executionMessage{
		{Role: "user", Content: "exact café"},
		{Role: "assistant", Content: "exact previous reply"},
	}
	document.Instructions = []executionInstruction{
		{Source: "developer", Content: "preserve exact order"},
		{Source: "policy", Content: "do not normalize e\u0301"},
	}
	body := executionMarshal(t, document)
	expected := executionProviderRequest(t, document)
	wantResult, err := provider.NewResult("bounded completion", nil)
	if err != nil {
		t.Fatalf("NewResult() returned an error: %v", err)
	}
	exchange, err := fake.ExpectResult(expected, wantResult)
	if err != nil {
		t.Fatalf("ExpectResult() returned an error: %v", err)
	}
	upstream := executionFake(t, exchange)
	executor := executionExecutor(t, upstream)

	outcome, executeErr := executor.Execute(context.Background(), strings.NewReader(string(body)))

	if executeErr != nil {
		t.Fatalf("Execute() returned an error: %v", executeErr)
	}
	gotResult, gotErr := assertExecutionProviderOutcome(t, outcome, document.RequestID)
	if gotResult != wantResult || gotErr != nil {
		t.Fatalf("provider outcome was not the exact scripted result")
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() returned an error: %v", err)
	}
}

func TestExecutorPreservesResultUsagePresence(t *testing.T) {
	tests := []struct {
		name  string
		usage *provider.Usage
	}{
		{name: "absent"},
		{name: "observed zero", usage: &provider.Usage{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := executionDefaultDocument()
			expected := executionProviderRequest(t, document)
			wantResult, err := provider.NewResult("usage result", test.usage)
			if err != nil {
				t.Fatalf("NewResult() returned an error: %v", err)
			}
			exchange, err := fake.ExpectResult(expected, wantResult)
			if err != nil {
				t.Fatalf("ExpectResult() returned an error: %v", err)
			}
			upstream := executionFake(t, exchange)
			executor := executionExecutor(t, upstream)

			outcome, executeErr := executor.Execute(
				context.Background(),
				strings.NewReader(string(executionMarshal(t, document))),
			)

			if executeErr != nil {
				t.Fatalf("Execute() returned an error: %v", executeErr)
			}
			gotResult, gotErr := assertExecutionProviderOutcome(t, outcome, document.RequestID)
			if gotResult != wantResult || gotErr != nil {
				t.Fatalf("provider outcome was not the exact result")
			}
			gotUsage, gotSeen := gotResult.Usage()
			if gotSeen != (test.usage != nil) || gotUsage != (provider.Usage{}) {
				t.Fatalf("Usage() = (%+v, %t), want zero with presence %t", gotUsage, gotSeen, test.usage != nil)
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestExecutorPreservesDirectFailureAndUsagePresence(t *testing.T) {
	tests := []struct {
		name  string
		usage *provider.Usage
	}{
		{name: "absent"},
		{name: "observed zero", usage: &provider.Usage{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := executionDefaultDocument()
			expected := executionProviderRequest(t, document)
			wantFailure, err := provider.NewFailure(provider.FailureRateLimited, true, test.usage)
			if err != nil {
				t.Fatalf("NewFailure() returned an error: %v", err)
			}
			exchange, err := fake.ExpectFailure(expected, wantFailure)
			if err != nil {
				t.Fatalf("ExpectFailure() returned an error: %v", err)
			}
			upstream := executionFake(t, exchange)
			executor := executionExecutor(t, upstream)

			outcome, executeErr := executor.Execute(
				context.Background(),
				strings.NewReader(string(executionMarshal(t, document))),
			)

			if executeErr != nil {
				t.Fatalf("Execute() returned an error: %v", executeErr)
			}
			gotResult, gotErr := assertExecutionProviderOutcome(t, outcome, document.RequestID)
			if gotResult != (provider.Result{}) || gotErr != wantFailure {
				t.Fatalf("provider outcome did not preserve the direct failure")
			}
			gotUsage, gotSeen := wantFailure.Usage()
			if gotSeen != (test.usage != nil) || gotUsage != (provider.Usage{}) {
				t.Fatalf("Usage() = (%+v, %t), want zero with presence %t", gotUsage, gotSeen, test.usage != nil)
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestExecutorForwardsExactContextAndPreservesMatchingTermination(t *testing.T) {
	tests := []struct {
		name    string
		newCtx  func() (context.Context, context.CancelFunc)
		wantErr error
	}{
		{
			name: "canceled",
			newCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, func() {}
			},
			wantErr: context.Canceled,
		},
		{
			name: "deadline exceeded",
			newCtx: func() (context.Context, context.CancelFunc) {
				return context.WithDeadline(context.Background(), time.Unix(1, 0))
			},
			wantErr: context.DeadlineExceeded,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := test.newCtx()
			defer cancel()
			recorder := &executionRecordingInvoker{err: test.wantErr}
			executor := executionExecutor(t, recorder)
			document := executionDefaultDocument()

			outcome, err := executor.Execute(
				ctx,
				strings.NewReader(string(executionMarshal(t, document))),
			)

			if err != nil {
				t.Fatalf("Execute() returned an error: %v", err)
			}
			gotResult, gotErr := assertExecutionProviderOutcome(t, outcome, document.RequestID)
			if gotResult != (provider.Result{}) || gotErr != test.wantErr {
				t.Fatalf("provider outcome did not preserve exact context sentinel")
			}
			if recorder.calls != 1 || recorder.ctx != ctx {
				t.Fatalf("Invoke() calls/context = (%d, same=%t), want (1, true)", recorder.calls, recorder.ctx == ctx)
			}
		})
	}
}

func TestExecutorFailsClosedOnMappingInconsistencyWithoutDispatch(t *testing.T) {
	recorder := &executionRecordingInvoker{}
	executor := executionExecutor(t, recorder)
	request := wireRequest{
		requestID:  "mapping-inconsistency",
		modelAlias: supportedAlias,
		conversation: []provider.Message{
			{Role: provider.MessageRole("unreachable-role"), Content: "MAPPING-SECRET"},
		},
	}

	outcome := executor.executeAdmitted(context.Background(), request)

	assertExecutionFailure(
		t,
		outcome,
		request.requestID,
		internalErrorCode,
		internalErrorMessage,
	)
	if recorder.calls != 0 {
		t.Fatalf("Invoke() calls = %d, want zero after mapping inconsistency", recorder.calls)
	}
}

func TestExecutorReplacesInvalidInvocationAlternativesWithoutTraversal(t *testing.T) {
	validResult, err := provider.NewResult("valid but mixed", nil)
	if err != nil {
		t.Fatalf("NewResult() returned an error: %v", err)
	}
	validFailure, err := provider.NewFailure(provider.FailureInternal, false, nil)
	if err != nil {
		t.Fatalf("NewFailure() returned an error: %v", err)
	}
	var typedNilFailure *provider.Failure
	rawTrap := &executionErrorTrap{}
	wrappedTrap := &executionWrapperTrap{nested: validFailure}
	mixedTrap := &executionErrorTrap{}
	tests := []struct {
		name   string
		result provider.Result
		err    error
		traps  []executionTrapCounts
	}{
		{name: "raw error", err: rawTrap, traps: []executionTrapCounts{rawTrap}},
		{name: "wrapped normalized failure", err: wrappedTrap, traps: []executionTrapCounts{wrappedTrap}},
		{name: "non-comparable error", err: executionNonComparableError("NON-COMPARABLE-SECRET")},
		{name: "direct typed nil failure", err: typedNilFailure},
		{name: "fabricated cancellation", err: context.Canceled},
		{name: "invalid result"},
		{name: "simultaneous result and error", result: validResult, err: mixedTrap, traps: []executionTrapCounts{mixedTrap}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := &executionRecordingInvoker{result: test.result, err: test.err}
			executor := executionExecutor(t, recorder)
			document := executionDefaultDocument()

			outcome, executeErr := executor.Execute(
				context.Background(),
				strings.NewReader(string(executionMarshal(t, document))),
			)

			if executeErr != nil {
				t.Fatalf("Execute() returned an error: %v", executeErr)
			}
			assertExecutionFailure(
				t,
				outcome,
				document.RequestID,
				internalErrorCode,
				internalErrorMessage,
			)
			if recorder.calls != 1 {
				t.Fatalf("Invoke() calls = %d, want exactly one", recorder.calls)
			}
			for _, trap := range test.traps {
				errorCalls, unwrapCalls := trap.executionCounts()
				if errorCalls != 0 || unwrapCalls != 0 {
					t.Fatalf("invalid error was traversed: Error=%d Unwrap=%d", errorCalls, unwrapCalls)
				}
			}
		})
	}
}

type executionRecordingInvoker struct {
	calls   int
	ctx     context.Context
	request provider.Request
	result  provider.Result
	err     error
}

func (recorder *executionRecordingInvoker) Invoke(
	ctx context.Context,
	request provider.Request,
) (provider.Result, error) {
	recorder.calls++
	recorder.ctx = ctx
	recorder.request = request
	return recorder.result, recorder.err
}

type executionTrapCounts interface {
	executionCounts() (int, int)
}

type executionErrorTrap struct {
	errorCalls int
}

func (trap *executionErrorTrap) Error() string {
	trap.errorCalls++
	panic("invalid error was formatted")
}

func (trap *executionErrorTrap) executionCounts() (int, int) {
	return trap.errorCalls, 0
}

type executionWrapperTrap struct {
	nested      error
	errorCalls  int
	unwrapCalls int
}

type executionNonComparableError []byte

func (executionNonComparableError) Error() string {
	panic("non-comparable invocation error was formatted")
}

func (trap *executionWrapperTrap) Error() string {
	trap.errorCalls++
	panic("invalid wrapped error was formatted")
}

func (trap *executionWrapperTrap) Unwrap() error {
	trap.unwrapCalls++
	panic("invalid wrapped error was unwrapped")
}

func (trap *executionWrapperTrap) executionCounts() (int, int) {
	return trap.errorCalls, trap.unwrapCalls
}

func executionDefaultDocument() executionDocument {
	return executionDocument{
		Version:              "v1",
		Kind:                 "model_turn.request",
		RequestID:            "request-009",
		ModelAlias:           supportedAlias,
		Conversation:         []executionMessage{{Role: "user", Content: "hello"}},
		Instructions:         []executionInstruction{},
		RequiredCapabilities: []string{},
	}
}

func executionBody(t *testing.T, edit func(*executionDocument)) []byte {
	t.Helper()
	document := executionDefaultDocument()
	edit(&document)
	return executionMarshal(t, document)
}

func executionMarshal(t *testing.T, document executionDocument) []byte {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() returned an error: %v", err)
	}
	return body
}

func executionProviderRequest(t *testing.T, document executionDocument) provider.Request {
	t.Helper()
	conversation := make([]provider.Message, len(document.Conversation))
	for index, message := range document.Conversation {
		role := provider.MessageRoleUser
		if message.Role == "assistant" {
			role = provider.MessageRoleAssistant
		}
		conversation[index] = provider.Message{Role: role, Content: message.Content}
	}
	instructions := make([]provider.Instruction, len(document.Instructions))
	for index, instruction := range document.Instructions {
		instructions[index] = provider.Instruction{
			Source:  instruction.Source,
			Content: instruction.Content,
		}
	}
	request, err := provider.NewRequest(conversation, instructions, nil)
	if err != nil {
		t.Fatalf("provider.NewRequest() returned an error: %v", err)
	}
	return request
}

func executionExecutor(t *testing.T, invoker provider.Invoker) *Executor {
	t.Helper()
	executor, err := NewExecutor(invoker)
	if err != nil {
		t.Fatalf("NewExecutor() returned an error: %v", err)
	}
	return executor
}

func executionFake(t *testing.T, exchanges ...fake.Exchange) *fake.Provider {
	t.Helper()
	upstream, err := fake.New(exchanges...)
	if err != nil {
		t.Fatalf("fake.New() returned an error: %v", err)
	}
	return upstream
}

func assertExecutionFailure(
	t *testing.T,
	outcome Outcome,
	requestID string,
	code string,
	message string,
) {
	t.Helper()
	gotID, hasID := outcome.RequestID()
	if !hasID || gotID != requestID {
		t.Fatalf("RequestID() = (%q, %t), want (%q, true)", gotID, hasID, requestID)
	}
	gotCode, hasCode := outcome.FailureCode()
	if !hasCode || gotCode != code {
		t.Fatalf("FailureCode() = (%q, %t), want (%q, true)", gotCode, hasCode, code)
	}
	wantBody := `{"version":"v1","kind":"model_turn.failed","request_id":"` +
		requestID + `","error":{"code":"` + code + `","message":"` +
		message + `","retryable":false}}`
	gotBody, hasBody := outcome.FailureBody()
	if !hasBody || string(gotBody) != wantBody {
		t.Fatalf("FailureBody() = (%q, %t), want (%q, true)", gotBody, hasBody, wantBody)
	}
	if _, _, isProvider := outcome.ProviderOutcome(); isProvider {
		t.Fatal("ProviderOutcome() reported a failure as a provider outcome")
	}
}

func assertExecutionProviderOutcome(
	t *testing.T,
	outcome Outcome,
	requestID string,
) (provider.Result, error) {
	t.Helper()
	gotID, hasID := outcome.RequestID()
	if !hasID || gotID != requestID {
		t.Fatalf("RequestID() = (%q, %t), want (%q, true)", gotID, hasID, requestID)
	}
	if _, hasBody := outcome.FailureBody(); hasBody {
		t.Fatal("FailureBody() reported a provider outcome as a failure")
	}
	if _, hasCode := outcome.FailureCode(); hasCode {
		t.Fatal("FailureCode() reported a provider outcome as a failure")
	}
	result, err, ok := outcome.ProviderOutcome()
	if !ok {
		t.Fatal("ProviderOutcome() did not report the provider alternative")
	}
	return result, err
}

var _ provider.Invoker = (*executionRecordingInvoker)(nil)
var _ error = (*executionErrorTrap)(nil)
var _ error = (*executionWrapperTrap)(nil)
var _ error = executionNonComparableError(nil)
var _ executionTrapCounts = (*executionErrorTrap)(nil)
var _ executionTrapCounts = (*executionWrapperTrap)(nil)
