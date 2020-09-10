package data

import (
	"testing"

	"github.com/huandu/go-assert"
	"github.com/huandu/go-clone"
)

func TestMerge(t *testing.T) {
	cases := [][]Data{
		{ // 空值
			Data{},
		},
		{ // 简单的数组复制
			Make(RawData{"a": 1}),
			Make(RawData{"a": 1}),
		},
		{ // 典型的数据结构
			Make(RawData{
				"map": RawData(nil), // 故意放一个空数据
			}),
			Make(RawData{
				"str":   "abcdefg",
				"int":   1234,
				"float": -43.21,
				"slice": []string{"first", "second"},
				"map": RawData{
					"a": true,
					"b": "string",
					"d": []int8{1, 2, 3},
				},
			}),
			Make(RawData{
				"str":   "zyxwvu",
				"uint":  5678,
				"nil":   nil,
				"slice": []string{"third", "forth"},
				"map": RawData{
					"a": false,
					"c": 123,
					"d": []uint{4, 5},
				},
			}),
			Make(RawData{
				"str":   "zyxwvu",
				"int":   1234,
				"uint":  5678,
				"float": -43.21,
				"slice": []string{"first", "second", "third", "forth"},
				"map": RawData{
					"a": false,
					"b": "string",
					"c": 123,
					"d": []uint{4, 5}, // 类型不同，应该覆盖而不是追加
				},
			}),
		},
	}
	const notExistKey = "this-is-a-key-not-exist"
	a := assert.New(t)

	for i, c := range cases {
		a.Use(&i, &c)

		input := c[:len(c)-1]
		expected := c[len(c)-1]

		if len(input) > 1 {
			target := Data{
				data: clone.Clone(input[0].data).(RawData),
			}
			MergeTo(&target, input[1:]...)
			a.Equal(expected, target)
		}

		actual := Merge(input...)
		a.Equal(expected, actual)

		if actual.Len() == 0 {
			continue
		}

		// 确保 actual 是从 c 中拷贝出来的，修改 actual 不会修改任何 c 的数据
		actual.data[notExistKey] = true

		for _, d := range input {
			_, ok := d.data[notExistKey]
			a.Assert(!ok)
		}
	}
}

func BenchmarkMerge(b *testing.B) {
	input := []Data{
		Make(RawData{
			"str":   "abcdefg",
			"int":   1234,
			"float": -43.21,
			"slice": []string{"first", "second"},
			"map": RawData{
				"a": true,
				"b": "string",
			},
		}),
		Make(RawData{
			"str":   "zyxwvu",
			"uint":  5678,
			"slice": []string{"third", "forth"},
			"map": RawData{
				"a": false,
				"c": 123,
			},
		}),
	}

	for i := 0; i < b.N; i++ {
		Merge(input...)
	}
}
