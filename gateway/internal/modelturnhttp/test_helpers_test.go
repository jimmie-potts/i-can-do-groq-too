package modelturnhttp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

type testRequestDocument struct {
	Version              string                   `json:"version"`
	Kind                 string                   `json:"kind"`
	RequestID            string                   `json:"request_id"`
	ModelAlias           string                   `json:"model_alias"`
	Conversation         []testRequestMessage     `json:"conversation"`
	Instructions         []testRequestInstruction `json:"instructions"`
	RequiredCapabilities []string                 `json:"required_capabilities"`
}

type testRequestMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type testRequestInstruction struct {
	Source  string `json:"source"`
	Content string `json:"content"`
}

type testInvokerFunc func(context.Context, provider.Request) (provider.Result, error)

func (function testInvokerFunc) Invoke(
	ctx context.Context,
	request provider.Request,
) (provider.Result, error) {
	return function(ctx, request)
}

func defaultTestDocument() testRequestDocument {
	return testRequestDocument{
		Version:              modelTurnVersion,
		Kind:                 "model_turn.request",
		RequestID:            "request-010",
		ModelAlias:           "learning-text",
		Conversation:         []testRequestMessage{{Role: "user", Content: "hello"}},
		Instructions:         []testRequestInstruction{},
		RequiredCapabilities: []string{},
	}
}

func marshalTestDocument(t *testing.T, document testRequestDocument) []byte {
	t.Helper()
	body, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() returned an error: %v", err)
	}
	return body
}

func providerRequestForDocument(t *testing.T, document testRequestDocument) provider.Request {
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

func expectedAdmissionFailure(requestID string, code string, message string) string {
	return `{"version":"v1","kind":"model_turn.failed","request_id":"` + requestID +
		`","error":{"code":"` + code + `","message":"` + message +
		`","retryable":false}}`
}
