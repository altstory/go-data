package data

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/huandu/go-assert"
)

func TestDataQuery(t *testing.T) {
	cases := []struct {
		Data   Data
		Fields []string
		Result interface{}
	}{
		{ // 空值
			Data{},
			nil,
			nil,
		},
		{ // 简单情况
			Make(RawData{
				"a": 1,
			}),
			[]string{"a"},
			int64(1),
		},
		{ // 不存在 field
			Make(RawData{
				"a": 1,
			}),
			[]string{"bad"},
			nil,
		},
		{ // 测试 fake query
			fullData,
			[]string{"fake", "query"},
			nil,
		},
		{ // 测试 fake query
			fullData,
			[]string{"fake.query"},
			float64(67.89),
		},
		{ // 复杂查询
			fullData,
			[]string{"anonymous_type", "data_list", "2", "b"},
			true,
		},
		{ // 特殊 map
			Make(RawData{
				"m": map[int]interface{}{
					123: map[uint]interface{}{
						456: map[float64]interface{}{
							0.5: "foo",
						},
					},
				},
			}),
			[]string{"m", "123", "456", "0.5"},
			"foo",
		},
	}

	a := assert.New(t)

loop:
	for i, c := range cases {
		a.Use(&i, &c)
		query := strings.Join(c.Fields, ".")

		a.Equal(c.Result, c.Data.Get(c.Fields...))

		for _, f := range c.Fields {
			if strings.Contains(f, ".") {
				continue loop
			}
		}

		a.Equal(c.Result, c.Data.Query(query))
	}
}

var (
	complexData = Make(RawData{
		"int":    123,
		"true":   true,
		"false":  false,
		"float":  12.34,
		"string": "string",
		"map": RawData{
			"m": "m",
		},
		"array": []RawData{
			{
				"d1": 1,
			},
			{
				"d2": "2",
			},
		},
		"ints":    []int{3, 2, 1},
		"floats":  []float64{5.5, 4.5, 3.5},
		"strings": []string{"s1", "s2", "s3"},
		"any":     []interface{}{1, "2", 3.3},
	})
	complexDataJSON = `{
	"any": [
		1,
		"2",
		3.3
	],
	"array": [
		{
			"d1": 1
		},
		{
			"d2": "2"
		}
	],
	"false": false,
	"float": 12.34,
	"floats": [
		5.5,
		4.5,
		3.5
	],
	"int": 123,
	"ints": [
		3,
		2,
		1
	],
	"map": {
		"m": "m"
	},
	"string": "string",
	"strings": [
		"s1",
		"s2",
		"s3"
	],
	"true": true
}`
)

func TestDataString(t *testing.T) {
	cases := []struct {
		Data       Data
		PrettyJSON string
	}{
		{ // 空值
			Data{},
			"{}",
		},
		{ // 典型情况
			complexData,
			complexDataJSON,
		},
	}

	a := assert.New(t)
	meta := "<json>"

	for _, c := range cases {
		a.Equal(c.Data.JSON(true), c.PrettyJSON)
		a.Equal(c.Data.PrettyString(), meta+"\n"+c.PrettyJSON)

		buf := &bytes.Buffer{}
		a.NilError(json.Compact(buf, []byte(c.PrettyJSON)))
		str := meta + buf.String()
		a.Equal(c.Data.String(), str)
	}
}

func TestDataParse(t *testing.T) {
	a := assert.New(t)

	buf := &bytes.Buffer{}
	a.NilError(json.Compact(buf, []byte(complexDataJSON)))
	complexDataCompactJSON := buf.String()

	cases := []struct {
		Str      string
		Data     Data
		HasError bool
	}{
		{ // 简单情况
			`<json>{}`,
			Data{},
			false,
		},
		{ // 典型情况
			"<json>" + complexDataCompactJSON,
			complexData,
			false,
		},
		{ // 错误格式
			`{"a":1}`,
			Data{},
			true,
		},
		{ // 错误 JSON
			`<json>{"a":1,}`,
			Data{},
			true,
		},
		{ // 错误 JSON 内容
			`<json>1`,
			Data{},
			true,
		},
		{ // 不认识的类型
			`<bson>{"a":1}`,
			Data{},
			true,
		},
	}

	for i, c := range cases {
		a.Use(&i, &c)
		d, err := Parse(c.Str)

		if c.HasError {
			a.NonNilError(err)
		} else {
			a.NilError(err)
		}

		a.Equal(d, c.Data)
	}
}

func TestDataJSONUnmarshal(t *testing.T) {
	cases := []struct {
		JSON     string
		Value    interface{}
		HasError bool
	}{
		{ // 测试是否能够直接生成符合 Data 规范的数据，主要是保证各种类型符合预期。
			`{"a":123, "data":{"int":123.0, "float":2.5, "strings":["s1", "s2"]}}`,
			&struct {
				A    int  `json:"a"`
				Data Data `json:"data"`
			}{
				A: 123,
				Data: Make(RawData{
					"int":     int64(123),
					"float":   float64(2.5),
					"strings": []string{"s1", "s2"},
				}),
			},
			false,
		},
		{ // 错误情况
			`{"data":["s1", "s2"]}`,
			&struct {
				Data Data `json:"data"`
			}{},
			true,
		},
	}

	a := assert.New(t)

	for _, c := range cases {
		vt := reflect.ValueOf(c.Value).Type()
		actual := reflect.New(vt).Elem()
		err := json.Unmarshal([]byte(c.JSON), actual.Addr().Interface())

		if c.HasError {
			a.NonNilError(err)
		} else {
			a.NilError(err)
		}

		a.Equal(c.Value, actual.Interface())
	}
}
