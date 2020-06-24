package json

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

func Deserialize(json []byte, i interface{}) error {
	d := makeDeserializer(json, i)
	err := d.deserialize()
	if err != nil {
		return err
	}
	return nil
}

func makeDeserializer(json []byte, i interface{}) deserializer {
	return deserializer{
		json: json,
		i:    i,
		pos:  0,
		eof:  len(json) - 1,
	}
}

type deserializer struct {
	json []byte
	i    interface{}
	pos  int
	eof  int
	err  error
}

type PrimitiveType uint

const (
	Invalid PrimitiveType = iota
	String
	Number
	Bool
	Nil
)

type deserializationError struct {
	msg string
}

func (d *deserializationError) Error() string {
	return d.msg
}

func newError(errorMsg string) error {
	return &deserializationError{errorMsg}
}

func (d *deserializer) deserialize() error {
	interfaceValue := reflect.ValueOf(d.i).Elem()
	if isJsonArray(interfaceValue.Kind()) {
		d.deserializeArray(interfaceValue)
		if d.hasError() {
			return d.err
		}
	} else if interfaceValue.Kind() == reflect.Struct {
		d.deserializeObject(interfaceValue)
		if d.hasError() {
			return d.err
		}
	}
	return nil
}

func (d *deserializer) deserializeArray(v reflect.Value) {

	if !isJsonArray(v.Kind()) {
		d.err = newError(fmt.Sprintf("Value is not JsonArray. Kind() is %s", v.Kind().String()))
		return
	}

	d.consumeByte('[')
	d.consumeWhitespace()

	if d.hasError() {
		return
	}

	arrayIndex := 0
	for {
		if d.peekCurrent() == ']' {
			break
		}

		if v.Kind() == reflect.Slice {
			// Grow slice if necessary
			if arrayIndex >= v.Cap() {
				newcap := v.Cap() + v.Cap()/2
				if newcap < 4 {
					newcap = 4
				}
				newv := reflect.MakeSlice(v.Type(), v.Len(), newcap)
				reflect.Copy(newv, v)
				v.Set(newv)
			}
			if arrayIndex >= v.Len() {
				v.SetLen(arrayIndex + 1)
			}
		}

		if arrayIndex < v.Len() {
			if d.peekCurrent() == '[' {
				d.deserializeArray(v.Index(arrayIndex))
				if d.hasError() {
					return
				}
				if d.peekCurrent() == ',' {
					d.consumeByte(',')
				}
			} else if d.peekCurrent() == '{' {
				d.deserializeObject(v.Index(arrayIndex))
				if d.hasError() {
					return
				}
				if d.peekCurrent() == ',' {
					d.consumeByte(',')
				}
			} else if d.isPrimitive() {
				primitiveType, err := d.parsePrimitiveType()
				if err != nil {
					d.err = err
					return
				}

				data, terminator := d.consumeUntilTerminator()
				_ = terminator
				switch primitiveType {
				case Number:
					switch v.Type().Elem().Kind() {
					case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
						uin, err := parseUint(data)
						if err != nil {
							d.err = err
							return
						}
						v.Index(arrayIndex).SetUint(uin)
					case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
						in, err := parseInt(data)
						if err != nil {
							d.err = err
							return
						}
						v.Index(arrayIndex).SetInt(in)
					case reflect.Float32, reflect.Float64:
						f, err := parseFloat(data)
						if err != nil {
							d.err = err
							return
						}
						v.Index(arrayIndex).SetFloat(f)
					default:
					}
				case Bool:
					b, err := parseBool(data)
					if err != nil {
						d.err = err
						return
					}
					v.Type()
					v.Set(reflect.Append(v, reflect.ValueOf(b)))
				case String:
					s, err := parseString(data)
					if err != nil {
						d.err = err
						return
					}
					v.Set(reflect.Append(v, reflect.ValueOf(s)))
				case Nil:
					err := parseNil(data)
					if err != nil {
						d.err = err
						return
					}
				}
				if terminator == ',' {
					d.consumeByte(terminator)
				}
			} else {
				d.err = newError(fmt.Sprintf("Expected object, array or primitive, got %s in loop num %d", string(d.peekCurrent()), arrayIndex))
				return
			}
		}
		arrayIndex++
		d.consumeWhitespace()

		if d.peekCurrent() == ']' {
			break
		}

		if arrayIndex < v.Len() {
			if v.Kind() == reflect.Array {
				z := reflect.Zero(v.Type().Elem())
				for ; arrayIndex < v.Len(); arrayIndex++ {
					v.Index(arrayIndex).Set(z)
				}
			}
		} else {
			v.SetLen(arrayIndex)
		}
	}

	d.consumeWhitespace()
	d.consumeByte(']')

	return
}

func (d *deserializer) deserializeObject(v reflect.Value) {
	d.consumeByte('{')
	d.consumeWhitespace()

	if d.hasError() {
		return
	}

	for {
		if d.peekCurrent() == '}' {
			break
		}

		key := d.consumeKey()
		if d.hasError() {
			return
		}

		keyFieldValue := v.FieldByName(key)
		if !keyFieldValue.IsValid() {
			t := reflect.TypeOf(v.Interface())
			d.err = newError(fmt.Sprintf("Key: %s, could not be found in the interface: %v", key, t.Name()))
			return
		}

		if d.peekCurrent() == '[' {

			d.deserializeArray(keyFieldValue)
			if d.hasError() {
				return
			}
			if d.peekCurrent() == ',' {
				d.consumeByte(',')
				d.consumeWhitespace()
			}
		} else if d.peekCurrent() == '{' {
			d.deserializeObject(keyFieldValue)
			if d.hasError() {
				return
			}
			if d.peekCurrent() == ',' {
				d.consumeByte(',')
				d.consumeWhitespace()
			}
		} else if d.isPrimitive() {
			primitiveType, err := d.parsePrimitiveType()
			if err != nil {
				d.err = err
				return
			}

			data, terminator := d.consumeUntilTerminator()
			_ = terminator
			switch primitiveType {
			case Number:
				switch v.Kind() {
				case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
					uin, err := parseUint(data)
					if err != nil {
						d.err = err
						return
					}
					keyFieldValue.SetUint(uin)
				case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
					in, err := parseInt(data)
					if err != nil {
						d.err = err
						return
					}
					keyFieldValue.SetInt(in)
				case reflect.Float32, reflect.Float64:
					f, err := parseFloat(data)
					if err != nil {
						d.err = err
						return
					}
					keyFieldValue.SetFloat(f)
				default:
				}
			case Bool:
				b, err := parseBool(data)
				if err != nil {
					d.err = err
					return
				}
				keyFieldValue.SetBool(b)
			case String:
				s, err := parseString(data)
				if err != nil {
					d.err = err
					return
				}
				keyFieldValue.SetString(s)
			case Nil:
				err := parseNil(data)
				if err != nil {
					d.err = err
					return
				}
			}
			if terminator == ',' {
				d.consumeByte(terminator)
			}
		} else {
			d.err = newError(fmt.Sprintf("Expected object, array or primitive, got %s", string(d.peekCurrent())))
			return
		}
		d.consumeWhitespace()
	}
	d.consumeWhitespace()
	d.consumeByte('}')

	return
}

func parseFloat(value []byte) (float64, error) {
	return strconv.ParseFloat(string(value), 64)
}

func parseInt(value []byte) (int64, error) {
	return strconv.ParseInt(string(value), 10, 64)
}

func parseUint(value []byte) (uint64, error) {
	return strconv.ParseUint(string(value), 10, 64)
}

func parseBool(value []byte) (bool, error) {
	if len(value) == 0 {
		return false, newError("Value array is empty")
	}

	strValue := string(value)
	if strValue == "true" {
		return true, nil
	} else if strValue == "false" {
		return false, nil
	} else {
		return false, newError(fmt.Sprintf("Expected true or false, got %s", strValue))
	}
}

func parseString(value []byte) (string, error) {
	if value[0] != '"' {
		return "", newError(fmt.Sprintf("Expected string to start with \" but started with %v", value[0]))
	}
	value = value[1:]

	if value[len(value)-1] != '"' {
		return "", newError(fmt.Sprintf("Expected string to end with \" but ended with %v", value[len(value)-1]))
	}
	value = value[:len(value)-1]

	strValue := unescapeString(string(value))

	return strValue, nil
}

func unescapeString(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		delimiter := s[i]
		strDelimiter := string(delimiter)
		_ = strDelimiter
		i++

		if delimiter == '\\' {
			ch := s[i]
			strCh := string(ch)
			_ = strCh
			i++
			if ch == '\\' || ch == '/' || ch == '"' || ch == '\'' {
				sb.WriteByte(ch)
			} else if ch == 'n' {
				sb.WriteByte('\n')
			} else if ch == 'r' {
				sb.WriteByte('\r')
			} else if ch == 'b' {
				sb.WriteByte('\b')
			} else if ch == 't' {
				sb.WriteByte('\t')
			} else if ch == 'f' {
				sb.WriteByte('\f')
			}
		} else {
			sb.WriteByte(delimiter)
		}
	}
	return sb.String()
}

func parseNil(value []byte) error {
	if len(value) == 0 {
		return newError("Value array is empty")
	}

	if string(value) != "null" {
		return newError(fmt.Sprintf("Expected null, got %s", string(value)))
	}
	return nil
}

func (d *deserializer) isPrimitive() bool {
	// There are no keys in arrays, so just look for any literal character or the start of a string
	b := d.peekCurrent()
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '"' || b == '-'
}

/*
	Whitespace should be consumed before this function is called, therefore peeking into whitespace will throw an
	error
*/
func (d *deserializer) parsePrimitiveType() (PrimitiveType, error) {

	if isWhitespace(d.peekCurrent()) {
		return Invalid, newError("Peeked into whitespace")
	}

	switch d.peekCurrent() {
	case '"':
		return String, nil
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return Number, nil
	case 't', 'f':
		return Bool, nil
	case 'n':
		return Nil, nil
	default:
		return Invalid, newError(fmt.Sprintf("Expected string, number, bool or nil, got %v", d.peekCurrent()))
	}
}

func (d *deserializer) consumeByte(b byte) {
	if d.atEof(0) {
		return
	}
	if d.json[d.pos] != b {
		d.err = newError(fmt.Sprintf("Expected %s Got %s", string(b), string(d.json[d.pos])))
		return
	}
	d.consume()
	return
}

/*
	Returns the []byte leading up to the terminator and the terminator
	NOTE: Does note consume terminator
*/
func (d *deserializer) consumeUntilTerminator() ([]byte, byte) {
	var sb strings.Builder
	b := d.peekCurrent()
	for b != '}' && b != ',' && b != ']' {
		sb.WriteByte(b)
		d.consume()
		b = d.peekCurrent()
	}

	return bytes.TrimRight([]byte(sb.String()), " \n\t\r"), b
}

func (d *deserializer) consumeUntil(b byte) string {
	if d.atEof(0) {
		return ""
	}
	var sb strings.Builder
	for d.json[d.pos] != b {
		sb.WriteByte(d.json[d.pos])
		d.pos++
		if d.atEof(0) {
			return ""
		}
	}
	return sb.String()
}

func (d *deserializer) consumeKey() string {
	d.consumeByte('"')
	s := d.consumeUntil('"')
	d.consumeByte('"')
	d.consumeByte(':')
	d.consumeWhitespace()
	return s
}

func (d *deserializer) isNull() bool {
	if d.atEof(4) {
		return false
	}
	return string(d.peek(4)) == "null"
}

func isWhitespace(b byte) bool {
	return b == '\t' ||
		b == ' ' ||
		b == '\n' ||
		b == '\r'
}

func (d *deserializer) consumeWhitespace() {
	for !d.atEof(0) && isWhitespace(d.json[d.pos]) {
		d.consume()
	}
}

func (d *deserializer) consume() byte {
	// return d.json[d.pos++]
	if d.atEof(0) {
		return 0
	}
	b := d.json[d.pos]
	d.pos++
	return b
}

func (d *deserializer) peek(offset int) []byte {
	if d.atEof(1 + offset) {
		return []byte{}
	}
	return d.json[d.pos : d.pos+1+offset]
}

func (d *deserializer) peekCurrent() byte {
	if d.atEof(0) {
		return 0
	}
	return d.json[d.pos]
}

func (d *deserializer) atEof(offset int) bool {
	if d.pos+offset > d.eof {
		d.err = newError(eof)
		return true
	} else {
		return false
	}
}

const eof = "unexpected end of JSON input"

func (d *deserializer) hasError() bool {
	return d.err != nil
}
