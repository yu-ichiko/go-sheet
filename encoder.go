package sheet

import (
	"reflect"
	"time"
	"unicode"
)

type Encoder struct {
	cells     []cell
	maxColumn int
	maxRow    int
}

func NewEncoder() *Encoder {
	return &Encoder{
		cells:     []cell{},
		maxColumn: 0,
		maxRow:    0,
	}
}

func (enc *Encoder) Encode(v interface{}) ([][]interface{}, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	err := enc.encode(rv)
	values := make([][]interface{}, enc.maxRow+1)
	for i := range values {
		values[i] = make([]interface{}, enc.maxColumn+1)
	}
	for _, cell := range enc.cells {
		values[cell.row][cell.column] = cell.value
	}
	return values, err
}

func (enc *Encoder) encode(v reflect.Value) error {
	if _, err := enc.reflectStruct(v, 0, 0); err != nil {
		return err
	}
	return nil
}

func (enc *Encoder) reflectStruct(v reflect.Value, column, row int) (int, error) {
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
		opt := genOption(tag)
		value := v.Field(i)
		if value.Kind() == reflect.Ptr && !value.IsNil() {
			value = value.Elem()
		}
		addNum, err := enc.reflectValue(value, column+n, row, opt)
		if err != nil {
			return 0, err
		}
		if addNum > 0 {
			n += addNum
		} else {
			n++
		}
	}
	return n, nil
}

func (enc *Encoder) reflectList(v reflect.Value, isStruct bool, column int, opt *option) (int, error) {
	col := 0
	for i := 0; i < v.Len(); i++ {
		n := 0
		if isStruct {
			enc.add(i+1, column, i)
			n = 1
		}
		n, err := enc.reflectValue(v.Index(i), column+n, i, opt)
		if err != nil {
			return 0, err
		}
		if col < n {
			col = n
		}
	}
	return col, nil
}

func (enc *Encoder) reflectValue(v reflect.Value, column, row int, opt *option) (int, error) {
	switch v.Interface().(type) {
	case time.Time:
		if opt != nil && opt.isDatetime {
			t, err := encodeDatetime(v)
			if err != nil {
				return 0, err
			}
			enc.add(t, column, row)
		} else {
			val := v.Interface().(time.Time)
			txt, err := val.MarshalText()
			if err != nil {
				return 0, err
			}
			enc.add(string(txt), column, row)
		}
	default:
		switch v.Kind() {
		case reflect.Ptr:
			if !v.IsNil() {
				n, err := enc.reflectValue(v.Elem(), column, row, opt)
				if err != nil {
					return 0, err
				}
				return n, nil
			}
		case reflect.Struct:
			n, err := enc.reflectStruct(v, column, row)
			if err != nil {
				return 0, err
			}
			return n, nil
		case reflect.Array:
			rv := reflect.New(v.Type()).Elem().Index(0)
			isStruct := rv.Kind() == reflect.Struct
			col, err := enc.reflectList(v, isStruct, column, opt)
			if err != nil {
				return 0, err
			}
			if isStruct {
				col++
			}
			return col, nil
		case reflect.Slice:
			col := 0
			rv := reflect.MakeSlice(v.Type(), 1, 1).Index(0)
			isStruct := rv.Kind() == reflect.Struct
			l := v.Len()
			if l > 0 {
				var err error
				col, err = enc.reflectList(v, isStruct, column, opt)
				if err != nil {
					return 0, err
				}
			} else {
				n := 0
				if isStruct {
					enc.add(0, column, row)
					n = 1
				}
				n, err := enc.reflectValue(rv, column+n, row, opt)
				if err != nil {
					return 0, err
				}
				if col < n {
					col = n
				}
			}
			if isStruct {
				col++
			}
			return col, nil
		}
		if opt != nil && opt.isDatetime {
			t, err := encodeDatetime(v)
			if err != nil {
				return 0, err
			}
			enc.add(t, column, row)
		} else {
			switch v.Kind() {
			case reflect.String:
				enc.add(v.String(), column, row)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				enc.add(v.Int(), column, row)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				enc.add(v.Uint(), column, row)
			case reflect.Float32, reflect.Float64:
				enc.add(v.Float(), column, row)
			case reflect.Bool:
				enc.add(v.Bool(), column, row)
			}
		}
	}
	return 0, nil
}

func (enc *Encoder) add(v interface{}, column, row int) {
	enc.cells = append(enc.cells, cell{
		column: column,
		row:    row,
		value:  v,
	})
	if enc.maxColumn < column {
		enc.maxColumn = column
	}
	if enc.maxRow < row {
		enc.maxRow = row
	}
}
