// Package fake provides FastGate's strict non-streaming provider test double.
package fake

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

const (
	// MaxExchanges bounds one in-memory fake-provider script.
	MaxExchanges = 64
)

type outcomeKind uint8

const (
	outcomeResult outcomeKind = iota + 1
	outcomeFailure
)

// Exchange is one immutable expected request and its scripted terminal outcome.
// Construct exchanges with ExpectResult or ExpectFailure.
type Exchange struct {
	expected provider.Request
	result   provider.Result
	failure  *provider.Failure
	outcome  outcomeKind
}

// ExpectResult constructs one expected request and completed result exchange.
func ExpectResult(expected provider.Request, result provider.Result) (Exchange, error) {
	exchange := Exchange{
		expected: expected,
		result:   result,
		outcome:  outcomeResult,
	}
	if err := exchange.validate(); err != nil {
		return Exchange{}, err
	}
	return exchange, nil
}

// ExpectFailure constructs one expected request and direct normalized failure exchange.
func ExpectFailure(expected provider.Request, failure *provider.Failure) (Exchange, error) {
	exchange := Exchange{
		expected: expected,
		failure:  failure,
		outcome:  outcomeFailure,
	}
	if err := exchange.validate(); err != nil {
		return Exchange{}, err
	}
	return exchange, nil
}

// Provider implements provider.Invoker with one strict ordered in-memory script.
//
// Provider has one owner. Invoke and VerifyComplete must not be called concurrently. A request
// mismatch, nil context, or extra invocation is a test-programming error: Provider records a
// bounded content-safe diagnostic and panics instead of inventing a provider failure. The recorded
// violation remains observable through VerifyComplete after recovery.
type Provider struct {
	exchanges []Exchange
	next      int
	violation error
}

// New validates and takes an immutable copy of an ordered exchange script.
// An empty script is valid and asserts that no provider invocation occurs.
func New(exchanges ...Exchange) (*Provider, error) {
	if len(exchanges) > MaxExchanges {
		return nil, fmt.Errorf("fake provider script must contain at most %d exchanges", MaxExchanges)
	}
	owned := slices.Clone(exchanges)
	for index, exchange := range owned {
		if err := exchange.validate(); err != nil {
			return nil, fmt.Errorf("fake provider exchange %d is invalid: %w", index+1, err)
		}
	}
	return &Provider{exchanges: owned}, nil
}

// Invoke matches the next request exactly and returns its scripted result or direct failure.
// It deliberately does not simulate cancellation, deadlines, streaming, or concurrent calls.
func (fake *Provider) Invoke(
	ctx context.Context,
	actual provider.Request,
) (provider.Result, error) {
	if fake == nil {
		panic(errors.New("fake provider is required"))
	}
	if fake.violation != nil {
		panic(fake.violation)
	}
	if ctx == nil {
		fake.fail("fake provider invocation context is required")
	}

	exchangeNumber := fake.next + 1
	if fake.next >= len(fake.exchanges) {
		fake.fail(fmt.Sprintf(
			"fake provider received unexpected request %d; the script has no remaining exchanges",
			exchangeNumber,
		))
	}

	exchange := fake.exchanges[fake.next]
	if path := firstRequestMismatch(exchange.expected, actual); path != "" {
		fake.fail(fmt.Sprintf(
			"fake provider request %d differed at %s",
			exchangeNumber,
			path,
		))
	}

	fake.next++
	if exchange.outcome == outcomeResult {
		return exchange.result, nil
	}
	return provider.Result{}, exchange.failure
}

// VerifyComplete reports a prior interaction violation or the next unconsumed exchange.
func (fake *Provider) VerifyComplete() error {
	if fake == nil {
		return errors.New("fake provider is required")
	}
	if fake.violation != nil {
		return fake.violation
	}
	if fake.next < len(fake.exchanges) {
		remaining := len(fake.exchanges) - fake.next
		return fmt.Errorf(
			"fake provider expected request %d, but %d exchange(s) were never invoked",
			fake.next+1,
			remaining,
		)
	}
	return nil
}

func (exchange Exchange) validate() error {
	if err := exchange.expected.Validate(); err != nil {
		return fmt.Errorf("expected request is invalid: %w", err)
	}
	switch exchange.outcome {
	case outcomeResult:
		if exchange.failure != nil {
			return errors.New("completed exchange cannot contain a failure")
		}
		if err := exchange.result.Validate(); err != nil {
			return fmt.Errorf("scripted result is invalid: %w", err)
		}
	case outcomeFailure:
		if exchange.result != (provider.Result{}) {
			return errors.New("failed exchange cannot contain a result")
		}
		if err := exchange.failure.Validate(); err != nil {
			return fmt.Errorf("scripted failure is invalid: %w", err)
		}
	default:
		return errors.New("exchange outcome is required")
	}
	return nil
}

func (fake *Provider) fail(diagnostic string) {
	if fake.violation == nil {
		fake.violation = errors.New(diagnostic)
	}
	panic(fake.violation)
}

func firstRequestMismatch(expected provider.Request, actual provider.Request) string {
	expectedConversation := expected.Conversation()
	actualConversation := actual.Conversation()
	if len(expectedConversation) != len(actualConversation) {
		return "conversation.length"
	}
	for index := range expectedConversation {
		if expectedConversation[index].Role != actualConversation[index].Role {
			return fmt.Sprintf("conversation[%d].role", index)
		}
		if expectedConversation[index].Content != actualConversation[index].Content {
			return fmt.Sprintf("conversation[%d].content", index)
		}
	}

	expectedInstructions := expected.Instructions()
	actualInstructions := actual.Instructions()
	if len(expectedInstructions) != len(actualInstructions) {
		return "instructions.length"
	}
	for index := range expectedInstructions {
		if expectedInstructions[index].Source != actualInstructions[index].Source {
			return fmt.Sprintf("instructions[%d].source", index)
		}
		if expectedInstructions[index].Content != actualInstructions[index].Content {
			return fmt.Sprintf("instructions[%d].content", index)
		}
	}

	expectedCapabilities := expected.RequiredCapabilities()
	actualCapabilities := actual.RequiredCapabilities()
	if len(expectedCapabilities) != len(actualCapabilities) {
		return "requiredCapabilities.length"
	}
	for index := range expectedCapabilities {
		if expectedCapabilities[index] != actualCapabilities[index] {
			return fmt.Sprintf("requiredCapabilities[%d]", index)
		}
	}
	return ""
}
