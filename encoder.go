package data

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// Encoder 用来将数据转化成 Data。
type Encoder struct {
	TagName   string // 在解析 struct 时候使用的 field tag，默认是 data。
	OmitEmpty bool   // 如果为 true，则默认所有字段都会忽略空值。
}

// Encode 将任意的 Go 类型转化成 Data。
//
// 需要注意，只有以下类型可以成功转化成 Data，如果 v 不是这些类型，Encode 会返回 nil。
//     - Go struct 和 struct 指针；
//     - 任意的 map[string]T 类型，T 可以是任意的类型。
func (enc *Encoder) Encode(v interface{}) Data {
	if v == nil {
		return emptyData
	}

	val := reflect.ValueOf(v)
	t := val.Type()

	for kind := t.Kind(); kind == reflect.Ptr || kind == reflect.Interface; kind = t.Kind() {
		t = t.Elem()
		val = val.Elem()
	}

	d := enc.encodeValue(val)
	return Data{
		data: d,
	}
}

func (enc *Encoder) encodeValue(val reflect.Value) RawData {
	switch val.Kind() {
	case reflect.Map:
		return enc.encodeMap(val)
	case reflect.Struct:
		return enc.encodeStruct(val)
	}

	return nil
}

func (enc *Encoder) encodeMap(val reflect.Value) RawData {
	t := val.Type()

	if t.Key().Kind() != reflect.String {
		return nil
	}

	d := RawData{}

	if val.Len() == 0 {
		return nil
	}

	iter := val.MapRange()

	for iter.Next() {
		k := iter.Key()
		v := iter.Value()

		d[k.String()] = enc.encodeMapValue(v)
	}

	return d
}

func (enc *Encoder) encodeStruct(val reflect.Value) RawData {
	d := RawData{}
	enc.encodeStructToData(val, d)
	return d
}

func (enc *Encoder) encodeStructToData(val reflect.Value, d RawData) {
	if val.Type().AssignableTo(typeOfData) {
		merge(reflect.ValueOf(d), val.Convert(typeOfData).Interface().(Data).data)
		return
	}

	t := val.Type()
	l := t.NumField()

	for i := 0; i < l; i++ {
		f := t.Field(i)
		tagName := enc.TagName

		if tagName == "" {
			tagName = defaultTagName
		}

		tag := f.Tag.Get(tagName)
		ft := ParseFieldTag(tag)

		if ft.Skipped {
			continue
		}

		k := f.Name

		if ft.Alias != "" {
			k = ft.Alias
		}

		fv := val.Field(i)
		v := enc.encodeMapValue(fv)

		if (ft.OmitEmpty || enc.OmitEmpty) && isEmpty(v) {
			continue
		}

		// 如果需要合并字段，且 v 是一个 Data，那么会将 v 内容浅拷贝到 d 里面。
		if ft.Squash {
			if data, ok := v.(RawData); ok {
				for k, v := range data {
					d[k] = v
				}

				continue
			}
		}

		d[k] = v
	}
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)

	if val.IsZero() {
		return true
	}

	switch val.Kind() {
	case reflect.Map, reflect.Slice:
		return val.Len() == 0
	}

	return false
}

func (enc *Encoder) encodeMapValue(val reflect.Value) interface{} {
	if !val.IsValid() {
		return nil
	}

	switch val.Type() {
	case typeOfTime:
		return val.Interface()
	case typeOfDuration:
		if val.Int() == 0 {
			return ""
		}

		return val.Interface().(fmt.Stringer).String()
	}

	switch val.Kind() {
	// 由于需要保持 Data 结构在序列化和反序列化的时候内容稳定，所以将所有的基础类型都统一成最大的类型。
	// 例如所有的 int* 都变成 int64。

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int()

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return val.Uint()

	case reflect.Float32, reflect.Float64:
		return val.Float()

	case reflect.Complex64, reflect.Complex128:
		return val.Complex()

	case reflect.Invalid:
		return nil

	case reflect.String:
		// 需要特别的支持 json.Number，将这种字符串变成数字。
		if num, ok := val.Interface().(json.Number); ok {
			i64, err := num.Int64()

			if err == nil {
				return i64
			}

			f64, err := num.Float64()

			if err == nil {
				return f64
			}

			// 如果不是合法的数字，将这个类型还原成普通的 string。
			return string(num)
		}

	case reflect.Array, reflect.Slice:
		l := val.Len()
		sliceType := reflect.SliceOf(toLargestType(val.Type().Elem()))
		values := reflect.MakeSlice(sliceType, l, l)

		for i := 0; i < l; i++ {
			v := enc.encodeMapValue(val.Index(i))
			values.Index(i).Set(reflect.ValueOf(v))
		}

		return values.Interface()

	case reflect.Interface, reflect.Ptr:
		val = val.Elem()
		return enc.encodeMapValue(val)

	case reflect.Map:
		t := val.Type()
		kt := t.Key()

		if k := kt.Kind(); k != reflect.String {
			return val.Interface()
		}

		d := RawData{}

		if val.Len() == 0 {
			return d
		}

		iter := val.MapRange()

		for iter.Next() {
			k := iter.Key()
			v := iter.Value()

			d[k.String()] = enc.encodeMapValue(v)
		}

		return d

	case reflect.Struct:
		return enc.encodeStruct(val)

	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// 这些类型不是数据。
		return nil
	}

	return val.Interface()
}

func toLargestType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return typeOfInt64
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return typeOfUint64
	case reflect.Float32, reflect.Float64:
		return typeOfFloat64
	case reflect.Complex64, reflect.Complex128:
		return typeOfComplex128
	case reflect.Struct:
		return typeOfObject
	}

	return t
}
