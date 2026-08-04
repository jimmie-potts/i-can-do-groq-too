package modelturn

import (
	"bytes"
	"encoding/json"
	"errors"
	"unicode/utf8"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

const (
	supportedAlias = "learning-text"

	toolCallsCapability = "tool_calls"
	maxIdentifierBytes  = 128
)

var errRequestMapping = errors.New("model-turn request mapping failed")

type wireRequest struct {
	requestID            string
	modelAlias           string
	conversation         []provider.Message
	instructions         []provider.Instruction
	requiredCapabilities []string
}

func safeRequestID(root map[string]json.RawMessage) (string, bool) {
	requestID, ok := decodeString(root["request_id"])
	if !ok || !validIdentifier(requestID) {
		return "", false
	}
	return requestID, true
}

func decodeWireRequest(root map[string]json.RawMessage) (wireRequest, bool) {
	if !hasExactFields(
		root,
		"version",
		"kind",
		"request_id",
		"model_alias",
		"conversation",
		"instructions",
		"required_capabilities",
	) {
		return wireRequest{}, false
	}

	version, versionOK := decodeString(root["version"])
	kind, kindOK := decodeString(root["kind"])
	requestID, requestIDOK := decodeString(root["request_id"])
	modelAlias, aliasOK := decodeString(root["model_alias"])
	if !versionOK || version != "v1" || !kindOK || kind != "model_turn.request" ||
		!requestIDOK || !validIdentifier(requestID) || !aliasOK || !validIdentifier(modelAlias) {
		return wireRequest{}, false
	}

	conversationRaw, ok := decodeArray(root["conversation"])
	if !ok || len(conversationRaw) < 1 || len(conversationRaw) > provider.MaxConversationMessages {
		return wireRequest{}, false
	}
	conversation := make([]provider.Message, len(conversationRaw))
	for index, raw := range conversationRaw {
		message, valid := decodeWireMessage(raw)
		if !valid {
			return wireRequest{}, false
		}
		conversation[index] = message
	}

	instructionsRaw, ok := decodeArray(root["instructions"])
	if !ok || len(instructionsRaw) > provider.MaxInstructions {
		return wireRequest{}, false
	}
	instructions := make([]provider.Instruction, len(instructionsRaw))
	for index, raw := range instructionsRaw {
		instruction, valid := decodeWireInstruction(raw)
		if !valid {
			return wireRequest{}, false
		}
		instructions[index] = instruction
	}

	capabilitiesRaw, ok := decodeArray(root["required_capabilities"])
	if !ok || len(capabilitiesRaw) > provider.MaxRequiredCapabilities {
		return wireRequest{}, false
	}
	capabilities := make([]string, len(capabilitiesRaw))
	for index, raw := range capabilitiesRaw {
		capability, valid := decodeString(raw)
		if !valid || capability != toolCallsCapability {
			return wireRequest{}, false
		}
		capabilities[index] = capability
	}

	return wireRequest{
		requestID:            requestID,
		modelAlias:           modelAlias,
		conversation:         conversation,
		instructions:         instructions,
		requiredCapabilities: capabilities,
	}, true
}

func decodeWireMessage(raw json.RawMessage) (provider.Message, bool) {
	object, ok := decodeObject(raw)
	if !ok || !hasExactFields(object, "role", "content") {
		return provider.Message{}, false
	}
	role, roleOK := decodeString(object["role"])
	content, contentOK := decodeString(object["content"])
	if !roleOK || role != "user" && role != "assistant" ||
		!contentOK || !validScalarText(content, 1, provider.MaxMessageTextRunes, false) {
		return provider.Message{}, false
	}
	providerRole := provider.MessageRoleUser
	if role == "assistant" {
		providerRole = provider.MessageRoleAssistant
	}
	return provider.Message{Role: providerRole, Content: content}, true
}

func decodeWireInstruction(raw json.RawMessage) (provider.Instruction, bool) {
	object, ok := decodeObject(raw)
	if !ok || !hasExactFields(object, "source", "content") {
		return provider.Instruction{}, false
	}
	source, sourceOK := decodeString(object["source"])
	content, contentOK := decodeString(object["content"])
	if !sourceOK || !validScalarText(source, 1, provider.MaxInstructionSourceRunes, true) ||
		!contentOK || !validScalarText(content, 1, provider.MaxInstructionTextRunes, false) {
		return provider.Instruction{}, false
	}
	return provider.Instruction{Source: source, Content: content}, true
}

func decodeObject(raw []byte) (map[string]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
		return nil, false
	}
	return object, true
}

func decodeArray(raw []byte) ([]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	var values []json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil || values == nil {
		return nil, false
	}
	return values, true
}

func decodeString(raw []byte) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", false
	}
	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return "", false
	}
	return value, true
}

func hasExactFields(object map[string]json.RawMessage, fields ...string) bool {
	if len(object) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return false
		}
	}
	return true
}

func validIdentifier(identifier string) bool {
	if len(identifier) < 1 || len(identifier) > maxIdentifierBytes ||
		!isASCIIAlphaNumeric(identifier[0]) {
		return false
	}
	for index := 1; index < len(identifier); index++ {
		character := identifier[index]
		if !isASCIIAlphaNumeric(character) && character != '.' && character != '_' &&
			character != ':' && character != '-' {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(character byte) bool {
	return character >= 'A' && character <= 'Z' ||
		character >= 'a' && character <= 'z' ||
		character >= '0' && character <= '9'
}

func validScalarText(text string, minimum int, maximum int, rejectControls bool) bool {
	length := 0
	for len(text) > 0 {
		character, size := utf8.DecodeRuneInString(text)
		if character == utf8.RuneError && size == 1 {
			return false
		}
		length++
		if length > maximum || rejectControls && isUnsafeSourceCharacter(character) {
			return false
		}
		text = text[size:]
	}
	return length >= minimum
}

func isUnsafeSourceCharacter(character rune) bool {
	return character <= '\u001f' ||
		character >= '\u007f' && character <= '\u009f' ||
		character == '\u2028' ||
		character == '\u2029'
}

func mapProviderRequest(request wireRequest) (provider.Request, error) {
	if len(request.conversation) < 1 || len(request.conversation) > provider.MaxConversationMessages ||
		len(request.instructions) > provider.MaxInstructions ||
		len(request.requiredCapabilities) != 0 {
		return provider.Request{}, errRequestMapping
	}

	providerRequest, err := provider.NewRequest(request.conversation, request.instructions, nil)
	if err != nil {
		return provider.Request{}, errRequestMapping
	}
	return providerRequest, nil
}
