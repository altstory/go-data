package data

import (
	"reflect"

	"github.com/huandu/go-clone"
)

// Merge 将多个 data 从左至右合并到 d 里面，如果有同名的 key 会进行深度遍历进行合并。
// 返回的 d 是一个全新的 Data，修改 d 的内容不会影响参数中任何的 data。
//
// 具体的合并规则是：
//     - 对于 Data 类型的数值，将相同的 key 进行深度合并；
//       - 如果出现同名 key 且 value 类型相同，都是 Data 或者 slice，深度合并 value 值；
//       - 如果出现同名 key 且 value 类型不同，后面出现的 value 覆盖前面的 value。
//     - 对于 slice 类型的数值，如果两个 slice 类型相同，后面出现的 slice 的值会被 append 进去。
func Merge(data ...Data) (d Data) {
	if len(data) == 0 {
		return emptyData
	}

	target := RawData{}
	merge(reflect.ValueOf(target), data[0].data, data[1:]...)
	return Data{
		data: target,
	}
}

// MergeTo 将多个 data 从左至右合并到 target 里面，如果有同名的 key 会进行深度遍历进行合并。
// 如果 target 为 nil，则直接返回，不做任何操作。
//
// 具体的合并规则是参考 `Merge` 的文档。
func MergeTo(target *Data, data ...Data) {
	if target == nil || len(data) == 0 {
		return
	}

	merge(reflect.ValueOf(target.data), data[0].data, data[1:]...)
}

func merge(target reflect.Value, data RawData, remaining ...Data) {
	for k, v := range data {
		key := reflect.ValueOf(k)
		from := target.MapIndex(key)
		to := mergeValue(from, v)

		target.SetMapIndex(key, to)
	}

	if len(remaining) == 0 {
		return
	}

	merge(target, remaining[0].data, remaining[1:]...)
}

// mergeValue 假定 target 和 v 都是 Data 中的值，因此不会出现 ptr、struct、interface 等特殊类型，
// 而且所有的 map 类型都是 Data。
func mergeValue(target reflect.Value, v interface{}) reflect.Value {
	if v == nil {
		return target
	}

	data := reflect.ValueOf(v)

	if target.IsValid() {
		for target.Kind() == reflect.Interface {
			target = target.Elem()
		}

		if target.Type() == data.Type() {
			switch target.Kind() {
			case reflect.Map:
				if target.IsNil() {
					target = reflect.MakeMap(target.Type())
				}

				iter := data.MapRange()

				for iter.Next() {
					key := iter.Key()
					from := target.MapIndex(key)
					to := mergeValue(from, iter.Value().Interface())

					target.SetMapIndex(key, to)
				}

				return target

			case reflect.Slice:
				return reflect.AppendSlice(target, data)
			}
		}
	}

	return reflect.ValueOf(clone.Clone(v))
}
