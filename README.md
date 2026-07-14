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
- **零隐藏 goroutine** — Manager 不会在调用者 goroutine 之外偷偷起后台线程（唯一例外是 `WaitSignal`，由 Go runtime 的信号处理 goroutine 负责捕获 SIGINT/SIGTERM，与你的代码无关）。`Task.Start` / `Task.Stop` 由 Manager 在调用者 goroutine **同步**驱动，但 Task 自己可以用 `go` 起后台服务——`Start` 不必阻塞，返回即代表"已启动"。
- **幂等关闭** — `sync.Once` 保证 Stop 安全调用多次

## 停机触发源

Manager 提供两种"等停机信号"的入口，按需选择：

- `mgr.Run()` —— **推荐入口**：`Start` 之后自动监听系统信号（SIGINT / SIGTERM，Windows 上为 Ctrl+C），收到后优雅关闭。适合独立二进制。
- `mgr.WaitSignal(timeout)` —— 与 `Run` 内部逻辑相同，但你可以先 `Start` 再做别的事，再手动等待信号。
- `mgr.Wait(ctx)` —— 停机由你**自己的 `context`** 驱动（如测试、K8s 探针、父进程编排），不依赖 OS 信号。把 ctx 取消即可触发优雅关闭。

## License

MIT
