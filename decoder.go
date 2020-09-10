package data

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/huandu/go-clone"
)

// Decoder 用来将 Data 设置到指定值里面去。
type Decoder struct {
	TagName string // 在解析 struct 时候使用的 field tag，默认是 data。
}

// Decode 将 d 解析到 v 中。
func (dec *Decoder) Decode(d Data, v interface{}) error {
	from := reflect.ValueOf(d.data)
	to := reflect.ValueOf(v)
	return dec.decode(from, to)
}

// DecodeQuery 解析 query 找到对应的值并且解析到 v 中。
// 其中，query 的格式详见 `Data#Qeury` 文档。
func (dec *Decoder) DecodeQuery(d Data, query string, v interface{}) error {
	from := reflect.ValueOf(d.Query(query))
	to := reflect.ValueOf(v)
	return dec.decode(from, to)
}

// DecodeField 通过 field 找到对应的值并且解析到 v 中。
// 其中，field 的格式详见 `Data#Get` 文档。
func (dec *Decoder) DecodeField(d Data, field []string, v interface{}) error {
	from := reflect.ValueOf(d.Get(field...))
	to := reflect.ValueOf(v)
	return dec.decode(from, to)
}

// decode 将 from 中的内容解析到 to 中去。
//
// 其中，to 必须可以通过反射设置值（例如输入的是一个指针），否则会返回错误。
//
// 由于 decode 仅在内部使用，这里会假定 from 要么是 Data，要么是已经 Data 里已经解析过的值，
// 因此 from 不可能是、也不可能包含任何 struct、chan、func、ptr 等不是数据的值。
func (dec *Decoder) decode(from reflect.Value, to reflect.Value) error {
	if to.Kind() == reflect.Ptr {
		to = to.Elem()
	}

	if !to.IsValid() {
		return errors.New("go-data: cannot decode to an invalid value")
	}

	if !to.CanSet() {
		return fmt.Errorf("go-data: cannot decode to a value of type %v which is not settable", to.Type())
	}

	// 如果 from == nil，那么直接跳过解析过程，同时也不报错。
	switch from.Kind() {
	case reflect.Invalid:
		return nil
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map:
		if from.IsNil() {
			return nil
		}
	}

	for to.Kind() == reflect.Ptr {
		if to.IsNil() {
			to.Set(reflect.New(to.Type().Elem()))
		}

		to = to.Elem()
	}

	for from.Kind() == reflect.Interface {
		from = from.Elem()
	}

	// 先处理一些知名类型。
	switch to.Type() {
	case typeOfDuration:
		if from.Kind() != reflect.String {
			return fmt.Errorf("go-data: cannot decode a value of type %v from %v", to.Type(), from.Type())
		}

		if str := from.String(); str == "" {
			to.SetInt(0)
		} else {
			dur, err := time.ParseDuration(from.String())

			if err != nil {
				return err
			}

			to.SetInt(int64(dur))
		}

		return nil

	case typeOfTime:
		if from.Type() != typeOfTime {
			return fmt.Errorf("go-data: cannot decode a value of type %v from %v", to.Type(), from.Type())
		}

		to.Set(from)
		return nil
	}

	// 再处理通用的类型。
	switch to.Kind() {
	case reflect.Bool:
		switch from.Kind() {
		case reflect.Bool:
			to.SetBool(from.Bool())
			return nil
		}

	case reflect.String:
		switch from.Kind() {
		case reflect.String:
			to.SetString(from.String())
			return nil
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch from.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := from.Int()

			if to.OverflowInt(i) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), i)
			}

			to.SetInt(i)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			ui := from.Uint()
			i := int64(ui)

			if ui > math.MaxInt64 || to.OverflowInt(i) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), ui)
			}

			to.SetInt(i)
			return nil
		case reflect.Float32, reflect.Float64:
			f := from.Float()
			i := int64(f)

			if f != math.Round(f) {
				return fmt.Errorf("go-data: cannot decode value of type %v from a float number %v", to.Type(), f)
			}

			if f > math.MaxInt64 || to.OverflowInt(i) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), f)
			}

			to.SetInt(i)
			return nil
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		switch from.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := from.Int()
			ui := uint64(i)

			if i < 0 || to.OverflowUint(ui) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), i)
			}

			to.SetUint(ui)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			ui := from.Uint()

			if to.OverflowUint(ui) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), ui)
			}

			to.SetUint(ui)
			return nil
		case reflect.Float32, reflect.Float64:
			f := from.Float()
			ui := uint64(f)

			if f != math.Round(f) {
				return fmt.Errorf("go-data: cannot decode value of type %v from a float number %v", to.Type(), f)
			}

			if f < 0 || f > math.MaxUint64 || to.OverflowUint(ui) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), f)
			}

			to.SetUint(ui)
			return nil
		}

	case reflect.Float32, reflect.Float64:
		switch from.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			i := from.Int()
			f := float64(i)
			to.SetFloat(f)
			return nil
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			ui := from.Uint()
			f := float64(ui)
			to.SetFloat(f)
			return nil
		case reflect.Float32, reflect.Float64:
			f := from.Float()

			if to.OverflowFloat(f) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), f)
			}

			to.SetFloat(f)
			return nil
		}

	case reflect.Complex64, reflect.Complex128:
		switch from.Kind() {
		case reflect.Complex64, reflect.Complex128:
			cmplx := from.Complex()

			if to.OverflowComplex(cmplx) {
				return fmt.Errorf("go-data: cannot decode value of type %v from %v due to overflow", to.Type(), cmplx)
			}

			to.SetComplex(cmplx)
			return nil
		}

	case reflect.Array:
		switch from.Kind() {
		case reflect.Array, reflect.Slice:
			fromLen := from.Len()
			toLen := to.Len()

			if fromLen > toLen {
				return fmt.Errorf("go-data: cannot decode value of type %v due to no enough room to store %v element(s)", to.Type(), fromLen)
			}

			for i := 0; i < fromLen; i++ {
				v := to.Index(i)

				if err := dec.decode(from.Index(i), v); err != nil {
					return err
				}
			}

			return nil
		}

	case reflect.Slice:
		switch from.Kind() {
		case reflect.Array, reflect.Slice:
			fromLen := from.Len()
			toType := to.Type()
			val := reflect.MakeSlice(toType, fromLen, fromLen)

			for i := 0; i < fromLen; i++ {
				v := val.Index(i)

				if err := dec.decode(from.Index(i), v); err != nil {
					return err
				}
			}

			to.Set(val)
			return nil
		}

	case reflect.Map:
		switch from.Kind() {
		case reflect.Map:
			toType := to.Type()
			toKeyType := toType.Key()
			toElemType := toType.Elem()

			if toKeyType.Kind() != reflect.String {
				return fmt.Errorf("go-data: cannot decode a value of type %v whose key is not string", to.Type())
			}

			val := reflect.MakeMap(toType)
			iter := from.MapRange()

			for iter.Next() {
				v := reflect.New(toElemType).Elem()

				if err := dec.decode(iter.Value(), v.Addr()); err != nil {
					return err
				}

				val.SetMapIndex(iter.Key(), v)
			}

			to.Set(val)
			return nil
		}

	case reflect.Struct:
		if to.Type().AssignableTo(typeOfData) {
			d := Data{}

			if err := dec.decode(from, reflect.ValueOf(&d.data)); err != nil {
				return err
			}

			if d.Len() == 0 {
				d = emptyData
			}

			to.Set(reflect.ValueOf(d))
			return nil
		}

		switch from.Kind() {
		case reflect.Map:
			numField := to.NumField()
			toType := to.Type()

			for i := 0; i < numField; i++ {
				f := toType.Field(i)
				fv := to.Field(i)

				if !fv.CanSet() || !fv.CanAddr() {
					continue
				}

				tagName := dec.TagName

				if tagName == "" {
					tagName = defaultTagName
				}

				tag := f.Tag.Get(tagName)
				ft := ParseFieldTag(tag)

				if ft.Skipped {
					continue
				}

				// 如果需要合并字段，且这个字段类型是一个 Struct 或 Ptr to Struct，那么会使用 from 的值来给 fv 赋值。
				if ft.Squash {
					fieldType := f.Type

					for fieldType.Kind() == reflect.Ptr {
						fieldType = fieldType.Elem()
					}

					if fieldType.Kind() == reflect.Struct {
						if err := dec.decode(from, fv.Addr()); err != nil {
							return err
						}

						continue
					}
				}

				k := f.Name

				if ft.Alias != "" {
					k = ft.Alias
				}

				kv := from.MapIndex(reflect.ValueOf(k))

				if !kv.IsValid() {
					continue
				}

				if err := dec.decode(kv, fv.Addr()); err != nil {
					return err
				}
			}

			return nil
		}

	case reflect.Interface:
		fromType := from.Type()
		toType := to.Type()

		if !fromType.Implements(toType) {
			return fmt.Errorf("go-data: cannot decode an interface value of type %v from %v", toType, fromType)
		}

		to.Set(reflect.ValueOf(clone.Clone(from.Interface())))
		return nil
	}

	return fmt.Errorf("go-data: cannot decode a value of type %v from %v", to.Type(), from.Type())
}
