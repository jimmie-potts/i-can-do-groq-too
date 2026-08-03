package provider_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

type invokerFunc func(context.Context, provider.Request) (provider.Result, error)

func (function invokerFunc) Invoke(
	ctx context.Context,
	request provider.Request,
) (provider.Result, error) {
	return function(ctx, request)
}

var _ provider.Invoker = invokerFunc(nil)

type traversalRecordingError struct {
	errorCalls  int
	unwrapCalls int
	asCalls     int
}

func (recording *traversalRecordingError) Error() string {
	recording.errorCalls++
	return "raw provider error"
}

func (recording *traversalRecordingError) Unwrap() error {
	recording.unwrapCalls++
	return context.Canceled
}

func (recording *traversalRecordingError) As(any) bool {
	recording.asCalls++
	return true
}

func TestRequestPreservesOrderAndOwnsItsSlices(t *testing.T) {
	conversation := []provider.Message{
		{Role: provider.MessageRoleUser, Content: "first"},
		{Role: provider.MessageRoleAssistant, Content: "second"},
	}
	instructions := []provider.Instruction{
		{Source: "first-source", Content: "first instruction"},
		{Source: "second-source", Content: "second instruction"},
	}
	capabilities := []provider.Capability{provider.CapabilityToolCalls}

	request, err := provider.NewRequest(conversation, instructions, capabilities)
	if err != nil {
		t.Fatalf("NewRequest() returned an error: %v", err)
	}

	conversation[0].Content = "mutated input"
	instructions[0].Content = "mutated input"
	capabilities[0] = "mutated_input"

	gotConversation := request.Conversation()
	gotInstructions := request.Instructions()
	gotCapabilities := request.RequiredCapabilities()
	if gotConversation[0].Content != "first" || gotConversation[1].Content != "second" {
		t.Fatalf("Conversation() = %#v, want original ordered values", gotConversation)
	}
	if gotInstructions[0].Content != "first instruction" ||
		gotInstructions[1].Content != "second instruction" {
		t.Fatalf("Instructions() = %#v, want original ordered values", gotInstructions)
	}
	if !slices.Equal(gotCapabilities, []provider.Capability{provider.CapabilityToolCalls}) {
		t.Fatalf("RequiredCapabilities() = %#v, want tool_calls", gotCapabilities)
	}

	gotConversation[0].Content = "mutated output"
	gotInstructions[0].Content = "mutated output"
	gotCapabilities[0] = "mutated_output"
	if request.Conversation()[0].Content != "first" {
		t.Fatal("mutating Conversation() output changed the request")
	}
	if request.Instructions()[0].Content != "first instruction" {
		t.Fatal("mutating Instructions() output changed the request")
	}
	if request.RequiredCapabilities()[0] != provider.CapabilityToolCalls {
		t.Fatal("mutating RequiredCapabilities() output changed the request")
	}
}

func TestNewRequestValidatesEveryBoundedField(t *testing.T) {
	tests := []struct {
		name   string
		change func(*requestParts)
		want   string
	}{
		{
			name: "empty conversation",
			change: func(parts *requestParts) {
				parts.conversation = nil
			},
			want: "conversation must contain between",
		},
		{
			name: "too many conversation messages",
			change: func(parts *requestParts) {
				parts.conversation = make(
					[]provider.Message,
					provider.MaxConversationMessages+1,
				)
				for index := range parts.conversation {
					parts.conversation[index] = provider.Message{
						Role: provider.MessageRoleUser, Content: "bounded",
					}
				}
			},
			want: "conversation must contain between",
		},
		{
			name: "unsupported role",
			change: func(parts *requestParts) {
				parts.conversation[0].Role = "system"
			},
			want: "conversation[0].role is unsupported",
		},
		{
			name: "empty message",
			change: func(parts *requestParts) {
				parts.conversation[0].Content = ""
			},
			want: "conversation[0].content must contain between",
		},
		{
			name: "message above scalar limit",
			change: func(parts *requestParts) {
				parts.conversation[0].Content = strings.Repeat(
					"😀",
					provider.MaxMessageTextRunes+1,
				)
			},
			want: "conversation[0].content must contain between",
		},
		{
			name: "message invalid UTF-8",
			change: func(parts *requestParts) {
				parts.conversation[0].Content = string([]byte{0xff})
			},
			want: "conversation[0].content must be valid UTF-8",
		},
		{
			name: "too many instructions",
			change: func(parts *requestParts) {
				parts.instructions = make([]provider.Instruction, provider.MaxInstructions+1)
				for index := range parts.instructions {
					parts.instructions[index] = provider.Instruction{Source: "source", Content: "text"}
				}
			},
			want: "instructions must contain at most",
		},
		{
			name: "empty instruction source",
			change: func(parts *requestParts) {
				parts.instructions[0].Source = ""
			},
			want: "instructions[0].source must contain between",
		},
		{
			name: "instruction source above scalar limit",
			change: func(parts *requestParts) {
				parts.instructions[0].Source = strings.Repeat(
					"界",
					provider.MaxInstructionSourceRunes+1,
				)
			},
			want: "instructions[0].source must contain between",
		},
		{
			name: "unsafe instruction source",
			change: func(parts *requestParts) {
				parts.instructions[0].Source = "unsafe\nsource"
			},
			want: "instructions[0].source must not contain controls",
		},
		{
			name: "instruction source invalid UTF-8",
			change: func(parts *requestParts) {
				parts.instructions[0].Source = string([]byte{0xff})
			},
			want: "instructions[0].source must be valid UTF-8",
		},
		{
			name: "empty instruction content",
			change: func(parts *requestParts) {
				parts.instructions[0].Content = ""
			},
			want: "instructions[0].content must contain between",
		},
		{
			name: "instruction content invalid UTF-8",
			change: func(parts *requestParts) {
				parts.instructions[0].Content = string([]byte{0xff})
			},
			want: "instructions[0].content must be valid UTF-8",
		},
		{
			name: "instruction content above scalar limit",
			change: func(parts *requestParts) {
				parts.instructions[0].Content = strings.Repeat(
					"x",
					provider.MaxInstructionTextRunes+1,
				)
			},
			want: "instructions[0].content must contain between",
		},
		{
			name: "too many capabilities",
			change: func(parts *requestParts) {
				parts.capabilities = []provider.Capability{
					provider.CapabilityToolCalls,
					provider.CapabilityToolCalls,
				}
			},
			want: "required capabilities must contain at most",
		},
		{
			name: "unsupported capability",
			change: func(parts *requestParts) {
				parts.capabilities = []provider.Capability{"provider_specific_feature"}
			},
			want: "required capabilities[0] is unsupported",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parts := validRequestParts()
			test.change(&parts)

			_, err := provider.NewRequest(
				parts.conversation,
				parts.instructions,
				parts.capabilities,
			)

			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewRequest() error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestInstructionSourceRejectsOnlyReviewedControlRanges(t *testing.T) {
	forbidden := make([]rune, 0, 67)
	for character := rune(0); character <= '\u001f'; character++ {
		forbidden = append(forbidden, character)
	}
	for character := rune('\u007f'); character <= '\u009f'; character++ {
		forbidden = append(forbidden, character)
	}
	forbidden = append(forbidden, '\u2028', '\u2029')
	for _, character := range forbidden {
		t.Run(fmt.Sprintf("reject U+%04X", character), func(t *testing.T) {
			parts := validRequestParts()
			parts.instructions[0].Source = "safe" + string(character)
			if _, err := provider.NewRequest(
				parts.conversation,
				parts.instructions,
				parts.capabilities,
			); err == nil {
				t.Fatalf("NewRequest() accepted U+%04X", character)
			}
		})
	}

	for _, character := range []rune{
		'\u0020', '\u007e', '\u00a0', '\u2027', '\u202a',
	} {
		t.Run(fmt.Sprintf("allow U+%04X", character), func(t *testing.T) {
			parts := validRequestParts()
			parts.instructions[0].Source = "safe" + string(character)
			if _, err := provider.NewRequest(
				parts.conversation,
				parts.instructions,
				parts.capabilities,
			); err != nil {
				t.Fatalf("NewRequest() rejected adjacent scalar U+%04X: %v", character, err)
			}
		})
	}
}

func TestRequestBoundsCountUnicodeScalarsRatherThanBytes(t *testing.T) {
	parts := validRequestParts()
	parts.conversation[0].Content = strings.Repeat("😀", provider.MaxMessageTextRunes)
	parts.instructions[0].Source = strings.Repeat("界", provider.MaxInstructionSourceRunes)
	parts.instructions[0].Content = strings.Repeat("😀", provider.MaxInstructionTextRunes)

	if _, err := provider.NewRequest(
		parts.conversation,
		parts.instructions,
		parts.capabilities,
	); err != nil {
		t.Fatalf("NewRequest() rejected exact Unicode scalar bounds: %v", err)
	}
}

func TestRequestAcceptsExactCardinalitiesAndEmptyOptionalLists(t *testing.T) {
	conversation := make([]provider.Message, provider.MaxConversationMessages)
	for index := range conversation {
		conversation[index] = provider.Message{Role: provider.MessageRoleUser, Content: "bounded"}
	}
	instructions := make([]provider.Instruction, provider.MaxInstructions)
	for index := range instructions {
		instructions[index] = provider.Instruction{Source: "source", Content: "bounded"}
	}

	if _, err := provider.NewRequest(conversation, instructions, nil); err != nil {
		t.Fatalf("NewRequest() rejected exact cardinality bounds: %v", err)
	}
	if _, err := provider.NewRequest(conversation[:1], nil, []provider.Capability{}); err != nil {
		t.Fatalf("NewRequest() rejected empty optional lists: %v", err)
	}
}

func TestToolCallsIsValidRequirementDataWithoutPerformingAdmission(t *testing.T) {
	parts := validRequestParts()
	request, err := provider.NewRequest(
		parts.conversation,
		parts.instructions,
		[]provider.Capability{provider.CapabilityToolCalls},
	)
	if err != nil {
		t.Fatalf("NewRequest() rejected provider-neutral requirement data: %v", err)
	}
	if !slices.Equal(
		request.RequiredCapabilities(),
		[]provider.Capability{provider.CapabilityToolCalls},
	) {
		t.Fatal("NewRequest() did not preserve tool_calls requirement data")
	}
}

func TestContentFieldsAllowControlsSelectedByTheSchema(t *testing.T) {
	parts := validRequestParts()
	parts.conversation[0].Content = "line one\nline two"
	parts.instructions[0].Content = "tab\tcontent"
	request, err := provider.NewRequest(parts.conversation, parts.instructions, parts.capabilities)
	if err != nil {
		t.Fatalf("NewRequest() rejected controls in content fields: %v", err)
	}
	result, err := provider.NewResult("line one\nline two", nil)
	if err != nil {
		t.Fatalf("NewResult() rejected controls in output text: %v", err)
	}
	if request.Conversation()[0].Content != "line one\nline two" ||
		request.Instructions()[0].Content != "tab\tcontent" ||
		result.OutputText() != "line one\nline two" {
		t.Fatal("content or output text was not preserved exactly")
	}
}

func TestTextValidationRejectsUTF8EncodedSurrogate(t *testing.T) {
	encodedSurrogate := string([]byte{0xed, 0xa0, 0x80})
	parts := validRequestParts()
	parts.conversation[0].Content = encodedSurrogate
	if _, err := provider.NewRequest(
		parts.conversation,
		parts.instructions,
		parts.capabilities,
	); err == nil {
		t.Fatal("NewRequest() accepted UTF-8 bytes encoding a surrogate")
	}
	if _, err := provider.NewResult(encodedSurrogate, nil); err == nil {
		t.Fatal("NewResult() accepted UTF-8 bytes encoding a surrogate")
	}
}

func TestRequestValidationDiagnosticsDoNotEchoContent(t *testing.T) {
	const sentinel = "DO-NOT-PRINT-THIS-SECRET"
	parts := validRequestParts()
	parts.instructions[0].Source = sentinel + "\n"

	_, err := provider.NewRequest(parts.conversation, parts.instructions, parts.capabilities)

	if err == nil {
		t.Fatal("NewRequest() accepted an unsafe instruction source")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("NewRequest() error exposed sentinel content: %q", err)
	}
}

func TestResultPreservesOptionalUsageAndOwnsItsInput(t *testing.T) {
	tests := []struct {
		name      string
		usage     *provider.Usage
		wantUsage provider.Usage
		wantSeen  bool
	}{
		{name: "absent"},
		{
			name:      "observed zero",
			usage:     &provider.Usage{},
			wantUsage: provider.Usage{},
			wantSeen:  true,
		},
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
			result, err := provider.NewResult("bounded output", test.usage)
			if err != nil {
				t.Fatalf("NewResult() returned an error: %v", err)
			}
			if test.usage != nil {
				test.usage.InputTokens = 99
				test.usage.OutputTokens = 99
			}

			gotUsage, gotSeen := result.Usage()
			if gotUsage != test.wantUsage || gotSeen != test.wantSeen {
				t.Fatalf(
					"Usage() = (%+v, %t), want (%+v, %t)",
					gotUsage,
					gotSeen,
					test.wantUsage,
					test.wantSeen,
				)
			}
		})
	}
}

func TestNewResultValidatesTextAndUsageBounds(t *testing.T) {
	tests := []struct {
		name       string
		outputText string
		usage      *provider.Usage
		want       string
	}{
		{name: "empty output", want: "output text must contain between"},
		{
			name:       "output above scalar limit",
			outputText: strings.Repeat("😀", provider.MaxOutputTextRunes+1),
			want:       "output text must contain between",
		},
		{
			name:       "output invalid UTF-8",
			outputText: string([]byte{0xff}),
			want:       "output text must be valid UTF-8",
		},
		{
			name:       "negative input usage",
			outputText: "bounded",
			usage:      &provider.Usage{InputTokens: -1},
			want:       "input token usage must be between",
		},
		{
			name:       "input usage above maximum",
			outputText: "bounded",
			usage:      &provider.Usage{InputTokens: provider.MaxUsageTokens + 1},
			want:       "input token usage must be between",
		},
		{
			name:       "negative output usage",
			outputText: "bounded",
			usage:      &provider.Usage{OutputTokens: -1},
			want:       "output token usage must be between",
		},
		{
			name:       "output usage above maximum",
			outputText: "bounded",
			usage:      &provider.Usage{OutputTokens: provider.MaxUsageTokens + 1},
			want:       "output token usage must be between",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := provider.NewResult(test.outputText, test.usage)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("NewResult() error = %v, want text %q", err, test.want)
			}
		})
	}

	if _, err := provider.NewResult(
		strings.Repeat("😀", provider.MaxOutputTextRunes),
		nil,
	); err != nil {
		t.Fatalf("NewResult() rejected exact output scalar bound: %v", err)
	}
}

func TestFailureCodesAreProviderOwnedAndContentFree(t *testing.T) {
	providerCodes := []provider.FailureCode{
		provider.FailureAuthenticationFailed,
		provider.FailureRateLimited,
		provider.FailureRequestRejected,
		provider.FailureUnavailable,
		provider.FailureInvalidResponse,
		provider.FailureUnsupportedUpstreamOutput,
		provider.FailureInternal,
	}
	for _, code := range providerCodes {
		t.Run(string(code), func(t *testing.T) {
			failure, err := provider.NewFailure(code, true, nil)
			if err != nil {
				t.Fatalf("NewFailure() returned an error: %v", err)
			}
			if failure.Code() != code || !failure.Retryable() {
				t.Fatalf("failure = (%q, %t), want (%q, true)", failure.Code(), failure.Retryable(), code)
			}
			if failure.Error() != "provider invocation failed" {
				t.Fatalf("Error() = %q, want fixed content-free text", failure.Error())
			}
		})
	}

	for _, code := range []provider.FailureCode{
		"invalid_request",
		"unsupported_capability",
		"vendor_specific_failure",
	} {
		t.Run("reject "+string(code), func(t *testing.T) {
			if _, err := provider.NewFailure(code, false, nil); err == nil {
				t.Fatalf("NewFailure(%q) returned no error", code)
			}
		})
	}

	failureType := reflect.TypeOf(provider.Failure{})
	wantFields := map[string]reflect.Type{
		"code":      reflect.TypeFor[provider.FailureCode](),
		"retryable": reflect.TypeFor[bool](),
		"usage":     reflect.TypeFor[provider.Usage](),
		"hasUsage":  reflect.TypeFor[bool](),
	}
	if failureType.NumField() != len(wantFields) {
		t.Fatalf("Failure has %d fields, want exact safe inventory of %d", failureType.NumField(), len(wantFields))
	}
	for index := 0; index < failureType.NumField(); index++ {
		field := failureType.Field(index)
		wantType, exists := wantFields[field.Name]
		if !exists || field.Type != wantType {
			t.Fatalf(
				"Failure field %q has type %v, want exact safe inventory",
				field.Name,
				field.Type,
			)
		}
	}
	if _, exists := reflect.TypeFor[*provider.Failure]().MethodByName("Unwrap"); exists {
		t.Fatal("Failure exposes a wrapped raw error")
	}
}

func TestValidateInvocationRejectsRawErrorWithoutTraversal(t *testing.T) {
	raw := &traversalRecordingError{}

	if err := provider.ValidateInvocation(context.Background(), provider.Result{}, raw); err == nil {
		t.Fatal("ValidateInvocation() accepted a raw provider error")
	}
	if raw.errorCalls != 0 || raw.unwrapCalls != 0 || raw.asCalls != 0 {
		t.Fatalf(
			"ValidateInvocation() traversed raw error: Error=%d Unwrap=%d As=%d",
			raw.errorCalls,
			raw.unwrapCalls,
			raw.asCalls,
		)
	}
}

func TestFailurePreservesOptionalUsage(t *testing.T) {
	tests := []struct {
		name      string
		usage     *provider.Usage
		wantUsage provider.Usage
		wantSeen  bool
		wantError string
	}{
		{name: "absent"},
		{
			name:      "observed zero",
			usage:     &provider.Usage{},
			wantUsage: provider.Usage{},
			wantSeen:  true,
		},
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
		{
			name:      "negative",
			usage:     &provider.Usage{InputTokens: -1},
			wantError: "input token usage must be between",
		},
		{
			name:      "negative output",
			usage:     &provider.Usage{OutputTokens: -1},
			wantError: "output token usage must be between",
		},
		{
			name: "input above maximum",
			usage: &provider.Usage{
				InputTokens: provider.MaxUsageTokens + 1,
			},
			wantError: "input token usage must be between",
		},
		{
			name: "above maximum",
			usage: &provider.Usage{
				OutputTokens: provider.MaxUsageTokens + 1,
			},
			wantError: "output token usage must be between",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			failure, err := provider.NewFailure(provider.FailureUnavailable, false, test.usage)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("NewFailure() error = %v, want text %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewFailure() returned an error: %v", err)
			}
			if test.usage != nil {
				test.usage.InputTokens = 99
				test.usage.OutputTokens = 99
			}
			gotUsage, gotSeen := failure.Usage()
			if gotUsage != test.wantUsage || gotSeen != test.wantSeen {
				t.Fatalf(
					"Usage() = (%+v, %t), want (%+v, %t)",
					gotUsage,
					gotSeen,
					test.wantUsage,
					test.wantSeen,
				)
			}
		})
	}
}

func TestValidateInvocationAllowsOnlyOneReviewedAlternative(t *testing.T) {
	result, err := provider.NewResult("bounded", nil)
	if err != nil {
		t.Fatalf("NewResult() returned an error: %v", err)
	}
	failure, err := provider.NewFailure(provider.FailureUnavailable, false, nil)
	if err != nil {
		t.Fatalf("NewFailure() returned an error: %v", err)
	}
	secretError := errors.New("raw provider error: DO-NOT-PRINT-THIS-SECRET")

	active := context.Background()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	deadline, stopDeadline := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stopDeadline()
	caused, cancelCause := context.WithCancelCause(context.Background())
	cancelCause(errors.New("caller-specific cause"))

	tests := []struct {
		name    string
		ctx     context.Context
		result  provider.Result
		err     error
		wantErr bool
	}{
		{name: "completed", ctx: active, result: result},
		{name: "normalized failure", ctx: active, err: failure},
		{name: "caller canceled", ctx: canceled, err: context.Canceled},
		{name: "caller deadline", ctx: deadline, err: context.DeadlineExceeded},
		{name: "caller canceled with custom cause", ctx: caused, err: context.Canceled},
		{name: "fabricated cancellation", ctx: active, err: context.Canceled, wantErr: true},
		{name: "wrong context termination", ctx: canceled, err: context.DeadlineExceeded, wantErr: true},
		{name: "nil context", result: result, wantErr: true},
		{name: "missing result and error", ctx: active, wantErr: true},
		{name: "result and failure", ctx: active, result: result, err: failure, wantErr: true},
		{name: "result and cancellation", ctx: canceled, result: result, err: context.Canceled, wantErr: true},
		{name: "raw provider error", ctx: active, err: secretError, wantErr: true},
		{name: "wrapped normalized failure", ctx: active, err: fmt.Errorf("unsafe wrapper: %w", failure), wantErr: true},
		{name: "wrapped cancellation", ctx: canceled, err: fmt.Errorf("unsafe wrapper: %w", context.Canceled), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := provider.ValidateInvocation(test.ctx, test.result, test.err)
			if test.wantErr && err == nil {
				t.Fatal("ValidateInvocation() returned no error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("ValidateInvocation() returned an error: %v", err)
			}
			if err != nil && strings.Contains(err.Error(), "DO-NOT-PRINT-THIS-SECRET") {
				t.Fatalf("ValidateInvocation() copied raw error content: %q", err)
			}
		})
	}

	var nilFailure *provider.Failure
	if err := provider.ValidateInvocation(active, provider.Result{}, nilFailure); err == nil {
		t.Fatal("ValidateInvocation() accepted a typed nil failure")
	}
}

func TestInvokerReceivesCallerContextAndValidatedRequest(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "caller-owned")
	parts := validRequestParts()
	request, err := provider.NewRequest(parts.conversation, parts.instructions, parts.capabilities)
	if err != nil {
		t.Fatalf("NewRequest() returned an error: %v", err)
	}
	result, err := provider.NewResult("controlled result", nil)
	if err != nil {
		t.Fatalf("NewResult() returned an error: %v", err)
	}

	invoker := invokerFunc(func(gotCtx context.Context, gotRequest provider.Request) (provider.Result, error) {
		if gotCtx.Value(contextKey{}) != "caller-owned" {
			t.Error("Invoke() did not receive the caller context")
		}
		if !reflect.DeepEqual(gotRequest.Conversation(), request.Conversation()) {
			t.Error("Invoke() did not receive the validated request")
		}
		return result, nil
	})

	gotResult, invokeErr := invoker.Invoke(ctx, request)
	if err := provider.ValidateInvocation(ctx, gotResult, invokeErr); err != nil {
		t.Fatalf("ValidateInvocation() returned an error: %v", err)
	}
}

func TestProviderBoundsMatchModelTurnV1Schemas(t *testing.T) {
	request := loadSchema(t, "request.schema.json")
	conversation := child(t, request, "conversation")
	message := item(t, conversation)
	assertInteger(t, "conversation.minItems", conversation.MinItems, 1)
	assertInteger(
		t,
		"conversation.maxItems",
		conversation.MaxItems,
		provider.MaxConversationMessages,
	)
	assertStringSet(
		t,
		"conversation.items.role.enum",
		child(t, message, "role").Enum,
		[]string{string(provider.MessageRoleUser), string(provider.MessageRoleAssistant)},
	)
	assertTextBounds(
		t,
		"conversation.items.content",
		child(t, message, "content"),
		1,
		provider.MaxMessageTextRunes,
	)

	instructions := child(t, request, "instructions")
	instruction := item(t, instructions)
	assertInteger(t, "instructions.minItems", instructions.MinItems, 0)
	assertInteger(t, "instructions.maxItems", instructions.MaxItems, provider.MaxInstructions)
	assertTextBounds(
		t,
		"instructions.items.source",
		child(t, instruction, "source"),
		1,
		provider.MaxInstructionSourceRunes,
	)
	if got, want := child(t, instruction, "source").Pattern,
		`^[^\u0000-\u001F\u007F-\u009F\u2028\u2029]+(?![\s\S])`; got != want {
		t.Errorf("instructions.items.source.pattern = %q, want %q", got, want)
	}
	assertTextBounds(
		t,
		"instructions.items.content",
		child(t, instruction, "content"),
		1,
		provider.MaxInstructionTextRunes,
	)

	capabilities := child(t, request, "required_capabilities")
	assertInteger(t, "required_capabilities.minItems", capabilities.MinItems, 0)
	assertInteger(
		t,
		"required_capabilities.maxItems",
		capabilities.MaxItems,
		provider.MaxRequiredCapabilities,
	)
	assertStringSet(
		t,
		"required_capabilities.items.enum",
		item(t, capabilities).Enum,
		[]string{string(provider.CapabilityToolCalls)},
	)

	success := loadSchema(t, "success.schema.json")
	assertTextBounds(
		t,
		"success.output_text",
		child(t, success, "output_text"),
		1,
		provider.MaxOutputTextRunes,
	)
	assertUsageSchema(t, "success.usage", child(t, success, "usage"))

	failure := loadSchema(t, "failure.schema.json")
	assertUsageSchema(t, "failure.usage", child(t, failure, "usage"))
	assertOptionalProperty(t, "success.usage", success, "usage")
	assertOptionalProperty(t, "failure.usage", failure, "usage")
	providerCodes := []string{
		string(provider.FailureAuthenticationFailed),
		string(provider.FailureRateLimited),
		string(provider.FailureRequestRejected),
		string(provider.FailureUnavailable),
		string(provider.FailureInvalidResponse),
		string(provider.FailureUnsupportedUpstreamOutput),
		string(provider.FailureInternal),
	}
	wireCodes := child(t, child(t, failure, "error"), "code").Enum
	allWireCodes := append(slices.Clone(providerCodes), "invalid_request", "unsupported_capability")
	assertStringSet(
		t,
		"failure error code enum",
		wireCodes,
		allWireCodes,
	)
}

type requestParts struct {
	conversation []provider.Message
	instructions []provider.Instruction
	capabilities []provider.Capability
}

func validRequestParts() requestParts {
	return requestParts{
		conversation: []provider.Message{
			{Role: provider.MessageRoleUser, Content: "review this bounded request"},
		},
		instructions: []provider.Instruction{
			{Source: "AGENTS.md", Content: "keep provider values neutral"},
		},
		capabilities: []provider.Capability{provider.CapabilityToolCalls},
	}
}

type schemaNode struct {
	Properties map[string]*schemaNode `json:"properties"`
	Items      *schemaNode            `json:"items"`
	Enum       []string               `json:"enum"`
	Required   []string               `json:"required"`
	MinItems   int                    `json:"minItems"`
	MaxItems   int                    `json:"maxItems"`
	MinLength  int                    `json:"minLength"`
	MaxLength  int                    `json:"maxLength"`
	Minimum    int64                  `json:"minimum"`
	Maximum    int64                  `json:"maximum"`
	Pattern    string                 `json:"pattern"`
}

func assertOptionalProperty(t *testing.T, name string, parent *schemaNode, property string) {
	t.Helper()
	if slices.Contains(parent.Required, property) {
		t.Errorf("%s is required, want optional", name)
	}
}

func loadSchema(t *testing.T, name string) *schemaNode {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the provider test")
	}
	path := filepath.Join(
		filepath.Dir(currentFile),
		"..",
		"..",
		"contracts",
		"model-turn",
		"v1",
		"schema",
		name,
	)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", name, err)
	}
	var schema schemaNode
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode schema %s: %v", name, err)
	}
	return &schema
}

func child(t *testing.T, parent *schemaNode, name string) *schemaNode {
	t.Helper()
	child := parent.Properties[name]
	if child == nil {
		t.Fatalf("schema property %q is missing", name)
	}
	return child
}

func item(t *testing.T, parent *schemaNode) *schemaNode {
	t.Helper()
	if parent.Items == nil {
		t.Fatal("schema items are missing")
	}
	return parent.Items
}

func assertTextBounds(t *testing.T, name string, node *schemaNode, minimum int, maximum int) {
	t.Helper()
	assertInteger(t, name+".minLength", node.MinLength, minimum)
	assertInteger(t, name+".maxLength", node.MaxLength, maximum)
}

func assertUsageSchema(t *testing.T, name string, usage *schemaNode) {
	t.Helper()
	for _, field := range []string{"input_tokens", "output_tokens"} {
		count := child(t, usage, field)
		if count.Minimum != 0 || count.Maximum != provider.MaxUsageTokens {
			t.Errorf(
				"%s.%s bounds = (%d, %d), want (0, %d)",
				name,
				field,
				count.Minimum,
				count.Maximum,
				provider.MaxUsageTokens,
			)
		}
	}
}

func assertInteger(t *testing.T, name string, got int, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", name, got, want)
	}
}

func assertStringSet(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	got = slices.Clone(got)
	want = slices.Clone(want)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}
