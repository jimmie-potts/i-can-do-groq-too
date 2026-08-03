// Package provider defines FastGate's provider-neutral non-streaming boundary.
package provider

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"
)

const (
	// MaxConversationMessages is the model-turn v1 conversation cardinality bound.
	MaxConversationMessages = 64
	// MaxInstructions is the model-turn v1 instruction-block cardinality bound.
	MaxInstructions = 32
	// MaxRequiredCapabilities is the model-turn v1 capability cardinality bound.
	MaxRequiredCapabilities = 1
	// MaxMessageTextRunes is the model-turn v1 message-content scalar-value bound.
	MaxMessageTextRunes = 65_536
	// MaxInstructionSourceRunes is the model-turn v1 instruction-source scalar-value bound.
	MaxInstructionSourceRunes = 256
	// MaxInstructionTextRunes is the model-turn v1 instruction-content scalar-value bound.
	MaxInstructionTextRunes = 65_536
	// MaxOutputTextRunes is the model-turn v1 completed-output scalar-value bound.
	MaxOutputTextRunes = 65_536
	// MaxUsageTokens is the largest non-negative JavaScript-safe token observation.
	MaxUsageTokens int64 = 9_007_199_254_740_991
)

// MessageRole identifies the speaker for one provider-neutral conversation message.
type MessageRole string

const (
	// MessageRoleUser is caller-authored model input.
	MessageRoleUser MessageRole = "user"
	// MessageRoleAssistant is previously observed assistant output.
	MessageRoleAssistant MessageRole = "assistant"
)

// Message is one ordered provider-neutral conversation value.
type Message struct {
	Role    MessageRole
	Content string
}

// Instruction is one ordered caller-supplied instruction block.
type Instruction struct {
	Source  string
	Content string
}

// Capability is a bounded behavior required by the admitted model turn.
type Capability string

const (
	// CapabilityToolCalls names the only capability spelling in model-turn v1.
	CapabilityToolCalls Capability = "tool_calls"
)

// Request is immutable provider-facing input for one admitted model turn.
//
// Wire framing, request correlation, logical-alias selection, endpoints, credentials, and provider
// model IDs stay above or outside this boundary. Slice inputs are copied during construction and
// accessors return copies so later caller mutation cannot change the request seen by an Invoker.
type Request struct {
	conversation         []Message
	instructions         []Instruction
	requiredCapabilities []Capability
}

// NewRequest validates and takes an immutable copy of provider-facing model-turn data.
func NewRequest(
	conversation []Message,
	instructions []Instruction,
	requiredCapabilities []Capability,
) (Request, error) {
	if err := validateRequestCardinalities(
		len(conversation),
		len(instructions),
		len(requiredCapabilities),
	); err != nil {
		return Request{}, err
	}
	request := Request{
		conversation:         slices.Clone(conversation),
		instructions:         slices.Clone(instructions),
		requiredCapabilities: slices.Clone(requiredCapabilities),
	}
	if err := request.Validate(); err != nil {
		return Request{}, err
	}
	return request, nil
}

// Validate checks the complete provider-facing request contract without exposing field contents.
func (request Request) Validate() error {
	if err := validateRequestCardinalities(
		len(request.conversation),
		len(request.instructions),
		len(request.requiredCapabilities),
	); err != nil {
		return err
	}
	for index, message := range request.conversation {
		field := fmt.Sprintf("conversation[%d]", index)
		if message.Role != MessageRoleUser && message.Role != MessageRoleAssistant {
			return fmt.Errorf("%s.role is unsupported", field)
		}
		if err := validateText(
			message.Content,
			field+".content",
			1,
			MaxMessageTextRunes,
			false,
		); err != nil {
			return err
		}
	}

	for index, instruction := range request.instructions {
		field := fmt.Sprintf("instructions[%d]", index)
		if err := validateText(
			instruction.Source,
			field+".source",
			1,
			MaxInstructionSourceRunes,
			true,
		); err != nil {
			return err
		}
		if err := validateText(
			instruction.Content,
			field+".content",
			1,
			MaxInstructionTextRunes,
			false,
		); err != nil {
			return err
		}
	}

	for index, capability := range request.requiredCapabilities {
		if capability != CapabilityToolCalls {
			return fmt.Errorf("required capabilities[%d] is unsupported", index)
		}
	}
	return nil
}

// Conversation returns a copy of the ordered conversation.
func (request Request) Conversation() []Message {
	return slices.Clone(request.conversation)
}

// Instructions returns a copy of the ordered instruction blocks.
func (request Request) Instructions() []Instruction {
	return slices.Clone(request.instructions)
}

// RequiredCapabilities returns a copy of the bounded capability requirements.
func (request Request) RequiredCapabilities() []Capability {
	return slices.Clone(request.requiredCapabilities)
}

// Usage is a non-authoritative token-count observation.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// Result is one immutable non-streaming provider completion.
type Result struct {
	outputText string
	usage      Usage
	hasUsage   bool
}

// NewResult validates one non-streaming provider completion.
// A nil usage pointer means that the provider did not report usage.
func NewResult(outputText string, usage *Usage) (Result, error) {
	result := Result{outputText: outputText}
	if usage != nil {
		result.usage = *usage
		result.hasUsage = true
	}
	if err := result.Validate(); err != nil {
		return Result{}, err
	}
	return result, nil
}

// Validate checks completed output and any observed usage.
func (result Result) Validate() error {
	if err := validateText(
		result.outputText,
		"output text",
		1,
		MaxOutputTextRunes,
		false,
	); err != nil {
		return err
	}
	if result.hasUsage {
		return validateUsage(result.usage)
	}
	return nil
}

// OutputText returns the complete provider output.
func (result Result) OutputText() string {
	return result.outputText
}

// Usage returns a copied usage observation and whether one was present.
func (result Result) Usage() (Usage, bool) {
	return result.usage, result.hasUsage
}

func (result Result) isZero() bool {
	return result.outputText == "" && result.usage == (Usage{}) && !result.hasUsage
}

// FailureCode is a normalized category an invoked provider adapter may return.
type FailureCode string

const (
	// FailureAuthenticationFailed means the provider rejected its configured credentials.
	FailureAuthenticationFailed FailureCode = "authentication_failed"
	// FailureRateLimited means the invoked provider reported a rate or quota limit.
	FailureRateLimited FailureCode = "rate_limited"
	// FailureRequestRejected means the provider rejected work after invocation began.
	FailureRequestRejected FailureCode = "request_rejected"
	// FailureUnavailable means the provider or required provider transport was unavailable.
	FailureUnavailable FailureCode = "unavailable"
	// FailureInvalidResponse means provider output could not satisfy the expected adapter contract.
	FailureInvalidResponse FailureCode = "invalid_response"
	// FailureUnsupportedUpstreamOutput means paid work produced an unsupported semantic value.
	FailureUnsupportedUpstreamOutput FailureCode = "unsupported_upstream_output"
	// FailureInternal means adapter-local processing failed after unsafe details were discarded.
	FailureInternal FailureCode = "internal_error"
)

// Failure is a normalized provider error safe to return through Invoker.
//
// It intentionally contains no provider-authored message, raw error, response body, header,
// request, credential, or endpoint. Fixed client-facing messages belong to a later presentation
// mapping. Retryable is an observation only and never authorizes a retry.
type Failure struct {
	code      FailureCode
	retryable bool
	usage     Usage
	hasUsage  bool
}

// NewFailure validates a normalized provider failure and copies optional usage.
func NewFailure(code FailureCode, retryable bool, usage *Usage) (*Failure, error) {
	failure := &Failure{code: code, retryable: retryable}
	if usage != nil {
		failure.usage = *usage
		failure.hasUsage = true
	}
	if err := failure.Validate(); err != nil {
		return nil, err
	}
	return failure, nil
}

// Error returns a fixed, content-free explanation. Inspect Code for the normalized category.
func (*Failure) Error() string {
	return "provider invocation failed"
}

// Validate checks the adapter-owned failure category and any observed usage.
func (failure *Failure) Validate() error {
	if failure == nil {
		return errors.New("provider failure is required")
	}
	switch failure.code {
	case FailureAuthenticationFailed,
		FailureRateLimited,
		FailureRequestRejected,
		FailureUnavailable,
		FailureInvalidResponse,
		FailureUnsupportedUpstreamOutput,
		FailureInternal:
	default:
		return errors.New("provider failure code is unsupported")
	}
	if failure.hasUsage {
		return validateUsage(failure.usage)
	}
	return nil
}

// Code returns the normalized provider-owned failure category.
func (failure *Failure) Code() FailureCode {
	if failure == nil {
		return ""
	}
	return failure.code
}

// Retryable reports a provider observation; it does not authorize another attempt.
func (failure *Failure) Retryable() bool {
	return failure != nil && failure.retryable
}

// Usage returns a copied usage observation and whether one was present.
func (failure *Failure) Usage() (Usage, bool) {
	if failure == nil {
		return Usage{}, false
	}
	return failure.usage, failure.hasUsage
}

// Invoker is the synchronous non-streaming provider port implemented by fake and live adapters.
//
// Invoke returns either a valid Result and nil, a zero Result and a direct *Failure, or a zero
// Result and exactly context.Canceled or context.DeadlineExceeded when the supplied caller context
// reports that same termination. Raw or wrapped provider errors must be normalized before crossing
// this boundary. Invoke defines no stream, retry, explicit cancellation method, or
// cleanup-confirmation lifecycle.
type Invoker interface {
	Invoke(context.Context, Request) (Result, error)
}

// ValidateInvocation enforces Invoker's result/error alternatives without exposing error content.
// A context termination is valid only when it matches the supplied caller context's state.
func ValidateInvocation(ctx context.Context, result Result, err error) error {
	if ctx == nil {
		return errors.New("provider invocation context is required")
	}
	if err == nil {
		return result.Validate()
	}
	if !result.isZero() {
		return errors.New("provider invocation cannot return a result and an error")
	}
	if err == context.Canceled || err == context.DeadlineExceeded {
		if ctx.Err() != err {
			return errors.New("provider invocation context termination does not match caller context")
		}
		return nil
	}

	failure, ok := err.(*Failure)
	if !ok {
		return errors.New(
			"provider invocation error must be a direct normalized failure or context termination",
		)
	}
	return failure.Validate()
}

func validateRequestCardinalities(
	conversation int,
	instructions int,
	requiredCapabilities int,
) error {
	if conversation < 1 || conversation > MaxConversationMessages {
		return fmt.Errorf(
			"conversation must contain between 1 and %d messages",
			MaxConversationMessages,
		)
	}
	if instructions > MaxInstructions {
		return fmt.Errorf("instructions must contain at most %d blocks", MaxInstructions)
	}
	if requiredCapabilities > MaxRequiredCapabilities {
		return fmt.Errorf(
			"required capabilities must contain at most %d value",
			MaxRequiredCapabilities,
		)
	}
	return nil
}

func validateUsage(usage Usage) error {
	if usage.InputTokens < 0 || usage.InputTokens > MaxUsageTokens {
		return fmt.Errorf("input token usage must be between 0 and %d", MaxUsageTokens)
	}
	if usage.OutputTokens < 0 || usage.OutputTokens > MaxUsageTokens {
		return fmt.Errorf("output token usage must be between 0 and %d", MaxUsageTokens)
	}
	return nil
}

func validateText(text string, field string, minimum int, maximum int, rejectControls bool) error {
	length := 0
	for len(text) > 0 {
		character, size := utf8.DecodeRuneInString(text)
		if character == utf8.RuneError && size == 1 {
			return fmt.Errorf("%s must be valid UTF-8", field)
		}
		length++
		if length > maximum {
			return fmt.Errorf(
				"%s must contain between %d and %d Unicode scalar values",
				field,
				minimum,
				maximum,
			)
		}
		if rejectControls && (character <= '\u001f' ||
			character >= '\u007f' && character <= '\u009f' ||
			character == '\u2028' ||
			character == '\u2029') {
			return fmt.Errorf("%s must not contain controls or line separators", field)
		}
		text = text[size:]
	}
	if length < minimum {
		return fmt.Errorf(
			"%s must contain between %d and %d Unicode scalar values",
			field,
			minimum,
			maximum,
		)
	}
	return nil
}
