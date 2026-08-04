package modelturn

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

const (
	// MaxRequestBodyBytes is the exact encoded model-turn request limit.
	MaxRequestBodyBytes = 8 * 1024 * 1024

	maxJSONContainerDepth = 16
	maxConsecutiveNoReads = 100
	readBufferBytes       = 32 * 1024
)

var (
	errRequestBodyTooLarge = errors.New("model-turn request body is too large")
	errReaderContract      = errors.New("model-turn body reader returned an invalid byte count")
)

func readRequestBody(reader io.Reader) ([]byte, error) {
	const maxRetainedBytes = MaxRequestBodyBytes + 1

	request := make([]byte, 0, readBufferBytes)
	var buffer [readBufferBytes]byte
	consecutiveNoReads := 0

	for {
		remaining := maxRetainedBytes - len(request)
		if remaining == 0 {
			return nil, errRequestBodyTooLarge
		}
		offered := min(remaining, len(buffer))
		n, readErr := reader.Read(buffer[:offered])
		if n < 0 || n > offered {
			return nil, errReaderContract
		}
		if n > 0 {
			needed := len(request) + n
			if needed > cap(request) {
				capacity := min(maxRetainedBytes, max(needed, cap(request)*2))
				grown := make([]byte, len(request), capacity)
				copy(grown, request)
				request = grown
			}
			request = append(request, buffer[:n]...)
			consecutiveNoReads = 0
			if len(request) > MaxRequestBodyBytes {
				return nil, errRequestBodyTooLarge
			}
		} else if readErr == nil {
			consecutiveNoReads++
			if consecutiveNoReads >= maxConsecutiveNoReads {
				return nil, io.ErrNoProgress
			}
		}

		if readErr == io.EOF {
			return request, nil
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}
func validateStrictDocument(raw []byte) bool {
	if !utf8.Valid(raw) || !hasValidSurrogateEscapes(raw) {
		return false
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if !walkJSONValue(decoder, 0) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

func walkJSONValue(decoder *json.Decoder, depth int) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		switch token.(type) {
		case nil, bool, string, json.Number:
			return true
		default:
			return false
		}
	}

	if delimiter != '{' && delimiter != '[' {
		return false
	}
	depth++
	if depth > maxJSONContainerDepth {
		return false
	}

	if delimiter == '{' {
		names := make(map[string]struct{})
		for decoder.More() {
			nameToken, nameErr := decoder.Token()
			name, ok := nameToken.(string)
			if nameErr != nil || !ok {
				return false
			}
			if _, duplicate := names[name]; duplicate {
				return false
			}
			names[name] = struct{}{}
			if !walkJSONValue(decoder, depth) {
				return false
			}
		}
	} else {
		for decoder.More() {
			if !walkJSONValue(decoder, depth) {
				return false
			}
		}
	}

	closing, closeErr := decoder.Token()
	if closeErr != nil {
		return false
	}
	want := json.Delim('}')
	if delimiter == '[' {
		want = ']'
	}
	return closing == want
}

func hasValidSurrogateEscapes(raw []byte) bool {
	inString := false
	for index := 0; index < len(raw); index++ {
		switch raw[index] {
		case '"':
			inString = !inString
		case '\\':
			if !inString {
				continue
			}
			index++
			if index >= len(raw) {
				return false
			}
			if raw[index] != 'u' {
				continue
			}
			code, ok := decodeHexEscape(raw, index+1)
			if !ok {
				return false
			}
			index += 4
			switch {
			case code >= 0xd800 && code <= 0xdbff:
				if index+6 >= len(raw) || raw[index+1] != '\\' || raw[index+2] != 'u' {
					return false
				}
				low, lowOK := decodeHexEscape(raw, index+3)
				if !lowOK || low < 0xdc00 || low > 0xdfff {
					return false
				}
				index += 6
			case code >= 0xdc00 && code <= 0xdfff:
				return false
			}
		}
	}
	return true
}

func decodeHexEscape(raw []byte, start int) (uint16, bool) {
	if start < 0 || start+4 > len(raw) {
		return 0, false
	}
	var value uint16
	for _, character := range raw[start : start+4] {
		value <<= 4
		switch {
		case character >= '0' && character <= '9':
			value += uint16(character - '0')
		case character >= 'a' && character <= 'f':
			value += uint16(character-'a') + 10
		case character >= 'A' && character <= 'F':
			value += uint16(character-'A') + 10
		default:
			return 0, false
		}
	}
	return value, true
}
