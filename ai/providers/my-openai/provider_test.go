package myopenai

import (
	"encoding/json"
	"testing"
)

/*
1. json.RawMessage 本质就是一个 []byte 字节数组
2. “ 和 "" 都是字符串的标志
3. 但是 `""`，相当于把 "" 也作为字符串的一部分
  - 注意：
  - “ 单引号用于括弧/表示json字符串
  - 直接 return string 那么 "" 也是字符串的一部分（这是正常的）
  - 我们想要的是：`"name"` ，“ 括住的json字符串"name"，解析成go string后输出应该是 name
  - 这里 json 字符串(`"name"`)，那么就输出是 name
  - 这里 go 字符串(`name`)，那么输出是 name

4. 解析后的结果：可以解析一个结构体，也可以解析一个string
*/
func TestDecodeArg(t *testing.T) {
	raw := json.RawMessage(`{"name":"xiaomi"}`) // 非法json字符串，json对象
	raw2 := json.RawMessage(`jsonstring`)       // 非法json，
	raw3 := json.RawMessage("jsonstring")       // 非法json，go 字符串
	raw4 := json.RawMessage(`"jsonstring"`)     // json 字符串 `` 表示json，"" 表示字符串

	r := decodeArg(raw)
	t.Log("原始raw:", raw)
	t.Log("对象:", r)

	r2 := decodeArg(raw2)
	t.Log("原始raw2: ", raw2)
	t.Log("字符串：", r2)

	r3 := decodeArg(raw3)
	t.Log("原始raw3: ", raw3)
	t.Log("字符串：", r3)

	r4 := decodeArg(raw4)
	t.Log("原始raw4: ", raw4)
	t.Log("字符串：", r4)

	t.Log("数组：", []int{1, 2, 3})
}
