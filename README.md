# go-infra

Go 基础设施工具集，用于构建生产级应用。

## 包

| 包 | 说明 |
|---|---|
| [lifecycle](./lifecycle) | 应用生命周期管理：顺序启动、LIFO 关闭、信号处理、优雅停机 |

## 使用

```go
import "github.com/qoderwork/go-infra/lifecycle"

func main() {
    mgr := lifecycle.New(lifecycle.WithTimeout(10 * time.Second))
    mgr.Add(myDB)
    mgr.Add(myHTTPServer)
    mgr.Run() // 阻塞直到收到信号，然后优雅关闭
}
```

完整示例见 [examples](./examples)。

## 设计原则

- **KISS** — 最小 API 表面，没有不必要的抽象
- **注册顺序即优先级** — 先加的先启动、后关闭，和 `defer` 一样直觉
- **零隐藏 goroutine** — 所有操作在调用者线程同步执行
- **幂等关闭** — `sync.Once` 保证 Stop 安全调用多次

## License

MIT
