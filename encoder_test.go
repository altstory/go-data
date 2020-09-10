package data

import (
	"testing"

	"github.com/huandu/go-assert"
)

func TestEncoder(t *testing.T) {
	cases := []struct {
		Value     interface{}
		Data      Data
		OmitEmpty bool
	}{
		{ // 空数据
			nil,
			Data{},
			false,
		},
		{ // 简单数据
			&AllValue{
				Int: 123,
			},
			Make(RawData{
				"int": int64(123),
			}),
			true,
		},
		{ // 测试所有类型
			allValues,
			fullData,
			false,
		},
	}
	a := assert.New(t)
	enc := &Encoder{
		TagName: "test",
	}

	for i, c := range cases {
		a.Use(&i, &c)

		enc.OmitEmpty = c.OmitEmpty
		a.Equal(c.Data, enc.Encode(c.Value))
	}
}
