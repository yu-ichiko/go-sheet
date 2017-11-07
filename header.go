package sheet

import (
	"fmt"
	"reflect"
	"unicode"
)

type headerCell struct {
	column int
	row    int
	key    string
	title  string
}

type HeaderEncoder struct {
	cells []headerCell
}

func NewHeaderEncoder() *HeaderEncoder {
	return &HeaderEncoder{
		cells: []headerCell{},
	}
}

func (enc *HeaderEncoder) Encode(v interface{}) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		rv = rv.Elem()
	}

	enc.encode(rv)
}

func (enc *HeaderEncoder) encode(v reflect.Value) {
	n := 0
	for i := 0; i < v.Type().NumField(); i++ {
		field := v.Type().Field(i)
		if !unicode.IsUpper(rune(field.Name[0])) {
			continue
		}
		tag := field.Tag.Get(tagName)
		if tag == "-" {
			continue
		}
		key := field.Name
		opt := newOption(tag)
		if opt.isDatetime {
			key += ":datetime"
		}
		fmt.Println(key)
		n++
	}
}

func (enc *HeaderEncoder) add(v reflect.Value, column, row int) {
	enc.cells = append(enc.cells, headerCell{})
}
