package fake_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider/fake"
)

var _ provider.Invoker = (*fake.Provider)(nil)

func TestProviderReturnsOrderedResultsAndOwnsScript(t *testing.T) {
	firstRequest := newRequest(t, "first request", "first-source", "first instruction", true)
	secondRequest := newRequest(t, "second request", "second-source", "second instruction", false)
	firstResult := newResult(t, "first result", nil)
	secondResult := newResult(t, "second result", nil)
	firstExchange := expectResult(t, firstRequest, firstResult)
	secondExchange := expectResult(t, secondRequest, secondResult)
	script := []fake.Exchange{firstExchange, secondExchange}
	upstream := newFake(t, script...)

	script[0] = secondExchange

	gotFirst, err := upstream.Invoke(context.Background(), firstRequest)
	if err != nil || gotFirst.OutputText() != "first result" {
		t.Fatalf("first Invoke() = (%q, %v), want first result", gotFirst.OutputText(), err)
	}
	gotSecond, err := upstream.Invoke(context.Background(), secondRequest)
	if err != nil || gotSecond.OutputText() != "second result" {
		t.Fatalf("second Invoke() = (%q, %v), want second result", gotSecond.OutputText(), err)
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() returned an error: %v", err)
	}
}

func TestProviderRejectsRequestsOutOfOrder(t *testing.T) {
	firstRequest := newRequest(t, "first request", "source", "instruction", false)
	secondRequest := newRequest(t, "second request", "source", "instruction", false)
	result := newResult(t, "bounded result", nil)
	upstream := newFake(
		t,
		expectResult(t, firstRequest, result),
		expectResult(t, secondRequest, result),
	)

	panicText := capturePanic(t, func() {
		_, _ = upstream.Invoke(context.Background(), secondRequest)
	})

	if !strings.Contains(panicText, "request 1 differed at conversation[0].content") {
		t.Fatalf("panic = %q, want first-request content mismatch", panicText)
	}
	if err := upstream.VerifyComplete(); err == nil || err.Error() != panicText {
		t.Fatalf("VerifyComplete() error = %v, want sticky order violation", err)
	}
}

func TestProviderPreservesCompletedUsage(t *testing.T) {
	tests := []struct {
		name      string
		usage     *provider.Usage
		wantUsage provider.Usage
		wantSeen  bool
	}{
		{name: "absent"},
		{name: "observed zero", usage: &provider.Usage{}, wantSeen: true},
		{
			name: "observed maximum",
			usage: &provider.Usage{
				InputTokens:  provider.MaxUsageTokens,
				OutputTokens: provider.MaxUsageTokens,
			},
			wantUsage: provider.Usage{
				InputTokens:  provider.MaxUsageTokens,
				OutputTokens: provider.MaxUsageTokens,
			},
			wantSeen: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := newRequest(t, "bounded request", "source", "instruction", false)
			result := newResult(t, "bounded result", test.usage)
			upstream := newFake(t, expectResult(t, request, result))
			ctx := context.Background()

			gotResult, gotErr := upstream.Invoke(ctx, request)

			if err := provider.ValidateInvocation(ctx, gotResult, gotErr); err != nil {
				t.Fatalf("ValidateInvocation() returned an error: %v", err)
			}
			gotUsage, gotSeen := gotResult.Usage()
			if gotUsage != test.wantUsage || gotSeen != test.wantSeen {
				t.Fatalf(
					"Usage() = (%+v, %t), want (%+v, %t)",
					gotUsage,
					gotSeen,
					test.wantUsage,
					test.wantSeen,
				)
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestProviderPreservesEveryFailureOutcome(t *testing.T) {
	tests := []struct {
		code      provider.FailureCode
		retryable bool
		usage     *provider.Usage
	}{
		{code: provider.FailureAuthenticationFailed},
		{code: provider.FailureRateLimited, retryable: true, usage: &provider.Usage{}},
		{
			code:  provider.FailureRequestRejected,
			usage: &provider.Usage{InputTokens: 9, OutputTokens: 4},
		},
		{
			code: provider.FailureUnavailable,
			usage: &provider.Usage{
				InputTokens:  provider.MaxUsageTokens,
				OutputTokens: provider.MaxUsageTokens,
			},
		},
		{code: provider.FailureInvalidResponse},
		{code: provider.FailureUnsupportedUpstreamOutput},
		{code: provider.FailureInternal},
	}

	request := newRequest(t, "bounded request", "source", "instruction", false)
	exchanges := make([]fake.Exchange, 0, len(tests))
	failures := make([]*provider.Failure, 0, len(tests))
	for _, test := range tests {
		failure := newFailure(t, test.code, test.retryable, test.usage)
		failures = append(failures, failure)
		exchanges = append(exchanges, expectFailure(t, request, failure))
	}
	upstream := newFake(t, exchanges...)

	for index, test := range tests {
		ctx := context.Background()
		gotResult, gotErr := upstream.Invoke(ctx, request)
		if gotResult != (provider.Result{}) {
			t.Fatalf("Invoke() result for %q is nonzero", test.code)
		}
		if gotErr != failures[index] {
			t.Fatalf("Invoke() error for %q was not the scripted direct failure", test.code)
		}
		if err := provider.ValidateInvocation(ctx, gotResult, gotErr); err != nil {
			t.Fatalf("ValidateInvocation() for %q returned an error: %v", test.code, err)
		}
		gotFailure := gotErr.(*provider.Failure)
		if gotFailure.Code() != test.code || gotFailure.Retryable() != test.retryable {
			t.Fatalf(
				"failure = (%q, %t), want (%q, %t)",
				gotFailure.Code(),
				gotFailure.Retryable(),
				test.code,
				test.retryable,
			)
		}
		gotUsage, gotSeen := gotFailure.Usage()
		wantSeen := test.usage != nil
		wantUsage := provider.Usage{}
		if test.usage != nil {
			wantUsage = *test.usage
		}
		if gotUsage != wantUsage || gotSeen != wantSeen {
			t.Fatalf(
				"failure usage = (%+v, %t), want (%+v, %t)",
				gotUsage,
				gotSeen,
				wantUsage,
				wantSeen,
			)
		}
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() returned an error: %v", err)
	}
}

func TestProviderReportsEveryRequestMismatchWithoutContent(t *testing.T) {
	const (
		expectedMessageSecret           = "EXPECTED-MESSAGE-SECRET"
		actualMessageSecret             = "ACTUAL-MESSAGE-SECRET"
		expectedSecondMessageSecret     = "EXPECTED-SECOND-MESSAGE-SECRET"
		actualSecondMessageSecret       = "ACTUAL-SECOND-MESSAGE-SECRET"
		expectedSourceSecret            = "EXPECTED-SOURCE-SECRET"
		actualSourceSecret              = "ACTUAL-SOURCE-SECRET"
		expectedSecondSourceSecret      = "EXPECTED-SECOND-SOURCE-SECRET"
		actualSecondSourceSecret        = "ACTUAL-SECOND-SOURCE-SECRET"
		expectedInstructionSecret       = "EXPECTED-INSTRUCTION-SECRET"
		actualInstructionSecret         = "ACTUAL-INSTRUCTION-SECRET"
		expectedSecondInstructionSecret = "EXPECTED-SECOND-INSTRUCTION-SECRET"
		actualSecondInstructionSecret   = "ACTUAL-SECOND-INSTRUCTION-SECRET"
	)
	expected := requestFromParts(t, requestParts{
		conversation: []provider.Message{
			{Role: provider.MessageRoleUser, Content: expectedMessageSecret},
			{Role: provider.MessageRoleAssistant, Content: expectedSecondMessageSecret},
		},
		instructions: []provider.Instruction{
			{Source: expectedSourceSecret, Content: expectedInstructionSecret},
			{Source: expectedSecondSourceSecret, Content: expectedSecondInstructionSecret},
		},
		capabilities: []provider.Capability{provider.CapabilityToolCalls},
	})
	result := newResult(t, "bounded result", nil)
	tests := []struct {
		name   string
		path   string
		actual requestParts
	}{
		{
			name: "conversation length",
			path: "conversation.length",
			actual: requestParts{
				conversation: []provider.Message{{Role: provider.MessageRoleUser, Content: expectedMessageSecret}},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "message role",
			path: "conversation[0].role",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleAssistant, Content: expectedMessageSecret},
					{Role: provider.MessageRoleAssistant, Content: expectedSecondMessageSecret},
				},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "message content",
			path: "conversation[0].content",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleUser, Content: actualMessageSecret},
					{Role: provider.MessageRoleAssistant, Content: expectedSecondMessageSecret},
				},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "multiple mismatches report the first path only",
			path: "conversation[0].role",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleAssistant, Content: actualMessageSecret},
					{Role: provider.MessageRoleUser, Content: actualSecondMessageSecret},
				},
				instructions: []provider.Instruction{
					{Source: actualSourceSecret, Content: actualInstructionSecret},
					{Source: actualSecondSourceSecret, Content: actualSecondInstructionSecret},
				},
			},
		},
		{
			name: "conversation entries swapped",
			path: "conversation[0].role",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleAssistant, Content: expectedSecondMessageSecret},
					{Role: provider.MessageRoleUser, Content: expectedMessageSecret},
				},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "second message role",
			path: "conversation[1].role",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleUser, Content: expectedMessageSecret},
					{Role: provider.MessageRoleUser, Content: expectedSecondMessageSecret},
				},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "second message content",
			path: "conversation[1].content",
			actual: requestParts{
				conversation: []provider.Message{
					{Role: provider.MessageRoleUser, Content: expectedMessageSecret},
					{Role: provider.MessageRoleAssistant, Content: actualSecondMessageSecret},
				},
				instructions: expected.Instructions(), capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "instruction length",
			path: "instructions.length",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{{Source: expectedSourceSecret, Content: expectedInstructionSecret}},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "instruction source",
			path: "instructions[0].source",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{
					{Source: actualSourceSecret, Content: expectedInstructionSecret},
					{Source: expectedSecondSourceSecret, Content: expectedSecondInstructionSecret},
				},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "instruction content",
			path: "instructions[0].content",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{
					{Source: expectedSourceSecret, Content: actualInstructionSecret},
					{Source: expectedSecondSourceSecret, Content: expectedSecondInstructionSecret},
				},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "second instruction source",
			path: "instructions[1].source",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{
					{Source: expectedSourceSecret, Content: expectedInstructionSecret},
					{Source: actualSecondSourceSecret, Content: expectedSecondInstructionSecret},
				},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "second instruction content",
			path: "instructions[1].content",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{
					{Source: expectedSourceSecret, Content: expectedInstructionSecret},
					{Source: expectedSecondSourceSecret, Content: actualSecondInstructionSecret},
				},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "instruction entries swapped",
			path: "instructions[0].source",
			actual: requestParts{
				conversation: expected.Conversation(),
				instructions: []provider.Instruction{
					{Source: expectedSecondSourceSecret, Content: expectedSecondInstructionSecret},
					{Source: expectedSourceSecret, Content: expectedInstructionSecret},
				},
				capabilities: expected.RequiredCapabilities(),
			},
		},
		{
			name: "required capabilities length",
			path: "requiredCapabilities.length",
			actual: requestParts{
				conversation: expected.Conversation(), instructions: expected.Instructions(),
			},
		},
	}
	secrets := []string{
		expectedMessageSecret,
		actualMessageSecret,
		expectedSecondMessageSecret,
		actualSecondMessageSecret,
		expectedSourceSecret,
		actualSourceSecret,
		expectedSecondSourceSecret,
		actualSecondSourceSecret,
		expectedInstructionSecret,
		actualInstructionSecret,
		expectedSecondInstructionSecret,
		actualSecondInstructionSecret,
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := requestFromParts(t, test.actual)
			upstream := newFake(t, expectResult(t, expected, result))

			panicText := capturePanic(t, func() {
				_, _ = upstream.Invoke(context.Background(), actual)
			})

			wantDiagnostic := fmt.Sprintf(
				"fake provider request 1 differed at %s",
				test.path,
			)
			if panicText != wantDiagnostic {
				t.Fatalf("panic = %q, want %q", panicText, wantDiagnostic)
			}
			assertNoSecrets(t, panicText, secrets...)

			repeatedPanic := capturePanic(t, func() {
				_, _ = upstream.Invoke(context.Background(), expected)
			})
			if repeatedPanic != panicText {
				t.Fatalf("sticky panic = %q, want original %q", repeatedPanic, panicText)
			}
			verifyErr := upstream.VerifyComplete()
			if verifyErr == nil || verifyErr.Error() != panicText {
				t.Fatalf("VerifyComplete() error = %v, want original violation", verifyErr)
			}
			assertNoSecrets(t, verifyErr.Error(), secrets...)
		})
	}
}

func TestProviderTreatsNilAndEmptyOptionalListsAsEqual(t *testing.T) {
	conversation := []provider.Message{{Role: provider.MessageRoleUser, Content: "bounded"}}
	tests := []struct {
		name                 string
		expectedInstructions []provider.Instruction
		actualInstructions   []provider.Instruction
		expectedCapabilities []provider.Capability
		actualCapabilities   []provider.Capability
	}{
		{
			name:               "nil instructions match empty instructions",
			actualInstructions: []provider.Instruction{},
		},
		{
			name:                 "empty instructions match nil instructions",
			expectedInstructions: []provider.Instruction{},
		},
		{
			name:               "nil capabilities match empty capabilities",
			actualCapabilities: []provider.Capability{},
		},
		{
			name:                 "empty capabilities match nil capabilities",
			expectedCapabilities: []provider.Capability{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := requestFromParts(t, requestParts{
				conversation: conversation,
				instructions: test.expectedInstructions,
				capabilities: test.expectedCapabilities,
			})
			actual := requestFromParts(t, requestParts{
				conversation: conversation,
				instructions: test.actualInstructions,
				capabilities: test.actualCapabilities,
			})
			upstream := newFake(t, expectResult(t, expected, newResult(t, "bounded result", nil)))

			if _, err := upstream.Invoke(context.Background(), actual); err != nil {
				t.Fatalf("Invoke() returned an error: %v", err)
			}
			if err := upstream.VerifyComplete(); err != nil {
				t.Fatalf("VerifyComplete() returned an error: %v", err)
			}
		})
	}
}

func TestProviderDoesNotNormalizeUnicodeDuringMatching(t *testing.T) {
	const (
		composed   = "caf\u00e9"
		decomposed = "cafe\u0301"
	)
	expectedParts := requestParts{
		conversation: []provider.Message{{Role: provider.MessageRoleUser, Content: composed}},
		instructions: []provider.Instruction{{Source: composed, Content: composed}},
	}
	tests := []struct {
		name   string
		path   string
		actual requestParts
	}{
		{
			name: "message content",
			path: "conversation[0].content",
			actual: requestParts{
				conversation: []provider.Message{{Role: provider.MessageRoleUser, Content: decomposed}},
				instructions: expectedParts.instructions,
			},
		},
		{
			name: "instruction source",
			path: "instructions[0].source",
			actual: requestParts{
				conversation: expectedParts.conversation,
				instructions: []provider.Instruction{{Source: decomposed, Content: composed}},
			},
		},
		{
			name: "instruction content",
			path: "instructions[0].content",
			actual: requestParts{
				conversation: expectedParts.conversation,
				instructions: []provider.Instruction{{Source: composed, Content: decomposed}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			expected := requestFromParts(t, expectedParts)
			actual := requestFromParts(t, test.actual)
			upstream := newFake(t, expectResult(t, expected, newResult(t, "bounded", nil)))

			panicText := capturePanic(t, func() {
				_, _ = upstream.Invoke(context.Background(), actual)
			})

			wantDiagnostic := "fake provider request 1 differed at " + test.path
			if panicText != wantDiagnostic {
				t.Fatalf("panic = %q, want %q", panicText, wantDiagnostic)
			}
			assertNoSecrets(t, panicText, composed, decomposed)
		})
	}
}

func TestProviderExtraInvocationIsSticky(t *testing.T) {
	t.Run("after consumed script", func(t *testing.T) {
		secrets := []string{
			"EXTRA-CALL-MESSAGE-SECRET",
			"EXTRA-CALL-SOURCE-SECRET",
			"EXTRA-CALL-INSTRUCTION-SECRET",
		}
		request := newRequest(t, secrets[0], secrets[1], secrets[2], false)
		upstream := newFake(t, expectResult(t, request, newResult(t, "result", nil)))
		if _, err := upstream.Invoke(context.Background(), request); err != nil {
			t.Fatalf("first Invoke() returned an error: %v", err)
		}

		panicText := capturePanic(t, func() {
			_, _ = upstream.Invoke(context.Background(), request)
		})
		if !strings.Contains(panicText, "unexpected request 2") {
			t.Fatalf("panic = %q, want second-request diagnostic", panicText)
		}
		assertNoSecrets(t, panicText, secrets...)
		if err := upstream.VerifyComplete(); err == nil || err.Error() != panicText {
			t.Fatalf("VerifyComplete() error = %v, want sticky extra-call violation", err)
		} else {
			assertNoSecrets(t, err.Error(), secrets...)
		}
	})

	t.Run("empty script proves zero dispatch", func(t *testing.T) {
		secrets := []string{
			"ZERO-DISPATCH-MESSAGE-SECRET",
			"ZERO-DISPATCH-SOURCE-SECRET",
			"ZERO-DISPATCH-INSTRUCTION-SECRET",
		}
		upstream := newFake(t)
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("empty VerifyComplete() returned an error: %v", err)
		}
		request := newRequest(t, secrets[0], secrets[1], secrets[2], false)
		panicText := capturePanic(t, func() {
			_, _ = upstream.Invoke(context.Background(), request)
		})
		if !strings.Contains(panicText, "unexpected request 1") {
			t.Fatalf("panic = %q, want first-request diagnostic", panicText)
		}
		assertNoSecrets(t, panicText, secrets...)
		if err := upstream.VerifyComplete(); err == nil || err.Error() != panicText {
			t.Fatalf("VerifyComplete() error = %v, want sticky zero-dispatch violation", err)
		} else {
			assertNoSecrets(t, err.Error(), secrets...)
		}
	})
}

func TestVerifyCompleteReportsUninvokedExchanges(t *testing.T) {
	secrets := []string{
		"FIRST-UNINVOKED-MESSAGE-SECRET",
		"FIRST-UNINVOKED-SOURCE-SECRET",
		"FIRST-UNINVOKED-INSTRUCTION-SECRET",
		"SECOND-UNINVOKED-MESSAGE-SECRET",
		"SECOND-UNINVOKED-SOURCE-SECRET",
		"SECOND-UNINVOKED-INSTRUCTION-SECRET",
	}
	first := newRequest(t, secrets[0], secrets[1], secrets[2], false)
	second := newRequest(t, secrets[3], secrets[4], secrets[5], false)
	result := newResult(t, "result", nil)

	tests := []struct {
		name      string
		invokeOne bool
		want      string
	}{
		{name: "no request invoked", want: "expected request 1, but 2 exchange(s)"},
		{name: "prefix invoked", invokeOne: true, want: "expected request 2, but 1 exchange(s)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			upstream := newFake(
				t,
				expectResult(t, first, result),
				expectResult(t, second, result),
			)
			if test.invokeOne {
				if _, err := upstream.Invoke(context.Background(), first); err != nil {
					t.Fatalf("Invoke() returned an error: %v", err)
				}
			}
			err := upstream.VerifyComplete()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("VerifyComplete() error = %v, want text %q", err, test.want)
			}
			assertNoSecrets(t, err.Error(), secrets...)
		})
	}
}

func TestExchangeAndScriptConstructionRejectInvalidValues(t *testing.T) {
	request := newRequest(t, "bounded", "source", "instruction", false)
	result := newResult(t, "bounded", nil)
	validExchange := expectResult(t, request, result)
	if fake.MaxExchanges != 64 {
		t.Fatalf("MaxExchanges = %d, want reviewed bound 64", fake.MaxExchanges)
	}

	if _, err := fake.ExpectResult(provider.Request{}, result); err == nil {
		t.Fatal("ExpectResult() accepted a zero request")
	}
	if _, err := fake.ExpectResult(request, provider.Result{}); err == nil {
		t.Fatal("ExpectResult() accepted a zero result")
	}
	if _, err := fake.ExpectFailure(request, nil); err == nil {
		t.Fatal("ExpectFailure() accepted a nil failure")
	}
	if _, err := fake.ExpectFailure(request, &provider.Failure{}); err == nil {
		t.Fatal("ExpectFailure() accepted a zero failure")
	}
	if _, err := fake.New(fake.Exchange{}); err == nil {
		t.Fatal("New() accepted a zero exchange")
	}
	oversized := make([]fake.Exchange, fake.MaxExchanges+1)
	for index := range oversized {
		oversized[index] = validExchange
	}
	if _, err := fake.New(oversized...); err == nil {
		t.Fatal("New() accepted an oversized valid script")
	} else if err.Error() != "fake provider script must contain at most 64 exchanges" {
		t.Fatalf("New() oversized error = %q, want exact bound diagnostic", err)
	}
	exactMaximum := make([]fake.Exchange, fake.MaxExchanges)
	for index := range exactMaximum {
		exactMaximum[index] = validExchange
	}
	upstream, err := fake.New(exactMaximum...)
	if err != nil {
		t.Fatalf("New() rejected an exact-maximum script: %v", err)
	}
	if upstream == nil {
		t.Fatal("New() returned a nil provider for an exact-maximum script")
	}
	for index := range exactMaximum {
		if _, err := upstream.Invoke(context.Background(), request); err != nil {
			t.Fatalf("Invoke() exchange %d returned an error: %v", index+1, err)
		}
	}
	if err := upstream.VerifyComplete(); err != nil {
		t.Fatalf("VerifyComplete() after exact-maximum script: %v", err)
	}
}

func TestProviderContextBoundaryDoesNotAddCancellationBehavior(t *testing.T) {
	secrets := []string{
		"CONTEXT-MESSAGE-SECRET",
		"CONTEXT-SOURCE-SECRET",
		"CONTEXT-INSTRUCTION-SECRET",
	}
	request := newRequest(t, secrets[0], secrets[1], secrets[2], false)
	result := newResult(t, "bounded result", nil)

	t.Run("already canceled context keeps scripted outcome", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		upstream := newFake(t, expectResult(t, request, result))

		gotResult, gotErr := upstream.Invoke(ctx, request)

		if gotErr != nil || gotResult.OutputText() != result.OutputText() {
			t.Fatalf("Invoke() = (%q, %v), want scripted result", gotResult.OutputText(), gotErr)
		}
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("VerifyComplete() returned an error: %v", err)
		}
	})

	t.Run("already canceled context keeps scripted failure", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		failure := newFailure(t, provider.FailureUnavailable, true, nil)
		upstream := newFake(t, expectFailure(t, request, failure))

		gotResult, gotErr := upstream.Invoke(ctx, request)

		if gotResult != (provider.Result{}) || gotErr != failure {
			t.Fatalf("Invoke() = (%+v, %v), want zero result and scripted failure", gotResult, gotErr)
		}
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("VerifyComplete() returned an error: %v", err)
		}
	})

	t.Run("reported deadline keeps scripted outcome", func(t *testing.T) {
		ctx := newTerminatedContext(context.DeadlineExceeded)
		upstream := newFake(t, expectResult(t, request, result))

		gotResult, gotErr := upstream.Invoke(ctx, request)

		if gotErr != nil || gotResult.OutputText() != result.OutputText() {
			t.Fatalf("Invoke() = (%q, %v), want scripted result", gotResult.OutputText(), gotErr)
		}
		if err := upstream.VerifyComplete(); err != nil {
			t.Fatalf("VerifyComplete() returned an error: %v", err)
		}
	})

	t.Run("nil context is sticky test violation", func(t *testing.T) {
		upstream := newFake(t, expectResult(t, request, result))
		panicText := capturePanic(t, func() {
			_, _ = upstream.Invoke(nil, request)
		})
		const wantDiagnostic = "fake provider invocation context is required"
		if panicText != wantDiagnostic {
			t.Fatalf("panic = %q, want %q", panicText, wantDiagnostic)
		}
		assertNoSecrets(t, panicText, secrets...)
		if err := upstream.VerifyComplete(); err == nil || err.Error() != panicText {
			t.Fatalf("VerifyComplete() error = %v, want sticky context violation", err)
		} else {
			assertNoSecrets(t, err.Error(), secrets...)
		}
	})
}

type terminatedContext struct {
	err  error
	done <-chan struct{}
}

func newTerminatedContext(err error) context.Context {
	done := make(chan struct{})
	close(done)
	return terminatedContext{err: err, done: done}
}

func (ctx terminatedContext) Deadline() (time.Time, bool) {
	return time.Unix(0, 0), true
}

func (ctx terminatedContext) Done() <-chan struct{} {
	return ctx.done
}

func (ctx terminatedContext) Err() error {
	return ctx.err
}

func (terminatedContext) Value(any) any {
	return nil
}

func TestRequestFieldInventoryLocksExactMatcher(t *testing.T) {
	assertExactFieldInventory(t, reflect.TypeOf(provider.Request{}), map[string]reflect.Type{
		"conversation":         reflect.TypeFor[[]provider.Message](),
		"instructions":         reflect.TypeFor[[]provider.Instruction](),
		"requiredCapabilities": reflect.TypeFor[[]provider.Capability](),
	})
	assertExactFieldInventory(t, reflect.TypeOf(provider.Message{}), map[string]reflect.Type{
		"Role":    reflect.TypeFor[provider.MessageRole](),
		"Content": reflect.TypeFor[string](),
	})
	assertExactFieldInventory(t, reflect.TypeOf(provider.Instruction{}), map[string]reflect.Type{
		"Source":  reflect.TypeFor[string](),
		"Content": reflect.TypeFor[string](),
	})

	// The current Request constructor admits only CapabilityToolCalls and at most one capability.
	// Length matching covers every reachable capability state. Add a value-mismatch case when the
	// reviewed capability vocabulary expands.
}

func assertExactFieldInventory(
	t *testing.T,
	valueType reflect.Type,
	wantFields map[string]reflect.Type,
) {
	t.Helper()
	if valueType.NumField() != len(wantFields) {
		t.Fatalf(
			"provider.%s has %d fields, fake matcher covers %d",
			valueType.Name(),
			valueType.NumField(),
			len(wantFields),
		)
	}
	for index := 0; index < valueType.NumField(); index++ {
		field := valueType.Field(index)
		wantType, exists := wantFields[field.Name]
		if !exists || field.Type != wantType {
			t.Fatalf(
				"provider.%s field %q has type %v; update the exact fake matcher",
				valueType.Name(),
				field.Name,
				field.Type,
			)
		}
	}
}

type requestParts struct {
	conversation []provider.Message
	instructions []provider.Instruction
	capabilities []provider.Capability
}

func newRequest(
	t *testing.T,
	message string,
	source string,
	instruction string,
	requireTools bool,
) provider.Request {
	t.Helper()
	parts := requestParts{
		conversation: []provider.Message{{Role: provider.MessageRoleUser, Content: message}},
		instructions: []provider.Instruction{{Source: source, Content: instruction}},
	}
	if requireTools {
		parts.capabilities = []provider.Capability{provider.CapabilityToolCalls}
	}
	return requestFromParts(t, parts)
}

func requestFromParts(t *testing.T, parts requestParts) provider.Request {
	t.Helper()
	request, err := provider.NewRequest(parts.conversation, parts.instructions, parts.capabilities)
	if err != nil {
		t.Fatalf("NewRequest() returned an error: %v", err)
	}
	return request
}

func newResult(t *testing.T, output string, usage *provider.Usage) provider.Result {
	t.Helper()
	result, err := provider.NewResult(output, usage)
	if err != nil {
		t.Fatalf("NewResult() returned an error: %v", err)
	}
	return result
}

func newFailure(
	t *testing.T,
	code provider.FailureCode,
	retryable bool,
	usage *provider.Usage,
) *provider.Failure {
	t.Helper()
	failure, err := provider.NewFailure(code, retryable, usage)
	if err != nil {
		t.Fatalf("NewFailure() returned an error: %v", err)
	}
	return failure
}

func expectResult(
	t *testing.T,
	request provider.Request,
	result provider.Result,
) fake.Exchange {
	t.Helper()
	exchange, err := fake.ExpectResult(request, result)
	if err != nil {
		t.Fatalf("ExpectResult() returned an error: %v", err)
	}
	return exchange
}

func expectFailure(
	t *testing.T,
	request provider.Request,
	failure *provider.Failure,
) fake.Exchange {
	t.Helper()
	exchange, err := fake.ExpectFailure(request, failure)
	if err != nil {
		t.Fatalf("ExpectFailure() returned an error: %v", err)
	}
	return exchange
}

func newFake(t *testing.T, exchanges ...fake.Exchange) *fake.Provider {
	t.Helper()
	upstream, err := fake.New(exchanges...)
	if err != nil {
		t.Fatalf("fake.New() returned an error: %v", err)
	}
	return upstream
}

func capturePanic(t *testing.T, action func()) string {
	t.Helper()
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		action()
	}()
	if recovered == nil {
		t.Fatal("action did not panic")
	}
	switch value := recovered.(type) {
	case error:
		return value.Error()
	case string:
		return value
	default:
		return fmt.Sprintf("panic type %T", recovered)
	}
}

func assertNoSecrets(t *testing.T, diagnostic string, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		if strings.Contains(diagnostic, secret) {
			t.Fatalf("diagnostic exposed sentinel content: %q", diagnostic)
		}
	}
}
