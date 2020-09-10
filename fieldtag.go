package data

import "strings"

// FieldTag 是一个解析完成的字段 tag。
//
// 格式是：
//     data:"alias,opt1,opt2,..."
//
// 当前支持以下选项：
//     - omitempty：忽略空值
//     - squash：将一个字段的内容展开到当前 struct
//
// 当 alias 为“-”时，当前字段会被跳过。
type FieldTag struct {
	Alias     string // 字段别名。
	Skipped   bool   // 字段别名为“-”时，跳过这个字段。
	OmitEmpty bool   // 忽略空值。
	Squash    bool   // 是否展开。
}

// ParseFieldTag 解析 field tag 的 alias 和选项。
// 详细的格式见 FieldTag 文档。
func ParseFieldTag(tag string) *FieldTag {
	opts := strings.Split(tag, ",")
	alias := strings.TrimSpace(opts[0])
	skipped := false
	omitEmpty := false
	squash := false

	for _, opt := range opts[1:] {
		switch opt {
		case "omitempty":
			omitEmpty = true
		case "squash":
			squash = true
		}
	}

	if alias == "-" {
		alias = ""
		skipped = true
	}

	return &FieldTag{
		Alias:     alias,
		Skipped:   skipped,
		OmitEmpty: omitEmpty,
		Squash:    squash,
	}
}
