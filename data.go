package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/tidwall/gjson"
)

const defaultTagName = "data"

const (
	dataMetaBegin = "<"
	dataTypeJSON  = "json"
	dataMetaEnd   = ">"
)

var emptyData Data

// Data 是一种通用的数据存储结构，可以用于表达各种以 JSON、TOML、YAML 等为介质的数据，
// 也可以用于序列化/反序列化 Go struct。
//
// Data 里面的数据不允许随意修改，只能通过 `MergeTo`、`Patch#ApplyTo` 等方法修改。
type Data struct {
	data RawData
}

var (
	_ json.Marshaler   = Data{}
	_ json.Unmarshaler = &Data{}
)

// RawData 是一种未经加工的 map[string]interface{}。
type RawData map[string]interface{}

// Make 将一个普通 m 转化成 Data。
// 虽然 m 和 Data 类型一样，但是要成为 Data 必须得过滤掉不合法的数据，
// 也需要将 struct 转化成 Data 格式。
//
// Make 适合用于将 JSON、YAML 等反序列化工具获得的 map[string]interface{} 数据转化成 Data，
// 假设需要将任意数据转化成 Data，应该使用 Encoder。
func Make(m map[string]interface{}) Data {
	enc := Encoder{}
	return enc.Encode(m)
}

// Parse 从 str 中解析 Data，这个 str 应该是符合 Data 序列化格式的字符串。
// 如果 str 格式不合法，返回错误。
//
// Data 序列化格式定义如下：
//     '<' type '>' raw
// 当前 type 仅支持 JSON，值为 `json`，对应的 raw 是 JSON 字符串。
// 例如：
//     <json>{"hello":"world!"}
func Parse(str string) (d Data, err error) {
	if !strings.HasPrefix(str, dataMetaBegin) {
		err = errors.New("go-data: invalid data string format")
		return
	}

	str = str[len(dataMetaBegin):]
	idx := strings.Index(str, dataMetaEnd)

	if idx < 0 {
		err = errors.New("go-data: invalid data string format")
		return
	}

	typeName := str[:idx]
	str = str[idx+len(dataMetaEnd):]

	switch typeName {
	case dataTypeJSON:
		d, err = ParseJSON(str)
	default:
		err = fmt.Errorf("go-data: invalid data type '%v'", typeName)
	}

	return
}

// ParseJSON 解析 JSON 字符串并且生成 Data，如果解析过程出现任何错误则返回错误。
// 由于 Data 是一个 map，所以 JSON 必须是一个 object，如果不是则返回错误。
func ParseJSON(str string) (d Data, err error) {
	if !gjson.Valid(str) {
		err = errors.New("go-data: invalid JSON string")
		return
	}

	res := gjson.Parse(str)

	if !res.IsObject() {
		err = errors.New("go-data: JSON must be an object")
		return
	}

	raw := RawData{}
	parseJSONToRawData(raw, res)

	if len(raw) != 0 {
		d = Data{
			data: raw,
		}
	}

	return
}

func parseJSONValue(res gjson.Result) (v interface{}, t reflect.Type) {
	switch res.Type {
	case gjson.True:
		v = true
		t = typeOfBool
		return
	case gjson.False:
		v = false
		t = typeOfBool
		return
	case gjson.Number:
		f := res.Float()

		if f >= math.MinInt64 && f <= math.MaxInt64 && math.Round(f) == f {
			v = int64(f)
			t = typeOfInt64
			return
		}

		v = f
		t = typeOfFloat64
		return
	case gjson.String:
		v = res.Str
		t = typeOfString
		return
	case gjson.JSON:
		if res.IsObject() {
			d := RawData{}
			parseJSONToRawData(d, res)
			v = d
			t = typeOfObject
			return
		}

		// 对于数组来说，需要根据数组元素的类型来决定 slice 的类型。
		// 假如 slice 所有元素类型一致，那么需要尽可能的生成这个类型的 slice。
		// 例如，如果里面都是整数，则 slice 类型是 []int64。
		v, t = parseJSONArray(res.Array())
		return
	}

	return
}

func parseJSONToRawData(d RawData, res gjson.Result) {
	res.ForEach(func(key, value gjson.Result) bool {
		v, _ := parseJSONValue(value)
		d[key.Str] = v
		return true
	})
}

func parseJSONArray(res []gjson.Result) (v interface{}, t reflect.Type) {
	vals := make([]reflect.Value, 0, len(res))
	var elemType reflect.Type

	for _, r := range res {
		val, vt := parseJSONValue(r)

		if elemType == nil {
			elemType = vt
		} else if elemType != vt && elemType != typeOfInterface {
			elemType = typeOfInterface
		}

		vals = append(vals, reflect.ValueOf(val))
	}

	if elemType == nil {
		elemType = typeOfInterface
	}

	t = reflect.SliceOf(elemType)
	slice := reflect.MakeSlice(t, 0, len(vals))
	slice = reflect.Append(slice, vals...)
	v = slice.Interface()
	return
}

// MarshalJSON 将 d 序列化成 JSON。
func (d Data) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	d.json(buf, false)
	return buf.Bytes(), nil
}

// UnmarshalJSON 解析 JSON 字符串并设置 d 的值。
// 这里不直接使用 `json.Unmarshal` 来反序列化的原因是，`Data` 内部要求统一所有的数据类型，
// 但 `json.Marshal` 无法满足这个要求。
func (d *Data) UnmarshalJSON(src []byte) error {
	str := *(*string)(unsafe.Pointer(&src))
	data, err := ParseJSON(str)

	if err != nil {
		return err
	}

	*d = data
	return nil
}

// Query 解析 query 找到对应的值并且返回，如果找不到则返回 nil。
//
// 其中，query 的格式是以“.”分隔的字段，例如 a.b.c 代表访问 d["a"]["b"]["c"]。
// 如果希望访问数组元素，可以直接写数组下标数字，比如 a.0.c 代表访问 d["a"][0]["c"]。
func (d Data) Query(query string) interface{} {
	return d.data.Query(query)
}

// Query 解析 query 找到对应的值并且返回，如果找不到则返回 nil。
//
// 其中，query 的格式是以“.”分隔的字段，例如 a.b.c 代表访问 d["a"]["b"]["c"]。
// 如果希望访问数组元素，可以直接写数组下标数字，比如 a.0.c 代表访问 d["a"][0]["c"]。
func (d RawData) Query(query string) interface{} {
	if query == "" {
		return d
	}

	fields := strings.Split(query, ".")
	return d.Get(fields...)
}

// Get 通过 fields 找到对应的值并且返回，如果找不到则返回 nil。
//
// 其中，field 是一个数组，例如 []string{"a", "b", "c"} 代表访问 d["a"]["b"]["c"]。
// 如果希望访问数组元素，可以直接写数组下标数字，比如 []string{"a", "0", "c"} 代表访问 d["a"][0]["c"]。
func (d Data) Get(fields ...string) interface{} {
	return d.data.Get(fields...)
}

// Get 通过 fields 找到对应的值并且返回，如果找不到则返回 nil。
//
// 其中，field 是一个数组，例如 []string{"a", "b", "c"} 代表访问 d["a"]["b"]["c"]。
// 如果希望访问数组元素，可以直接写数组下标数字，比如 []string{"a", "0", "c"} 代表访问 d["a"][0]["c"]。
func (d RawData) Get(fields ...string) interface{} {
	if len(fields) == 0 {
		return d
	}

	v := d.get(fields, nil)

	if !v.IsValid() {
		return nil
	}

	return v.Interface()
}

func (d RawData) get(fields []string, modifier func(v reflect.Value) reflect.Value) (found reflect.Value) {
	if len(fields) == 0 {
		found = reflect.ValueOf(d)

		if modifier != nil {
			found = modifier(found)
		}

		return
	}

	k := fields[0]
	fields = fields[1:]
	val := reflect.ValueOf(d).MapIndex(reflect.ValueOf(k))

	if !val.IsValid() || val.IsNil() {
		return
	}

	if len(fields) == 0 {
		if modifier != nil {
			val = modifier(val)

			if val.IsValid() {
				d[k] = val.Interface()
			} else {
				delete(d, k)
			}
		}

		found = val
		return
	}

	callModifier := func(i int, v reflect.Value) reflect.Value {
		if i == len(fields)-1 && modifier != nil {
			return modifier(v)
		}

		return v
	}

	for i, f := range fields {
		for val.Kind() == reflect.Interface {
			val = val.Elem()
		}

		switch val.Kind() {
		case reflect.Map:
			t := val.Type()
			kt := t.Key()

			switch kt.Kind() {
			case reflect.String:
				idx := reflect.ValueOf(f)
				kv := val.MapIndex(idx)

				if !kv.IsValid() {
					return
				}

				kv = callModifier(i, kv)
				val.SetMapIndex(idx, kv)
				val = kv

			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n, err := strconv.ParseInt(f, 10, 64)

				if err != nil {
					return
				}

				idx := reflect.New(kt).Elem()

				if idx.OverflowInt(n) {
					return
				}

				idx.SetInt(n)
				kv := val.MapIndex(idx)

				if !kv.IsValid() {
					return
				}

				kv = callModifier(i, kv)
				val.SetMapIndex(idx, kv)
				val = kv

			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				n, err := strconv.ParseUint(f, 10, 64)

				if err != nil {
					return
				}

				idx := reflect.New(kt).Elem()

				if idx.OverflowUint(n) {
					return
				}

				idx.SetUint(n)
				kv := val.MapIndex(idx)

				if !kv.IsValid() {
					return
				}

				kv = callModifier(i, kv)
				val.SetMapIndex(idx, kv)
				val = kv

			case reflect.Float32, reflect.Float64:
				fl, err := strconv.ParseFloat(f, 64)

				if err != nil {
					return
				}

				idx := reflect.New(kt).Elem()

				if idx.OverflowFloat(fl) {
					return
				}

				idx.SetFloat(fl)
				kv := val.MapIndex(idx)

				if !kv.IsValid() {
					return
				}

				kv = callModifier(i, kv)
				val.SetMapIndex(idx, kv)
				val = kv

			default:
				return
			}
		case reflect.Slice:
			n, err := strconv.ParseInt(f, 10, 64)

			if err != nil {
				return
			}

			if n < 0 || n > math.MaxInt32 {
				return
			}

			l := val.Len()
			idx := int(n)

			if idx >= l {
				return
			}

			kv := val.Index(idx)
			kv.Set(callModifier(i, kv))
			val = kv

		default:
			return
		}

		if !val.IsValid() {
			return
		}
	}

	found = val
	return
}

// Delete 将 query 中查询到的值删除。
// 如果 query 为空字符串，则清空 d。
func (d *RawData) Delete(queries ...string) {
	// 如果需要清空 d，优先做这个处理，这样会更快一些。
	for _, query := range queries {
		if query == "" {
			*d = nil
			return
		}
	}

	for _, query := range queries {
		d.delete(query)
	}
}

func (d RawData) delete(query string) {
	fields := strings.Split(query, ".")
	target := fields[len(fields)-1] // 最后一个 key 是目标 key。
	fields = fields[:len(fields)-1]

	d.get(fields, func(val reflect.Value) (v reflect.Value) {
		if target == "" {
			return
		}

		for val.Kind() == reflect.Interface {
			val = val.Elem()
		}

		v = val

		switch val.Kind() {
		case reflect.Map:
			if val.Type().AssignableTo(typeOfObject) {
				val.SetMapIndex(reflect.ValueOf(target), reflect.Value{})
			}

			v = val
		case reflect.Slice:
			idx, err := strconv.ParseInt(target, 10, 64)

			if err != nil {
				return
			}

			i := int(idx)
			l := val.Len()

			if i < 0 || i >= l {
				return
			}

			if i == l-1 {
				val.SetLen(l - 1)
				return
			}

			front := val.Slice(0, i)
			remaining := val.Slice(i+1, l)
			v = reflect.AppendSlice(front, remaining)
		}

		return
	})
}

// JSON 返回 d 对应的 JSON 字符串。
// 如果 pretty 为 true，会为打印优化输出格式。
func (d Data) JSON(pretty bool) string {
	buf := &bytes.Buffer{}
	d.json(buf, pretty)
	return buf.String()
}

func (d Data) json(buf *bytes.Buffer, pretty bool) {
	if d.Len() == 0 {
		buf.WriteString("{}")
		return
	}

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)

	if pretty {
		enc.SetIndent("", "\t")
	}

	enc.Encode(d.data)

	// 干掉最后多余的那个 \n
	data := buf.Bytes()

	if len(data) > 0 && data[len(data)-1] == '\n' {
		buf.Truncate(buf.Len() - 1)
	}
}

// PrettyString 输出用于打印输出的存储格式。
func (d Data) PrettyString() string {
	buf := &bytes.Buffer{}
	buf.WriteString(dataMetaBegin + dataTypeJSON + dataMetaEnd)
	buf.WriteRune('\n')
	d.json(buf, true)
	return buf.String()
}

// String 返回 d 的可存储格式，这个格式可以用 Parse 解析并还原成 Data 结构。
func (d Data) String() string {
	buf := &bytes.Buffer{}
	buf.WriteString(dataMetaBegin + dataTypeJSON + dataMetaEnd)
	d.json(buf, false)
	return buf.String()
}

// Len 返回 d 的数据个数。
func (d Data) Len() int {
	return len(d.data)
}

// Clone 复制一份 d 的内容。
func (d Data) Clone() Data {
	return Merge(d)
}
