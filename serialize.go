package json

import (
	_ "fmt"
	"reflect"
	"strconv"
	"strings"
)

func Serialize(i interface{}) (string, error) {
	s := makeSerializer(i)
	err := s.serialize()
	if err != nil {
		return "", err
	}
	return s.json.String(), nil
}

func makeSerializer(i interface{}) serializer {
	var b strings.Builder
	s := serializer{b, i, 0}
	return s
}

type serializer struct {
	json        strings.Builder
	i           interface{}
	indentLevel int
}

func (s *serializer) serialize() error {
	interfaceValue := reflect.ValueOf(s.i)
	_ = interfaceValue
	interfaceType := reflect.TypeOf(s.i)

	if interfaceType.Kind() == reflect.Struct {
		err := s.serializeObject(s.i)
		if err != nil {
			return err
		}
	} else if isJsonArray(interfaceType.Kind()) {
		err := s.serializeArray(s.i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *serializer) serializeArray(i interface{}) error {
	interfaceValue := reflect.ValueOf(i)
	// interfaceType := reflect.TypeOf(i)

	if !interfaceValue.IsNil() {
		s.startArray()
		for arrayIndex := 0; arrayIndex < interfaceValue.Len(); arrayIndex++ {
			if interfaceValue.Index(arrayIndex).Kind() == reflect.Struct {
				err := s.serializeObject(interfaceValue.Index(arrayIndex).Interface())
				if err != nil {
					return err
				}
			} else if isPrimitive(interfaceValue.Index(arrayIndex).Kind()) {
				err := s.serializePrimitive(interfaceValue.Index(arrayIndex))
				if err != nil {
					return err
				}
			} else if isJsonArray(interfaceValue.Index(arrayIndex).Kind()) {
				err := s.serializeArray(interfaceValue.Index(arrayIndex).Interface())
				if err != nil {
					return err
				}
			}
			if arrayIndex < interfaceValue.Len()-1 {
				s.appendComma()
			}
		}
		s.endArray()
	} else {
		s.json.WriteString("null")
	}

	return nil
}

func (s *serializer) serializeObject(i interface{}) error {
	interfaceValue := reflect.ValueOf(i)
	interfaceType := reflect.TypeOf(i)

	s.startObject()

	for i := 0; i < interfaceValue.NumField(); i++ {
		fieldValue := interfaceValue.Field(i)
		fieldType := interfaceType.Field(i)

		s.appendKey(fieldType)

		if isPrimitive(fieldValue.Kind()) {
			err := s.serializePrimitive(fieldValue)
			if err != nil {
				return err
			}
		} else if fieldValue.Kind() == reflect.Struct {
			err := s.serializeObject(fieldValue.Interface())
			if err != nil {
				return err
			}
		} else if isJsonArray(fieldValue.Kind()) {
			err := s.serializeArray(fieldValue.Interface())
			if err != nil {
				return err
			}
		} else {
			panic("Unserializable Type: " + fieldValue.Kind().String())
		}

		// If there is another field after this current field, append a comma
		if s.shouldAppendComma(interfaceValue, i) {
			s.appendComma()
		}
	}

	s.endObject()
	return nil
}

func (s *serializer) startObject() {
	s.json.WriteString("{\n")
	s.indentLevel++
	s.appendTabs()
}

func (s *serializer) endObject() {
	s.json.WriteString("\n")
	s.indentLevel--
	s.appendTabs()
	s.json.WriteString("}")
}

func (s *serializer) startArray() {
	s.json.WriteString("[\n")
	s.indentLevel++
	s.appendTabs()
}

func (s *serializer) endArray() {
	s.json.WriteString("\n")
	s.indentLevel--
	s.appendTabs()
	s.json.WriteString("]")
}

func (s *serializer) appendTabs() {
	for i := 0; i < s.indentLevel; i++ {
		s.json.WriteByte('\t')
	}
}

func (s *serializer) shouldAppendComma(structValue reflect.Value, fieldIndex int) bool {
	return fieldIndex < structValue.NumField()-1
}

func (s *serializer) appendComma() {
	s.json.WriteByte(',')
	s.json.WriteByte('\n')
	s.appendTabs()
}

func (s *serializer) serializePrimitive(fieldValue reflect.Value) error {
	switch fieldValue.Kind() {
	case reflect.Bool:
		s.json.WriteString(strconv.FormatBool(fieldValue.Bool()))
		break
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		s.json.WriteString(strconv.FormatInt(fieldValue.Int(), 10))
		break
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		s.json.WriteString(strconv.FormatUint(fieldValue.Uint(), 10))
		break
	case reflect.Float32, reflect.Float64:
		s.json.WriteString(strconv.FormatFloat(fieldValue.Float(), 'f', -1, fieldValue.Type().Bits()))
		break
	case reflect.String:
		s.json.WriteString("\"")
		str := fieldValue.String()
		escapedString := escapeString(str)
		s.json.WriteString(escapedString)
		s.json.WriteString("\"")
		break
	}
	return nil
}

func escapeString(s string) string {
	var sb strings.Builder
	for i := range s {
		c := s[i]
		switch c {
		case '\\':
			fallthrough
		case '"':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '/':
			sb.WriteByte('\\')
			sb.WriteByte(c)
		case '\b':
			sb.WriteString("\\b")
		case '\t':
			sb.WriteString("\\t")
		case '\n':
			sb.WriteString("\\n")
		case '\f':
			sb.WriteString("\\f")
		case '\r':
			sb.WriteString("\\r")
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func (s *serializer) appendKey(field reflect.StructField) {
	s.json.WriteString("\"")
	s.json.WriteString(field.Name)
	s.json.WriteString("\": ")
}
