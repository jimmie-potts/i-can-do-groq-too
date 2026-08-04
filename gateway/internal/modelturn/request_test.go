package modelturn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/jimmie-potts/i-can-do-groq-too/gateway/internal/provider"
)

func TestDecodeWireRequestPreservesExactV1Values(t *testing.T) {
	raw := []byte(`{
  "version":"v1",
  "kind":"model_turn.request",
  "request_id":"req.exact:009",
  "model_alias":"learning-text",
  "conversation":[
    {"role":"user","content":"cafe\u0301"},
    {"role":"assistant","content":"previous reply \ud83d\ude00"}
  ],
  "instructions":[
    {"source":"first","content":"keep exact order"},
    {"source":"second","content":"do not normalize e\u0301"}
  ],
  "required_capabilities":["tool_calls"]
}`)

	request, ok := requestTestDecodeRaw(t, raw)
	if !ok {
		t.Fatal("decodeWireRequest() rejected an exact v1 request")
	}
	want := wireRequest{
		requestID:  "req.exact:009",
		modelAlias: "learning-text",
		conversation: []provider.Message{
			{Role: provider.MessageRoleUser, Content: "cafe\u0301"},
			{Role: provider.MessageRoleAssistant, Content: "previous reply 😀"},
		},
		instructions: []provider.Instruction{
			{Source: "first", Content: "keep exact order"},
			{Source: "second", Content: "do not normalize e\u0301"},
		},
		requiredCapabilities: []string{"tool_calls"},
	}
	if !reflect.DeepEqual(request, want) {
		t.Fatal("decodeWireRequest() did not preserve exact ordered v1 values")
	}
}

func TestDecodeWireRequestAcceptsExactBounds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "request identifier 128 ASCII bytes",
			mutate: func(document map[string]any) {
				document["request_id"] = "A" + strings.Repeat("1", 127)
			},
		},
		{
			name: "model alias 128 ASCII bytes",
			mutate: func(document map[string]any) {
				document["model_alias"] = "M" + strings.Repeat("1", 127)
			},
		},
		{
			name: "64 conversation messages",
			mutate: func(document map[string]any) {
				document["conversation"] = requestTestMessages(provider.MaxConversationMessages)
			},
		},
		{
			name: "65536 message scalars",
			mutate: func(document map[string]any) {
				document["conversation"] = []any{requestTestMessage("user", strings.Repeat("界", provider.MaxMessageTextRunes))}
			},
		},
		{
			name: "32 instructions",
			mutate: func(document map[string]any) {
				document["instructions"] = requestTestInstructions(provider.MaxInstructions)
			},
		},
		{
			name: "256 instruction source scalars",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{requestTestInstruction(strings.Repeat("é", provider.MaxInstructionSourceRunes), "content")}
			},
		},
		{
			name: "65536 instruction content scalars",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{requestTestInstruction("source", strings.Repeat("😀", provider.MaxInstructionTextRunes))}
			},
		},
		{
			name: "one capability",
			mutate: func(document map[string]any) {
				document["required_capabilities"] = []any{"tool_calls"}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := requestTestDocument()
			test.mutate(document)
			if _, ok := requestTestDecodeDocument(t, document); !ok {
				t.Fatal("decodeWireRequest() rejected an exact schema bound")
			}
		})
	}
}

func TestDecodeWireRequestRejectsWrongTypesAndExactFieldViolations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "wrong version", mutate: func(document map[string]any) { document["version"] = "v2" }},
		{name: "wrong kind", mutate: func(document map[string]any) { document["kind"] = "model_turn.completed" }},
		{name: "numeric version", mutate: func(document map[string]any) { document["version"] = 1 }},
		{name: "null kind", mutate: func(document map[string]any) { document["kind"] = nil }},
		{name: "boolean request identifier", mutate: func(document map[string]any) { document["request_id"] = true }},
		{name: "array model alias", mutate: func(document map[string]any) { document["model_alias"] = []any{} }},
		{name: "object conversation", mutate: func(document map[string]any) { document["conversation"] = map[string]any{} }},
		{name: "null instructions", mutate: func(document map[string]any) { document["instructions"] = nil }},
		{name: "string capabilities", mutate: func(document map[string]any) { document["required_capabilities"] = "tool_calls" }},
		{name: "unknown root field", mutate: func(document map[string]any) { document["temperature"] = 0 }},
		{
			name: "case-variant root field",
			mutate: func(document map[string]any) {
				delete(document, "request_id")
				document["Request_ID"] = "req-009"
			},
		},
		{
			name: "unknown message field",
			mutate: func(document map[string]any) {
				document["conversation"].([]any)[0].(map[string]any)["name"] = "caller"
			},
		},
		{
			name: "missing message content",
			mutate: func(document map[string]any) {
				delete(document["conversation"].([]any)[0].(map[string]any), "content")
			},
		},
		{
			name: "missing message role",
			mutate: func(document map[string]any) {
				delete(document["conversation"].([]any)[0].(map[string]any), "role")
			},
		},
		{
			name: "null message",
			mutate: func(document map[string]any) {
				document["conversation"] = []any{nil}
			},
		},
		{
			name: "case-variant message role",
			mutate: func(document map[string]any) {
				message := document["conversation"].([]any)[0].(map[string]any)
				delete(message, "role")
				message["Role"] = "user"
			},
		},
		{
			name: "unknown instruction field",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{map[string]any{"source": "s", "content": "c", "extra": true}}
			},
		},
		{
			name: "missing instruction source",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{map[string]any{"content": "c"}}
			},
		},
		{
			name: "missing instruction content",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{map[string]any{"source": "s"}}
			},
		},
		{
			name: "string instruction",
			mutate: func(document map[string]any) {
				document["instructions"] = []any{"not-an-object"}
			},
		},
	}

	for _, field := range []string{
		"version",
		"kind",
		"request_id",
		"model_alias",
		"conversation",
		"instructions",
		"required_capabilities",
	} {
		field := field
		tests = append(tests, struct {
			name   string
			mutate func(map[string]any)
		}{
			name: "missing root field " + field,
			mutate: func(document map[string]any) {
				delete(document, field)
			},
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := requestTestDocument()
			test.mutate(document)
			if _, ok := requestTestDecodeDocument(t, document); ok {
				t.Fatal("decodeWireRequest() accepted an inexact v1 shape")
			}
		})
	}
}

func TestDecodeWireRequestRejectsValuesBeyondV1Bounds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "empty request identifier", mutate: func(document map[string]any) { document["request_id"] = "" }},
		{name: "request identifier 129 bytes", mutate: func(document map[string]any) { document["request_id"] = "A" + strings.Repeat("1", 128) }},
		{name: "request identifier starts with punctuation", mutate: func(document map[string]any) { document["request_id"] = "-req" }},
		{name: "request identifier contains slash", mutate: func(document map[string]any) { document["request_id"] = "req/009" }},
		{name: "request identifier contains non-ASCII", mutate: func(document map[string]any) { document["request_id"] = "req-é" }},
		{name: "empty model alias", mutate: func(document map[string]any) { document["model_alias"] = "" }},
		{name: "model alias 129 bytes", mutate: func(document map[string]any) { document["model_alias"] = "M" + strings.Repeat("1", 128) }},
		{name: "model alias contains whitespace", mutate: func(document map[string]any) { document["model_alias"] = "learning text" }},
		{name: "empty conversation", mutate: func(document map[string]any) { document["conversation"] = []any{} }},
		{name: "65 conversation messages", mutate: func(document map[string]any) {
			document["conversation"] = requestTestMessages(provider.MaxConversationMessages + 1)
		}},
		{name: "unsupported message role", mutate: func(document map[string]any) {
			document["conversation"] = []any{requestTestMessage("system", "content")}
		}},
		{name: "numeric message role", mutate: func(document map[string]any) { document["conversation"] = []any{requestTestMessage(1, "content")} }},
		{name: "empty message content", mutate: func(document map[string]any) { document["conversation"] = []any{requestTestMessage("user", "")} }},
		{name: "non-string message content", mutate: func(document map[string]any) { document["conversation"] = []any{requestTestMessage("user", true)} }},
		{name: "65537 message scalars", mutate: func(document map[string]any) {
			document["conversation"] = []any{requestTestMessage("user", strings.Repeat("界", provider.MaxMessageTextRunes+1))}
		}},
		{name: "33 instructions", mutate: func(document map[string]any) {
			document["instructions"] = requestTestInstructions(provider.MaxInstructions + 1)
		}},
		{name: "empty instruction source", mutate: func(document map[string]any) { document["instructions"] = []any{requestTestInstruction("", "content")} }},
		{name: "non-string instruction source", mutate: func(document map[string]any) {
			document["instructions"] = []any{requestTestInstruction(false, "content")}
		}},
		{name: "257 instruction source scalars", mutate: func(document map[string]any) {
			document["instructions"] = []any{requestTestInstruction(strings.Repeat("é", provider.MaxInstructionSourceRunes+1), "content")}
		}},
		{name: "empty instruction content", mutate: func(document map[string]any) { document["instructions"] = []any{requestTestInstruction("source", "")} }},
		{name: "non-string instruction content", mutate: func(document map[string]any) { document["instructions"] = []any{requestTestInstruction("source", 1)} }},
		{name: "65537 instruction content scalars", mutate: func(document map[string]any) {
			document["instructions"] = []any{requestTestInstruction("source", strings.Repeat("😀", provider.MaxInstructionTextRunes+1))}
		}},
		{name: "two capabilities", mutate: func(document map[string]any) { document["required_capabilities"] = []any{"tool_calls", "tool_calls"} }},
		{name: "unknown capability", mutate: func(document map[string]any) { document["required_capabilities"] = []any{"image_input"} }},
		{name: "non-string capability", mutate: func(document map[string]any) { document["required_capabilities"] = []any{true} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := requestTestDocument()
			test.mutate(document)
			if _, ok := requestTestDecodeDocument(t, document); ok {
				t.Fatal("decodeWireRequest() accepted a value outside a v1 bound")
			}
		})
	}
}

func TestDecodeWireRequestInstructionSourceControlProfile(t *testing.T) {
	var rejected []rune
	for character := rune('\u0000'); character <= '\u001f'; character++ {
		rejected = append(rejected, character)
	}
	for character := rune('\u007f'); character <= '\u009f'; character++ {
		rejected = append(rejected, character)
	}
	rejected = append(rejected, '\u2028', '\u2029')
	for _, character := range rejected {
		t.Run("reject U+"+requestTestHexRune(character), func(t *testing.T) {
			document := requestTestDocument()
			document["instructions"] = []any{requestTestInstruction("before"+string(character)+"after", "content")}
			if _, ok := requestTestDecodeDocument(t, document); ok {
				t.Fatal("decodeWireRequest() accepted a forbidden instruction-source character")
			}
		})
	}

	allowed := []rune{'\u0020', '\u007e', '\u00a0', '\u2027', '\u202a'}
	for _, character := range allowed {
		t.Run("allow U+"+requestTestHexRune(character), func(t *testing.T) {
			document := requestTestDocument()
			document["instructions"] = []any{requestTestInstruction("before"+string(character)+"after", "content")}
			if _, ok := requestTestDecodeDocument(t, document); !ok {
				t.Fatal("decodeWireRequest() rejected an allowed instruction-source character")
			}
		})
	}
}

func TestValidIdentifierUsesExactASCIIProfile(t *testing.T) {
	accepted := []string{
		"A",
		"0",
		"a.b_c:d-e",
		"Z" + strings.Repeat("9", maxIdentifierBytes-1),
	}
	for _, identifier := range accepted {
		if !validIdentifier(identifier) {
			t.Errorf("validIdentifier(%q) = false, want true", identifier)
		}
	}

	rejected := []string{
		"",
		".starts-with-punctuation",
		"_starts-with-punctuation",
		":starts-with-punctuation",
		"-starts-with-punctuation",
		"contains space",
		"contains/slash",
		"contains-é",
		"A" + strings.Repeat("1", maxIdentifierBytes),
	}
	for _, identifier := range rejected {
		if validIdentifier(identifier) {
			t.Errorf("validIdentifier(%q) = true, want false", identifier)
		}
	}
}

func TestSafeRequestIDRequiresWholeStrictDocument(t *testing.T) {
	invalidUTF8 := append([]byte(`{"request_id":"req-utf8","content":"`), 0xff)
	invalidUTF8 = append(invalidUTF8, []byte(`"}`)...)
	tests := []struct {
		name   string
		raw    []byte
		wantID string
		wantOK bool
	}{
		{name: "complete request", raw: requestTestMarshal(t, requestTestDocument()), wantID: "req-009", wantOK: true},
		{name: "wrong version remains safely correlated", raw: requestTestMutatedRaw(t, func(document map[string]any) { document["version"] = "v2" }), wantID: "req-009", wantOK: true},
		{name: "unknown field with huge exponent remains safely correlated", raw: []byte(`{"version":"v1","kind":"model_turn.request","request_id":"req-number","model_alias":"learning-text","conversation":[{"role":"user","content":"x"}],"instructions":[],"required_capabilities":[],"extra":1e999}`), wantID: "req-number", wantOK: true},
		{name: "decoded escaped identifier", raw: []byte(`{"request_id":"\u0072eq-escaped","extra":true}`), wantID: "req-escaped", wantOK: true},
		{name: "missing identifier", raw: []byte(`{"version":"v1"}`)},
		{name: "wrong identifier type", raw: []byte(`{"request_id":7}`)},
		{name: "lexically invalid identifier", raw: []byte(`{"request_id":"-unsafe"}`)},
		{name: "non-object root", raw: []byte(`["req-array"]`)},
		{name: "malformed prefix containing identifier", raw: []byte(`{"request_id":"req-prefix",`)},
		{name: "duplicate identifier", raw: []byte(`{"request_id":"req-first","request_id":"req-second"}`)},
		{name: "decoded duplicate identifier", raw: []byte(`{"request_id":"req-first","request\u005fid":"req-second"}`)},
		{name: "duplicate nested name", raw: []byte(`{"request_id":"req-nested","nested":{"x":1,"x":2}}`)},
		{name: "lone high surrogate elsewhere", raw: []byte(`{"request_id":"req-high","content":"\ud800"}`)},
		{name: "lone low surrogate elsewhere", raw: []byte(`{"request_id":"req-low","content":"\udc00"}`)},
		{name: "invalid UTF-8 elsewhere", raw: invalidUTF8},
		{name: "invalid number elsewhere", raw: []byte(`{"request_id":"req-number","extra":NaN}`)},
		{name: "second top-level value", raw: []byte(`{"request_id":"req-first"} {"request_id":"req-second"}`)},
		{name: "identifier above public bound", raw: []byte(`{"request_id":"A` + strings.Repeat("1", maxIdentifierBytes) + `"}`)},
		{name: "identifier with trailing newline", raw: []byte("{\"request_id\":\"req-newline\\n\"}")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotID, gotOK := requestTestRecoverSafeID(test.raw)
			if gotID != test.wantID || gotOK != test.wantOK {
				t.Fatalf("safe request ID = (%q, %t), want (%q, %t)", gotID, gotOK, test.wantID, test.wantOK)
			}
		})
	}
}

func TestSafeRequestIDIsIndependentOfRemainingSchemaAdmission(t *testing.T) {
	tests := []struct {
		name          string
		mutate        func(map[string]any)
		wantSafeID    bool
		wantWireShape bool
	}{
		{name: "valid request", mutate: func(map[string]any) {}, wantSafeID: true, wantWireShape: true},
		{name: "wrong version", mutate: func(document map[string]any) { document["version"] = "v2" }, wantSafeID: true},
		{name: "unknown root field", mutate: func(document map[string]any) { document["secret"] = "not echoed" }, wantSafeID: true},
		{name: "unknown capability", mutate: func(document map[string]any) { document["required_capabilities"] = []any{"image_input"} }, wantSafeID: true},
		{name: "unsafe instruction source", mutate: func(document map[string]any) {
			document["instructions"] = []any{requestTestInstruction("bad\nsource", "content")}
		}, wantSafeID: true},
		{name: "schema-valid tool requirement", mutate: func(document map[string]any) { document["required_capabilities"] = []any{"tool_calls"} }, wantSafeID: true, wantWireShape: true},
		{name: "missing identifier", mutate: func(document map[string]any) { delete(document, "request_id") }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := requestTestMutatedRaw(t, test.mutate)
			if !validateStrictDocument(raw) {
				t.Fatal("generated phase-separation document was not strict JSON")
			}
			root, ok := decodeObject(raw)
			if !ok {
				t.Fatal("generated phase-separation document was not an object")
			}
			_, safe := safeRequestID(root)
			_, decoded := decodeWireRequest(root)
			if safe != test.wantSafeID || decoded != test.wantWireShape {
				t.Fatalf("phase results = (safe ID %t, wire shape %t), want (%t, %t)", safe, decoded, test.wantSafeID, test.wantWireShape)
			}
		})
	}
}

func TestMapProviderRequestPreservesDomainValuesAndCopiesCollections(t *testing.T) {
	wire := requestTestWireRequest()

	got, err := mapProviderRequest(wire)
	if err != nil {
		t.Fatalf("mapProviderRequest() returned an error: %v", err)
	}
	wantConversation := append([]provider.Message(nil), wire.conversation...)
	wantInstructions := append([]provider.Instruction(nil), wire.instructions...)
	if !reflect.DeepEqual(got.Conversation(), wantConversation) {
		t.Fatal("mapped conversation did not preserve exact order and content")
	}
	if !reflect.DeepEqual(got.Instructions(), wantInstructions) {
		t.Fatal("mapped instructions did not preserve exact order and content")
	}
	if capabilities := got.RequiredCapabilities(); len(capabilities) != 0 {
		t.Fatalf("mapped required capabilities = %v, want empty", capabilities)
	}

	wire.conversation[0].Content = "mutated after mapping"
	wire.instructions[0].Content = "mutated after mapping"
	if !reflect.DeepEqual(got.Conversation(), wantConversation) || !reflect.DeepEqual(got.Instructions(), wantInstructions) {
		t.Fatal("provider request changed after wire collections were mutated")
	}
}

func TestMapProviderRequestFailsClosedOnCardinalityAndDomainInconsistency(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*wireRequest)
	}{
		{name: "empty conversation", mutate: func(request *wireRequest) { request.conversation = nil }},
		{name: "too many conversation messages", mutate: func(request *wireRequest) {
			request.conversation = make([]provider.Message, provider.MaxConversationMessages+1)
		}},
		{name: "too many instructions", mutate: func(request *wireRequest) {
			request.instructions = make([]provider.Instruction, provider.MaxInstructions+1)
		}},
		{name: "nonempty admitted capabilities", mutate: func(request *wireRequest) { request.requiredCapabilities = []string{"tool_calls"} }},
		{name: "unsupported provider role", mutate: func(request *wireRequest) { request.conversation[0].Role = provider.MessageRole("system") }},
		{name: "empty provider message", mutate: func(request *wireRequest) { request.conversation[0].Content = "" }},
		{name: "unsafe provider source", mutate: func(request *wireRequest) { request.instructions[0].Source = "unsafe\nsource" }},
		{name: "empty provider instruction", mutate: func(request *wireRequest) { request.instructions[0].Content = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := requestTestWireRequest()
			test.mutate(&request)
			got, err := mapProviderRequest(request)
			if err != errRequestMapping {
				t.Fatalf("mapProviderRequest() error = %v, want fixed mapping error", err)
			}
			if len(got.Conversation()) != 0 || len(got.Instructions()) != 0 || len(got.RequiredCapabilities()) != 0 {
				t.Fatal("mapProviderRequest() returned provider data with its fixed error")
			}
		})
	}
}

func TestCommittedRequestFixturesMatchRuntimeDecoder(t *testing.T) {
	contractRoot := requestFixtureContractRoot(t)
	manifestRaw, err := os.ReadFile(filepath.Join(contractRoot, "fixtures", "cases.json"))
	if err != nil {
		t.Fatalf("read request fixture manifest: %v", err)
	}
	var manifest requestFixtureManifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatalf("decode request fixture manifest: %v", err)
	}

	requestCases := 0
	for _, fixtureCase := range manifest.Cases {
		if fixtureCase.Schema != "schema/request.schema.json" {
			continue
		}
		requestCases++
		t.Run(fixtureCase.Name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(contractRoot, filepath.FromSlash(fixtureCase.Fixture)))
			if err != nil {
				t.Fatalf("read committed request fixture: %v", err)
			}

			strict := validateStrictDocument(raw)
			wantStrict := fixtureCase.Valid || fixtureCase.Expected.Keyword != "json"
			if strict != wantStrict {
				t.Fatalf("strict JSON classification = %t, want %t", strict, wantStrict)
			}
			if !strict {
				return
			}
			root, ok := decodeObject(raw)
			if !ok {
				t.Fatal("strict committed request fixture did not decode as an object")
			}
			_, admitted := decodeWireRequest(root)
			if admitted != fixtureCase.Valid {
				t.Fatalf("request schema classification = %t, want %t", admitted, fixtureCase.Valid)
			}
		})
	}
	if requestCases == 0 {
		t.Fatal("request fixture manifest contained no request-schema cases")
	}
}

type requestFixtureManifest struct {
	Cases []struct {
		Name     string `json:"name"`
		Schema   string `json:"schema"`
		Fixture  string `json:"fixture"`
		Valid    bool   `json:"valid"`
		Expected struct {
			Keyword string `json:"keyword"`
		} `json:"expected"`
	} `json:"cases"`
}

func requestTestDocument() map[string]any {
	return map[string]any{
		"version":               "v1",
		"kind":                  "model_turn.request",
		"request_id":            "req-009",
		"model_alias":           "learning-text",
		"conversation":          []any{requestTestMessage("user", "content")},
		"instructions":          []any{},
		"required_capabilities": []any{},
	}
}

func requestTestMessage(role any, content any) map[string]any {
	return map[string]any{"role": role, "content": content}
}

func requestTestMessages(count int) []any {
	messages := make([]any, count)
	for index := range messages {
		messages[index] = requestTestMessage("user", "content")
	}
	return messages
}

func requestTestInstruction(source any, content any) map[string]any {
	return map[string]any{"source": source, "content": content}
}

func requestTestInstructions(count int) []any {
	instructions := make([]any, count)
	for index := range instructions {
		instructions[index] = requestTestInstruction("source", "content")
	}
	return instructions
}

func requestTestDecodeDocument(t *testing.T, document map[string]any) (wireRequest, bool) {
	t.Helper()
	return requestTestDecodeRaw(t, requestTestMarshal(t, document))
}

func requestTestDecodeRaw(t *testing.T, raw []byte) (wireRequest, bool) {
	t.Helper()
	if !validateStrictDocument(raw) {
		t.Fatal("generated decoder test input was not strict JSON")
	}
	root, ok := decodeObject(raw)
	if !ok {
		t.Fatal("generated decoder test input was not an object")
	}
	return decodeWireRequest(root)
}

func requestTestMarshal(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request test document: %v", err)
	}
	return raw
}

func requestTestMutatedRaw(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()
	document := requestTestDocument()
	mutate(document)
	return requestTestMarshal(t, document)
}

func requestTestRecoverSafeID(raw []byte) (string, bool) {
	if !validateStrictDocument(raw) {
		return "", false
	}
	root, ok := decodeObject(raw)
	if !ok {
		return "", false
	}
	return safeRequestID(root)
}

func requestTestWireRequest() wireRequest {
	return wireRequest{
		requestID:  "req-map-009",
		modelAlias: "learning-text",
		conversation: []provider.Message{
			{Role: provider.MessageRoleUser, Content: "first"},
			{Role: provider.MessageRoleAssistant, Content: "second e\u0301"},
		},
		instructions: []provider.Instruction{
			{Source: "first", Content: "instruction one"},
			{Source: "second", Content: "instruction two"},
		},
	}
}

func requestFixtureContractRoot(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the request test")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "contracts", "model-turn", "v1")
}

func requestTestHexRune(character rune) string {
	const digits = "0123456789ABCDEF"
	value := uint32(character)
	width := 4
	if value > 0xffff {
		width = 6
	}
	encoded := make([]byte, width)
	for index := width - 1; index >= 0; index-- {
		encoded[index] = digits[value&0xf]
		value >>= 4
	}
	return string(encoded)
}
