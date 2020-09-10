# go-data：可方便稳定序列化的树状数据结构 #

`go-data` 旨在封装所有类似 JSON 结构数据的处理方法，可以用于处理数据或配置文件（JSON、TOML、YAML 等）。
其核心数据结构 `Data` 是 `data.RawData` 的包装，但不同于简单的 map，通过 `Make` 或 `Encoder` 获得的 `Data` 可以保证里面不包含任何无法序列化的结构，例如 `func`、`chan` 等，也不会包含任何指针、interface 等。通过这样的加工，可以保证 `Data` 使用任何序列化和反序列化工具，比如 `json.Marshal` 和 `json.Unmarshal`，得到稳定输出（除非遇到了工具的 bug，比如 JSON 无法表达超过 `2^53` 的整数）。

## 使用方法 ##

### 构造 `Data` ###

`Make` 或 `Encoder` 可以将任意结构转换成 `Data`。需要注意，由于 `Data` 本质上是一个 map，所以只有 `struct`、结构的指针、`map[string]T` 可以成功转换成 `Data`。

```go
type T struct {
    Foo     int    `sample:"foo"`
    Bar     string `sample:"bar"`
    Empty   uint   `sample:"empty,omitempty"` // 设置 omitempty 后，如果 Empty 未设置值则不会被放入 Data
    Skipped bool   `sample:"-"`               // 名字为 -，则代表这个字段会被忽略
    *Embedded      `sample:",squash"`         // 设置 squash，这个结构会被展开（inline）到上层结构中去
}

type Embedded struct {
    Player bool `sample:"player"`
}

func main() {
    t := &T{
        Foo: 123,
        Bar: "player",
        Embedded: &Embedded{
            Player: true,
        },
    }
    enc := data.Encoder{
        TagName: "sample", // 自定义 field tag
    }
    d := enc.Encode(t)
    fmt.Println(d)

    // Output:
    // {
    //     "foo": 123,
    //     "bar": "player",
    //     "player": true,
    // }
}
```

### 读取数据 ###

`Data` 提供一些方法来方便的读取里面的数据。

```go
func main() {
    d := data.Make(data.RawData{
        "foo": data.RawData{
            "bar": 123,
        },
    })

    fmt.Println(d.Get("foo", "bar")) // 输出：123
    fmt.Println(d.Query("foo.bar"))  // 输出：123
}
```

### 解析数据 ###

通过使用 `Decoder` 可以将 `Data` 解析到任意 Go 结构里面去。

当前支持以下类型的解析：

* 布尔：`bool`
* 所有的整型：`int`/`int8`/.../`int64`/`uint`/`uint8`/.../`uint64`
* 所有的浮点：`float32`/`float64`
* 所有的复数：`complex64`/`complex128`
* 字符串：`string`
* 各种 Go 内置类型：`map`/`slice`/`array`/`struct`
* 时间类型：`time.Time`/`time.Duration`

其中，`time.Duration` 的源数据需要是符合 `time.ParseDuration` 规则的字符串，比如 `"2m30s"`。

```go
type T struct {
    Foo int           `sample:"foo"`
    Bar string        `sample:"bar"`
    Dur time.Duration `sample:"dur"`
}

func main() {
    d := data.Make(data.RawData{
        "foo": 123,
        "bar": "player",
        "dur": "2m30s",
    })
    dec := data.Decoder{
        TagName: "sample", // 自定义 field tag
    }

    var t T
    enc.Decode(d, &t)
    fmt.Println(t.Foo, t.Bar, t.Dur)
    fmt.Println(t.Dur == 2*time.Minute+30*time.Second)

    var foo int
    enc.DecodeQuery(d, "foo", &foo)
    fmt.Println("foo:", foo)

    // 输出：
    // 123  player  2m30s
    // true
    // foo:    123
}
```

### 序列化和反序列化 ###

为了方便将 `Data` 进行持久化存储，特别提供了专用的格式来进行序列化和反序列化。

如果要将 `Data` 序列化，可以直接调用 `Data#String` 方法。如果要反序列化，则使用 `Parse` 函数。

```go
func main() {
    d := data.Make(data.RawData{
        "foo": 123,
        "bar": "player",
    })
    str := d.String()
    fmt.Println(str)

    parsed, err := data.Parse(str)            // 输出：<json>{"bar":"player","foo":123}
    fmt.Println(err)                          // 输出：nil
    fmt.Println(reflect.DeepEqual(parsed), d) // 输出：true
}
```

### 通过 `Patch` 进行增量更新 ###

由于 `Data` 底层数据结构相对复杂，手动更新数据会出现很多问题，比如难以跟踪变化，在持久化存储时会出现难以追查的并发冲突问题。
为了解决这个，推荐所有对 `Data` 的变更都采用 `Patch` 来实现。

```go
func main() {
    patch := data.NewPatch()

    // 删除 d["v2"]、d["v3"][1]、d["v4"]["v4-4"]。
    patch.Add([]string{"v2", "v3.1", "v4.v4-1"}, nil)

    // 添加数据。
    patch.Add(nil, map[string]Data{
        // 在根添加数据。
        "": data.Make(data.RawData{
            "v1": []int{2, 3},
            "v2": 456,
        }),

        // 在 d["v4"] 里添加数据。
        "v4": data.Make(data.RawData{
            "v4-1": "new",
        }),
    })

    // 同时删除并添加数据。
    patch.Add([]string{"v4.v4-2"}, map[string]Data{
        "v4": data.Make(data.RawData{
            "v4-2": data.RawData{
                "new": true,
            },
        }),
    })

    d := data.Make(data.RawData{
        "v1": []int{1},
        "v2": 123,
        "v3": []string{"first", "second", "third"},
        "v4": data.RawData{
            "v4-1": "old",
            "v4-2": data.RawData{
                "old": true,
            },
        },
    })
    patch.ApplyTo(&d)

    fmt.Println(d)

    // Output:
    // <json>{"v1":[1,2,3],"v2":456,"v3":["first","third"],"v4":{"v4-1":"new","v4-2":{"new":true}}}
}
```

## 工作原理 ##

将数据编码成 `Data` 或者将 `Data` 数据提取到任意 Go 结构，这个的工作原理与 `json.Marshal` 和 `json.Unmarshal` 类似，可以查阅相关文章了解实现原理，这里不赘述。

为了保证 `Data` 序列化/反序列化结果能够稳定，这个库做了几件重要的事情：

* 将所有结构、指针、map、interface 等变成标准的 `Data` 类型；
* 将所有具有宽度的类型，比如 `int`/`int8`/`int16`/`int32` 等，都转化成最大尺寸的类型，比如 `int64`，这样保证同样的数据通过 `Make` 或 `Encoder` 得到 `Data` 时能够稳定；
* 将所有 slice、array 也标准化成 slice，并且如果 slice 的元素类型是 `int`/`int8`/`int16`/`int32` 等，也都转化成最大尺寸的类型，保证 slice 类型也能稳定；
* 消除所有类型别名，比如如果有一个类型是 `type MyInt int`，则会变成普通的 `int64`。
