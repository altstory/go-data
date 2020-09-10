package data

import (
	"fmt"
	"reflect"
	"sort"
)

// Patch 代表一系列的对 Data 的修改操作。
type Patch struct {
	actions []*PatchAction
}

// PatchAction 代表一个 patch 操作。
type PatchAction struct {
	Deletes []string        `data:"deletes"`
	Updates map[string]Data `data:"updates"`
}

// NewPatch 创建一个新 Patch 对象。
func NewPatch() *Patch {
	return &Patch{}
}

// Add 增加一个新的 patch 操作。
// 每个 patch 操作都由 deletes 和 updates 组成，实际 apply 的时候
// 会先删除所有 deletes 字段，然后再使用 updates 进行更新。
// updates 的 key 是 Data 的 query，apply 时候会按照字典顺序从小到大排序之后合并更新到 Data。
//
// 需要注意，`Patch#Apply`/`Patch#ApplyTo` 使用 `Merge`/`MergeTo` 来更新 Data，
// merge 系列函数会深度遍历 map/slice，这导致新值无法简单覆盖老值。
// 如果希望新值覆盖老值，而不是合并，那么得先用 deletes 删除老值再合并。
func (patch *Patch) Add(deletes []string, updates map[string]Data) {
	patch.actions = append(patch.actions, &PatchAction{
		Deletes: deletes,
		Updates: updates,
	})
}

// Actions 返回所有的 action。
func (patch *Patch) Actions() []*PatchAction {
	return patch.actions
}

// Apply 将 d 复制一份出来，并将所有变更应用在 d 的副本上，
// d 本身不会受到任何影响。
//
// Apply 在如下情况下报错：
//     * updates 的某个 query 无法找到对应元素；
//     * updates 的某个 query 查询出的结果并不是一个 RawData。
func (patch *Patch) Apply(d Data) (applied Data, err error) {
	d = d.Clone()

	if err = patch.ApplyTo(&d); err != nil {
		return
	}

	applied = d
	return
}

// ApplyTo 将变更直接应用于 target 上，将会修改 target 内部值。
//
// ApplyTo 的出错条件与 Apply 相同。
func (patch *Patch) ApplyTo(target *Data) error {
	if target == nil {
		return nil
	}

	for _, action := range patch.actions {
		if err := action.ApplyTo(target); err != nil {
			return err
		}
	}

	return nil
}

// ApplyTo 将一个 action 应用到 target。
func (action *PatchAction) ApplyTo(target *Data) error {
	data := target.data

	// 先删除。
	data.Delete(action.Deletes...)
	target.data = data // Delete 可能重置 data 内容。

	if len(action.Updates) == 0 {
		return nil
	}

	// 再更新。
	queries := make([]string, 0, len(action.Updates))

	for query := range action.Updates {
		queries = append(queries, query)
	}

	// 需要以字典序升序排列，这样处理起来能保证先处理上层数据，再下层。
	// 比如，同时有更新 "a" 和 "a.b" 时候，保证 "a" 先执行。
	sort.Strings(queries)

	for _, query := range queries {
		v := data.Query(query)

		if v == nil {
			return fmt.Errorf("go-data: fail to apply patch due to invalid query `%v` when updating", query)
		}

		d, ok := v.(RawData)

		if !ok {
			return fmt.Errorf("go-data: fail to apply patch due to query `%v` pointing to a value in unsupported type", query)
		}

		merge(reflect.ValueOf(d), action.Updates[query].data)
	}

	return nil
}
