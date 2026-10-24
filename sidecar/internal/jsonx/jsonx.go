// Package jsonx is a small order-preserving JSON model, used to edit files
// such as ~/.claude/settings.json without reordering or re-escaping keys.
package jsonx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Object is a JSON object that remembers key order. Values are *Object,
// []any, string, json.Number, bool or nil.
type Object struct {
	Keys []string
	Vals map[string]any
}

// NewObject returns an empty Object.
func NewObject() *Object {
	return &Object{Vals: map[string]any{}}
}

// Get returns the value for k.
func (o *Object) Get(k string) (any, bool) {
	v, ok := o.Vals[k]
	return v, ok
}

// Set replaces k in place, or appends it as a new last key.
func (o *Object) Set(k string, v any) {
	if o.Vals == nil {
		o.Vals = map[string]any{}
	}
	if _, ok := o.Vals[k]; !ok {
		o.Keys = append(o.Keys, k)
	}
	o.Vals[k] = v
}

// Delete removes k if present.
func (o *Object) Delete(k string) {
	if _, ok := o.Vals[k]; !ok {
		return
	}
	delete(o.Vals, k)
	for i, key := range o.Keys {
		if key == k {
			o.Keys = append(o.Keys[:i], o.Keys[i+1:]...)
			break
		}
	}
}

// SyntaxError reports invalid JSON with a 1-based line and column.
type SyntaxError struct {
	Line, Col int
	Msg       string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("invalid JSON at line %d, column %d: %s", e.Line, e.Col, e.Msg)
}

func syntaxErr(data []byte, offset int64, msg string) *SyntaxError {
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	if offset < 0 {
		offset = 0
	}
	prefix := data[:offset]
	line := bytes.Count(prefix, []byte{'\n'}) + 1
	// offset counts bytes read, so the offending byte is the last one.
	col := int(offset) - bytes.LastIndexByte(prefix, '\n') - 1
	if col < 1 {
		col = 1
	}
	return &SyntaxError{Line: line, Col: col, Msg: msg}
}

// Parse decodes data, whose root must be an object. Duplicate keys keep the
// first position and the last value.
func Parse(data []byte) (*Object, error) {
	if !utf8.Valid(data) {
		off := 0
		for off < len(data) {
			r, n := utf8.DecodeRune(data[off:])
			if r == utf8.RuneError && n <= 1 {
				break
			}
			off += n
		}
		return nil, syntaxErr(data, int64(off+1), "invalid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, wrapErr(data, dec, err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, errors.New("jsonx: root must be an object")
	}
	obj, err := parseObject(data, dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, syntaxErr(data, dec.InputOffset(), "unexpected data after top-level value")
		}
		return nil, wrapErr(data, dec, err)
	}
	if err := checkSurrogates(data); err != nil {
		return nil, err
	}
	return obj, nil
}

// checkSurrogates rejects \u escapes that form an unpaired surrogate, which
// would otherwise be silently rewritten as U+FFFD. data must be valid JSON.
func checkSurrogates(data []byte) error {
	hex := func(i int) (rune, bool) {
		if i+6 > len(data) || data[i] != '\\' || data[i+1] != 'u' {
			return 0, false
		}
		var r rune
		for _, c := range data[i+2 : i+6] {
			switch {
			case c >= '0' && c <= '9':
				c -= '0'
			case c >= 'a' && c <= 'f':
				c = c - 'a' + 10
			case c >= 'A' && c <= 'F':
				c = c - 'A' + 10
			default:
				return 0, false
			}
			r = r<<4 | rune(c)
		}
		return r, true
	}
	inStr := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if !inStr {
			inStr = c == '"'
			continue
		}
		switch c {
		case '"':
			inStr = false
		case '\\':
			if i+1 < len(data) && data[i+1] == 'u' {
				r, _ := hex(i)
				switch {
				case r >= 0xD800 && r < 0xDC00:
					if lo, ok := hex(i + 6); ok && lo >= 0xDC00 && lo < 0xE000 {
						i += 11
						continue
					}
					return syntaxErr(data, int64(i+1), fmt.Sprintf("unpaired surrogate \\u%04x", r))
				case r >= 0xDC00 && r < 0xE000:
					return syntaxErr(data, int64(i+1), fmt.Sprintf("unpaired surrogate \\u%04x", r))
				}
				i += 5
			} else {
				i++
			}
		}
	}
	return nil
}

func wrapErr(data []byte, dec *json.Decoder, err error) error {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		// Decoder.Token can report a misleading offset (e.g. a trailing comma),
		// so prefer the position from a full-document scan.
		var scratch any
		var full *json.SyntaxError
		if errors.As(json.Unmarshal(data, &scratch), &full) {
			se = full
		}
		return syntaxErr(data, se.Offset, se.Error())
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return syntaxErr(data, int64(len(data)), "unexpected end of JSON input")
	}
	return err
}

// parseObject reads members after the opening '{' through the closing '}'.
func parseObject(data []byte, dec *json.Decoder) (*Object, error) {
	obj := NewObject()
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, wrapErr(data, dec, err)
		}
		key, ok := kt.(string)
		if !ok {
			return nil, syntaxErr(data, dec.InputOffset(), "object key must be a string")
		}
		v, err := parseValue(data, dec)
		if err != nil {
			return nil, err
		}
		obj.Set(key, v)
	}
	if _, err := dec.Token(); err != nil { // '}'
		return nil, wrapErr(data, dec, err)
	}
	return obj, nil
}

func parseValue(data []byte, dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, wrapErr(data, dec, err)
	}
	if d, ok := tok.(json.Delim); ok {
		switch d {
		case '{':
			return parseObject(data, dec)
		case '[':
			arr := []any{}
			for dec.More() {
				v, err := parseValue(data, dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil { // ']'
				return nil, wrapErr(data, dec, err)
			}
			return arr, nil
		}
		return nil, syntaxErr(data, dec.InputOffset(), "unexpected delimiter")
	}
	return tok, nil // string, json.Number, bool or nil
}

// Marshal renders o with 2-space indent, key order kept, no HTML escaping and
// a trailing newline. Empty objects and arrays are written as {} and [].
func Marshal(o *Object) []byte {
	var b bytes.Buffer
	writeObject(&b, o, 0)
	b.WriteByte('\n')
	return b.Bytes()
}

func indent(b *bytes.Buffer, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
}

func writeString(b *bytes.Buffer, s string) {
	var sb bytes.Buffer
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // encoding a string cannot fail
	b.Write(bytes.TrimRight(sb.Bytes(), "\n"))
}

func writeObject(b *bytes.Buffer, o *Object, depth int) {
	if o == nil || len(o.Keys) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteString("{\n")
	for i, k := range o.Keys {
		indent(b, depth+1)
		writeString(b, k)
		b.WriteString(": ")
		writeValue(b, o.Vals[k], depth+1)
		if i < len(o.Keys)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	indent(b, depth)
	b.WriteByte('}')
}

func writeValue(b *bytes.Buffer, v any, depth int) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case *Object:
		writeObject(b, x, depth)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteString("[\n")
		for i, e := range x {
			indent(b, depth+1)
			writeValue(b, e, depth+1)
			if i < len(x)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		indent(b, depth)
		b.WriteByte(']')
	case string:
		writeString(b, x)
	case json.Number:
		b.WriteString(x.String())
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	default:
		// Unsupported types are a programming error; emit null rather than panic.
		b.WriteString("null")
	}
}
