package modelturn

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

type outcomeTestInvokerFunc func(context.Context, provider.Request) (provider.Result, error)

func (function outcomeTestInvokerFunc) Invoke(
	ctx context.Context,
	request provider.Request,
) (provider.Result, error) {
	return function(ctx, request)
}

type outcomeTestCountingReader struct {
	reads int
}

func (reader *outcomeTestCountingReader) Read([]byte) (int, error) {
	reader.reads++
	return 0, io.EOF
}

type outcomeTestTypedNilInvoker struct{}

func (*outcomeTestTypedNilInvoker) Invoke(
	context.Context,
	provider.Request,
) (provider.Result, error) {
	return provider.Result{}, nil
}

func TestOutcomeZeroValueHasNoAlternative(t *testing.T) {
	assertOutcomeTestZero(t, Outcome{})
}

func TestOutcomeFailureAlternativesExposeExactCopiedBodies(t *testing.T) {
	tests := []struct {
		name       string
		outcome    Outcome
		wantBody   string
		wantID     string
		wantIDOK   bool
		wantCode   string
		wantCodeOK bool
	}{
		{
			name:     "uncorrelated",
			outcome:  newUncorrelatedFailure(),
			wantBody: "invalid request\n",
		},
		{
			name:       "invalid request",
			outcome:    newInvalidRequestFailure("req-009"),
			wantBody:   `{"version":"v1","kind":"model_turn.failed","request_id":"req-009","error":{"code":"invalid_request","message":"The request is invalid.","retryable":false}}`,
			wantID:     "req-009",
			wantIDOK:   true,
			wantCode:   "invalid_request",
			wantCodeOK: true,
		},
		{
			name:       "unsupported capability",
			outcome:    newUnsupportedCapabilityFailure("req-009"),
			wantBody:   `{"version":"v1","kind":"model_turn.failed","request_id":"req-009","error":{"code":"unsupported_capability","message":"The required capability is not supported.","retryable":false}}`,
			wantID:     "req-009",
			wantIDOK:   true,
			wantCode:   "unsupported_capability",
			wantCodeOK: true,
		},
		{
			name:       "internal",
			outcome:    newInternalFailure("req-009"),
			wantBody:   `{"version":"v1","kind":"model_turn.failed","request_id":"req-009","error":{"code":"internal_error","message":"The request could not be processed.","retryable":false}}`,
			wantID:     "req-009",
			wantIDOK:   true,
			wantCode:   "internal_error",
			wantCodeOK: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			firstBody, ok := test.outcome.FailureBody()
			if !ok || string(firstBody) != test.wantBody {
				t.Fatalf("FailureBody() = (%q, %t), want (%q, true)", firstBody, ok, test.wantBody)
			}
			if test.name == "uncorrelated" && len(firstBody) != 16 {
				t.Fatalf("uncorrelated FailureBody() length = %d, want 16", len(firstBody))
			}

			firstBody[0] = '!'
			secondBody, ok := test.outcome.FailureBody()
			if !ok || string(secondBody) != test.wantBody {
				t.Fatalf("FailureBody() after caller mutation = (%q, %t), want a fresh copy", secondBody, ok)
			}
			if bytes.Equal(firstBody, secondBody) {
				t.Fatal("FailureBody() returned caller-mutable shared bytes")
			}

			gotID, gotIDOK := test.outcome.RequestID()
			if gotID != test.wantID || gotIDOK != test.wantIDOK {
				t.Fatalf("RequestID() = (%q, %t), want (%q, %t)", gotID, gotIDOK, test.wantID, test.wantIDOK)
			}
			gotCode, gotCodeOK := test.outcome.FailureCode()
			if gotCode != test.wantCode || gotCodeOK != test.wantCodeOK {
				t.Fatalf("FailureCode() = (%q, %t), want (%q, %t)", gotCode, gotCodeOK, test.wantCode, test.wantCodeOK)
			}
			if _, _, ok := test.outcome.ProviderOutcome(); ok {
				t.Fatal("ProviderOutcome() reported true for a failure outcome")
			}
		})
	}
}

func TestOutcomeProviderAlternativesPreserveIdentity(t *testing.T) {
	usage := &provider.Usage{InputTokens: 3, OutputTokens: 5}
	result, err := provider.NewResult("bounded output", usage)
	if err != nil {
		t.Fatalf("provider.NewResult() returned an error: %v", err)
	}
	failure, err := provider.NewFailure(provider.FailureUnavailable, true, usage)
	if err != nil {
		t.Fatalf("provider.NewFailure() returned an error: %v", err)
	}

	tests := []struct {
		name       string
		outcome    Outcome
		wantID     string
		wantResult provider.Result
		wantErr    error
	}{
		{
			name:       "completed result",
			outcome:    newProviderOutcome("req-result", result, nil),
			wantID:     "req-result",
			wantResult: result,
		},
		{
			name:    "normalized failure",
			outcome: newProviderOutcome("req-failure", provider.Result{}, failure),
			wantID:  "req-failure",
			wantErr: failure,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotResult, gotErr, ok := test.outcome.ProviderOutcome()
			if !ok {
				t.Fatal("ProviderOutcome() reported false")
			}
			if gotResult != test.wantResult || gotErr != test.wantErr {
				t.Fatalf("ProviderOutcome() did not preserve the exact result/error alternative")
			}
			gotID, idOK := test.outcome.RequestID()
			if !idOK || gotID != test.wantID {
				t.Fatalf("RequestID() = (%q, %t), want (%q, true)", gotID, idOK, test.wantID)
			}
			if _, ok := test.outcome.FailureBody(); ok {
				t.Fatal("FailureBody() reported true for a provider outcome")
			}
			if _, ok := test.outcome.FailureCode(); ok {
				t.Fatal("FailureCode() reported true for a provider outcome")
			}
		})
	}
}

func TestNewExecutorRejectsNilInvokerInterface(t *testing.T) {
	executor, err := NewExecutor(nil)
	if executor != nil {
		t.Fatal("NewExecutor(nil) returned a nonnil executor")
	}
	if err == nil || err.Error() != "model-turn invoker is required" {
		t.Fatalf("NewExecutor(nil) error = %v, want exact fixed error", err)
	}
}

func TestNewExecutorLeavesTypedNilDetectionToTheCaller(t *testing.T) {
	var typedNil *outcomeTestTypedNilInvoker

	executor, err := NewExecutor(typedNil)

	if err != nil || executor == nil {
		t.Fatalf("NewExecutor(typed nil) = (%v, %v), want accepted interface", executor, err)
	}
}

func TestExecuteRejectsNilInputsBeforeReadOrInvocation(t *testing.T) {
	invocations := 0
	executor, err := NewExecutor(outcomeTestInvokerFunc(func(
		context.Context,
		provider.Request,
	) (provider.Result, error) {
		invocations++
		return provider.Result{}, nil
	}))
	if err != nil {
		t.Fatalf("NewExecutor() returned an error: %v", err)
	}

	t.Run("context", func(t *testing.T) {
		reader := &outcomeTestCountingReader{}
		outcome, executeErr := executor.Execute(nil, reader)
		if executeErr == nil || executeErr.Error() != "model-turn context is required" {
			t.Fatalf("Execute(nil, reader) error = %v, want exact fixed error", executeErr)
		}
		assertOutcomeTestZero(t, outcome)
		if reader.reads != 0 {
			t.Fatalf("reader Read() calls = %d, want 0", reader.reads)
		}
		if invocations != 0 {
			t.Fatalf("Invoke() calls = %d, want 0", invocations)
		}
	})

	t.Run("reader", func(t *testing.T) {
		outcome, executeErr := executor.Execute(context.Background(), nil)
		if executeErr == nil || executeErr.Error() != "model-turn body reader is required" {
			t.Fatalf("Execute(context, nil) error = %v, want exact fixed error", executeErr)
		}
		assertOutcomeTestZero(t, outcome)
		if invocations != 0 {
			t.Fatalf("Invoke() calls = %d, want 0", invocations)
		}
	})
}

func assertOutcomeTestZero(t *testing.T, outcome Outcome) {
	t.Helper()
	if _, ok := outcome.RequestID(); ok {
		t.Fatal("RequestID() reported true for zero Outcome")
	}
	if _, ok := outcome.FailureBody(); ok {
		t.Fatal("FailureBody() reported true for zero Outcome")
	}
	if _, ok := outcome.FailureCode(); ok {
		t.Fatal("FailureCode() reported true for zero Outcome")
	}
	if _, _, ok := outcome.ProviderOutcome(); ok {
		t.Fatal("ProviderOutcome() reported true for zero Outcome")
	}
}
