package sheet

import (
	"bytes"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unicode"
)

var csvPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

func getCSVPool() *bytes.Buffer {
	return csvPool.Get().(*bytes.Buffer)
}

func resetCSVPool(buf *bytes.Buffer) {
	buf.Truncate(0)
	csvPool.Put(buf)
}

var cellsPool = sync.Pool{
	New: func() interface{} {
		return &cells{
			list: make([]cell, 0, 1024),
		}
	},
}

func getCellPool() *cells {
	return cellsPool.Get().(*cells)
}

func resetCellPool(cells *cells) {
	cells.truncate()
	cellsPool.Put(cells)
}

type cell struct {
	column int
	row    int
	value  interface{}
}

type cells struct {
	list []cell
}

func (c *cells) add(cell cell) {
	c.list = append(c.list, cell)
}

func (c *cells) truncate() {
	c.list = c.list[:0]
}

type encoder struct {
	cells     *cells
	maxColumn int
	maxRow    int
}

func newEncoder() *encoder {
	return &encoder{
		maxColumn: 0,
		maxRow:    0,
	}
}

func (enc *encoder) init() {
	enc.cells = getCellPool()
	enc.maxColumn = 0
	enc.maxRow = 0
}

func (enc *encoder) reset() {
	resetCellPool(enc.cells)
}

func (enc *encoder) Encode(v interface{}) ([][]interface{}, error) {
	enc.init()
	defer enc.reset()
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		rv = rv.Elem()
	}
	if _, err := enc.reflectStruct(rv, 0, 0, false); err != nil {
		return nil, err
	}
	values := make([][]interface{}, enc.maxRow+1)
	for i := range values {
		values[i] = make([]interface{}, enc.maxColumn+1)
	}
	for _, cell := range enc.cells.list {
		values[cell.row][cell.column] = cell.value
	}
	return values, nil
}

func (enc *encoder) reflectStruct(v reflect.Value, column, row int, isNil bool) (int, error) {
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
		opt := newOption(tag, false)
		addNum, err := enc.reflectValue(v.Field(i), column+n, row, opt, isNil)
		if err != nil {
			return 0, err
		}
		if addNum > 0 {
			n += addNum
		} else {
			n++
		}
		resetOption(opt)
	}
	return n, nil
}

func (enc *encoder) reflectList(v reflect.Value, isStruct bool, column, row int, opt *option, isNil bool) (int, error) {
	col := 0
	if opt.isCSV && !isStruct {
		buf := getCSVPool()
		for i := 0; i < v.Len(); i++ {
			switch v.Index(i).Kind() {
			case reflect.String:
				buf.WriteString(v.Index(i).String())
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				buf.WriteString(strconv.FormatInt(v.Index(i).Int(), 10))
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				buf.WriteString(strconv.FormatUint(v.Index(i).Uint(), 10))
			case reflect.Float32:
				buf.WriteString(strconv.FormatFloat(v.Index(i).Float(), 'f', -1, 32))
			case reflect.Float64:
				buf.WriteString(strconv.FormatFloat(v.Index(i).Float(), 'e', -1, 64))
			case reflect.Bool:
				buf.WriteString(strconv.FormatBool(v.Index(i).Bool()))
			}
			if i < v.Len()-1 {
				buf.WriteString(",")
			}
		}
		enc.add(buf.String(), column, row)
		resetCSVPool(buf)
	} else {
		for i := 0; i < v.Len(); i++ {
			n := 0
			if isStruct {
				enc.add(i+1, column, row+i)
				n = 1
			}
			n, err := enc.reflectValue(v.Index(i), column+n, row+i, opt, isNil)
			if err != nil {
				return 0, err
			}
			if col < n {
				col = n
			}
		}
	}
	return col, nil
}

func (enc *encoder) reflectValue(v reflect.Value, column, row int, opt *option, isNil bool) (int, error) {
	switch v.Kind() {
	case reflect.Ptr:
		isNil = v.IsNil()
		if isNil {
			v = reflect.New(v.Type().Elem())
		}
		n, err := enc.reflectValue(v.Elem(), column, row, opt, isNil)
		if err != nil {
			return 0, err
		}
		return n, nil
	case reflect.Struct:
		switch v.Type() {
		case typeOfTime:
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
			n, err := enc.reflectStruct(v, column, row, isNil)
			if err != nil {
				return 0, err
			}
			return n, nil
		}
	case reflect.Array:
		isStruct := v.Type().Elem().Kind() == reflect.Struct
		col, err := enc.reflectList(v, isStruct, column, row, opt, isNil)
		if err != nil {
			return 0, err
		}
		if isStruct {
			col++
		}
		return col, nil
	case reflect.Slice:
		col := 0
		isStruct := v.Type().Elem().Kind() == reflect.Struct
		if v.Len() > 0 {
			var err error
			col, err = enc.reflectList(v, isStruct, column, row, opt, isNil)
			if err != nil {
				return 0, err
			}
		} else {
			v := reflect.New(v.Type().Elem()).Elem()
			n := 0
			if isStruct {
				enc.add(nil, column, row)
				n = 1
			}
			n, err := enc.reflectValue(v, column+n, row, opt, true)
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
	} else if isNil {
		enc.add(nil, column, row)
	} else {
		switch v.Kind() {
		case reflect.String:
			enc.add(v.String(), column, row)
		case reflect.Int:
			enc.add(int(v.Int()), column, row)
		case reflect.Int8:
			enc.add(int8(v.Int()), column, row)
		case reflect.Int16:
			enc.add(int16(v.Int()), column, row)
		case reflect.Int32:
			enc.add(int32(v.Int()), column, row)
		case reflect.Int64:
			enc.add(v.Int(), column, row)
		case reflect.Uint:
			enc.add(uint(v.Uint()), column, row)
		case reflect.Uint8:
			enc.add(uint8(v.Uint()), column, row)
		case reflect.Uint16:
			enc.add(uint16(v.Uint()), column, row)
		case reflect.Uint32:
			enc.add(uint32(v.Uint()), column, row)
		case reflect.Uint64:
			enc.add(v.Uint(), column, row)
		case reflect.Float32:
			enc.add(float32(v.Float()), column, row)
		case reflect.Float64:
			enc.add(v.Float(), column, row)
		case reflect.Bool:
			enc.add(v.Bool(), column, row)
		}
	}
	return 0, nil
}

func (enc *encoder) add(v interface{}, column, row int) {
	enc.cells.add(cell{
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
