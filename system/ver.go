package system

import (
	"fmt"
	"strconv"
	"strings"
)

// versionSegmentWidth 是 uint64 十进制的最大位数。每段等宽补零后，
// 字符串的字典序与数值序一致，可直接交给调用方的比较函数。
const versionSegmentWidth = 20

// CompareVersionFunc 比较 ip 或版本号。
//
// 按 "." 分段解析为无符号整数，段数不足的一侧补 0（因此 "1.2" 等价于
// "1.2.0"），"-" 之后的后缀（如 "1.2.3-beta"）忽略；非数字段按 0 处理。
// 两段都规范化为等长字符串后再交给 f，例如 strings 的 ==、<、>=。
// 旧实现按 %03s 补零，段数不同或出现 >=1000 的段会因长度不等直接返回
// true（误判为相等），已修正。
func CompareVersionFunc(a, b string, f func(a, b string) bool) bool {
	s1 := versionSegments(a)
	s2 := versionSegments(b)
	width := len(s1)
	if len(s2) > width {
		width = len(s2)
	}
	return f(normalizeVersion(s1, width), normalizeVersion(s2, width))
}

func versionSegments(str string) []uint64 {
	parts := strings.Split(str, ".")
	out := make([]uint64, 0, len(parts))
	for _, item := range parts {
		if idx := strings.IndexByte(item, '-'); idx != -1 {
			item = item[:idx]
		}
		v, err := strconv.ParseUint(strings.TrimSpace(item), 10, 64)
		if err != nil {
			v = 0 // 非数字段统一按 0 处理
		}
		out = append(out, v)
	}
	return out
}

func normalizeVersion(seg []uint64, width int) string {
	var b strings.Builder
	b.Grow(width * versionSegmentWidth)
	for i := 0; i < width; i++ {
		var v uint64
		if i < len(seg) {
			v = seg[i]
		}
		fmt.Fprintf(&b, "%0*d", versionSegmentWidth, v)
	}
	return b.String()
}
