# go-infra

Go 基础设施工具集，用于构建生产级应用。

## 包

**零第三方依赖**：`go.mod` 只有 module 与 `go 1.24` 两行，全部包仅依赖标准库。

| 包 | 说明 |
|---|---|
| **并发与调度** | |
| [structured](./structured) | 结构化并发：WaitGroup（panic 传播）、限流任务池、Error/Context/ResultPool、并发 Map/ForEach、有序 Stream、Collector、指数退避 Retry |
| [workerpool](./workerpool) | 进程级常驻工作池：N 个常驻 worker + 缓冲队列、显式 `ErrQueueFull`、ctx 背压与关闭、panic 恢复不杀 worker |
| [conc](./conc) | fire-and-forget 助手与并发数据结构：GoRun/GoSafe/Timer、泛型 Map、TTLMap/TTLCache、PanicStack |
| [queue](./queue) | 无锁队列：有界 CAS 环形队列 `LockFree`、无界 Michael-Scott 泛型队列 `MSQueue[T]`、非并发安全 `CirQueue` |
| [event](./event) | 类型安全的事件总线（观察者模式），泛型 `Bus[T]` |
| **缓冲与内存** | |
| [buffer/ring](./buffer/ring) | 环形缓冲区：实现 `io.Reader`/`io.Writer`，支持自动扩容、ReadFrom/WriteTo |
| [buffer/ringpool](./buffer/ringpool) | 环形缓冲对象池：带自适应校准，减少 GC 压力 |
| [buffer/elastic](./buffer/elastic) | 弹性环形缓冲区：懒加载池化缓冲，空时自动归还 |
| [buffer/linkedlist](./buffer/linkedlist) | 字节切片链表缓冲：Peek/Discard 零拷贝窥视、ReadFrom/WriteTo、配合 bytepool 复用 |
| [bytepool](./bytepool) | 字节切片池：32 级分级池，减少 GC 压力 |
| [bytesconv](./bytesconv) | 零拷贝转换：`[]byte` ↔ `string` 无内存分配转换 |
| **通用工具** | |
| [lifecycle](./lifecycle) | 应用生命周期管理：顺序启动、LIFO 关闭、信号处理、优雅停机 |
| [licensing](./licensing) | 离线软件授权：Ed25519 签名、机器绑定、Feature/Capacity 两层授权、密钥轮换 |
| [cache](./cache) | LRU 缓存：支持容量限制、驱逐回调、线程安全包装 |
| [reason](./reason) | 错误分类与原因码 |
| [hook](./hook) | 缓存/转换/摘要/pprof/定时等常用钩子 |
| [executil](./executil) | 跨平台命令执行（Unix sh / Windows） |
| [system](./system) | 系统信息、IP、目录与备份工具 |
| [sliceutil](./sliceutil) | 切片工具：Unique、Intersect、Map、Filter、Reduce（泛型实现）|
| [strutil](./strutil) | 字符串工具：SwapCase、Capitalize、Abbreviate、Reverse、Truncate 等 |
| [mathutil](./mathutil) | 数学工具：2 的幂运算（Ceil/Floor/Closest/IsPowerOfTwo） |
| [randutil](./randutil) | 随机字符串：支持 `math/rand` 和 `crypto/rand`，DNS-1035 兼容 |
| [envutil](./envutil) | 环境变量解析：String/Int/Float64/Bool，支持默认值和必填 |
| [assert](./assert) | 零依赖测试断言（替代 testify 常用子集） |
| **CLI / 示例** | |
| [cmd/license-tool](./cmd/license-tool) | 授权签发与校验命令行工具 |
| [examples](./examples) | HTTP 服务、Worker、License 三个完整示例 |

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

## 结构化并发（structured）

`structured` 与 `conc` **分工不同，不要混用**：`conc.GoRun` 是 fire-and-forget，子 goroutine 的 panic 只记录日志（含 `PanicStack` 调用栈）不会传播；`structured` 里每个 goroutine 都有归属者，`Wait()` 会把子 goroutine 的 panic 连同栈一起传播回调用方。

```go
import "github.com/qoderwork/go-infra/structured"

p := structured.NewPool().WithErrors().WithMaxGoroutines(8)
for _, item := range items {
    item := item
    p.Go(func() error { return handle(item) })
}
if err := p.Wait(); err != nil {
    return err // errors.Join 聚合所有任务的错误；子任务 panic 会在此 re-panic
}
```

按场景选最小工具：

| 需求 | 入口 |
|---|---|
| 起一批 goroutine 并等全部结束 | `structured.WaitGroup` / `Go` + `Wait` |
| 限制并发数 | `NewPool().WithMaxGoroutines(n)` |
| 任务会返回错误 / 首个错误取消其它任务 | `WithErrors()` / `WithContext(ctx).WithCancelOnError()` |
| 收集结果且保持提交顺序 | `NewResultPool[T]()` |
| 并发处理切片 | `ForEach` / `ForEachIdx` / `Map` / `MapErr` |
| 任务并发执行、回调按提交顺序串行执行 | `NewStream()` |
| 并发生产、同步消费结果 | `Collector[T]` 的 `Iter()` / `All()` |
| 指数退避重试 | `Retry(ctx, cfg, fn)` |

两个使用约束：

- 所有池都必须调用一次 `Wait()`；`With*` 配置方法必须在第一次 `Go()` 之前调用，否则 panic。
- `Collector.Submit` 在缓冲区满时会阻塞，且 ctx 不能解阻塞它。缓冲区要能容纳所有未消费的提交量，或者先在另一个 goroutine 启动 `All()` / `Iter()`。

## 常驻工作池（workerpool）

`structured.Pool` 是「一批任务」的作用域并发，必须 `Wait()`；`workerpool.Pool` 是**进程级常驻资源**，任务可以在任何时候从任何地方提交，直到 `Close()`。它是 `pkg/pool/goroutine`（ants 包装）的零依赖替代：不引入 ants / x/sync / zap，也不带包级 `init()` 副作用。

```go
p, err := workerpool.New(8, 1024, workerpool.WithContext(ctx))
if err != nil {
    return err
}
defer p.Close()

if err := p.Submit(func() { handle(job) }); errors.Is(err, workerpool.ErrQueueFull) {
    return err // 任务没有被提交，可重试 / 降级 / 上报，绝不会静默丢弃
}
// 或者带背压、可取消地等待空位：
// err = p.SubmitWait(reqCtx, func() { handle(job) })
```

| | `structured.Pool` | `workerpool.Pool` |
|---|---|---|
| 生命周期 | 一批任务，`Wait()` 结束 | 进程级常驻，`Close()` 结束 |
| 提交 | `Go()` 阻塞等待 goroutine 名额 | `Submit()` 立即返回 / `SubmitWait(ctx)` 背压 |
| 队列满 | 阻塞 | 返回 `ErrQueueFull`，任务不提交 |
| 错误 / panic | `WithErrors()` 聚合、panic 传播到 `Wait()` | `WithPanicHandler()` 记录，worker 不死 |
| 关闭 | `Wait()` | `Close()` 排空队列；`Shutdown(ctx)` 只限制等待时间 |

三个语义要点：

- `Close()` 幂等，会**等所有已入队任务跑完**；`Shutdown(ctx)` 超时只是不再等待，关闭仍在后台继续，用 `Done()` 观察真正结束，任务不会被中途放弃。
- `SubmitWait` 等空位时持有读锁，但关闭流程会先关 `quit` 唤醒它，再拿写锁 `close(tasks)`，所以不会死锁，也不会出现 send on closed channel。
- 任务签名是 `func()`，不带 ctx、不返回 error；需要取消或结果时由调用方在闭包里自己传 channel / context。

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

## 授权（license 包）

极简、零依赖的**离线**软件授权方案。用 Ed25519 数字签名而非加密来保证防篡改与身份认证：授权文件是明文 JSON，可被审计；防伪造靠的是签名，不是隐藏内容。

### 核心概念

- **签发 / 验签分离**：`license-tool` 持有**私钥**签发授权；你的程序只**内置公钥**验签，无法伪造。
- **数据模型**：`License` 含 `ID / Product / Subject / Features[] / Capacity / NotBefore / Expiry / Machine / Version`。`Features` 是功能开关列表，`Capacity` 是数量上限（`map[string]int64`），二者构成 Feature + Capacity 两层授权。
- **确定性签名**：签名字节 = `License` 结构体的 JSON 编码。Go 的 `json` 按字段声明顺序输出、并对 map 的 key 排序，因此签发端与验签端产生完全一致字节。
- **机器绑定**：`Machine` 声明允许的机器指纹列表；`strict`（默认）要求精确匹配，指纹源缺失或读取失败即拒绝；`loose` 在指纹源缺失或读取失败时放行（best-effort），但成功读取后仍需匹配。指纹由 `licensing/machine` 计算（Linux: dmidecode system-uuid → 主板序列号 → `/etc/machine-id`；macOS: system_profiler；Windows: PowerShell），sha256 归一化为 UUID v5 格式，**fail-closed 拒绝 hostname/MAC 等可变标识符与占位符**。
- **密钥轮换**：`Version` 字段选择验签用的公钥；`Verifier.WithKey(v, pub)` 注册多版本公钥，换密钥时旧授权仍有效。

### 快速开始

签发（在你这边，持有私钥）：

```bash
go run ./cmd/license-tool genkey -priv private.pem -pub public.pem
# 写一个 template.json（字段见 License 结构体），然后：
go run ./cmd/license-tool sign -key private.pem -in template.json -out license.lic
```

验签（在程序里，只内置公钥）：

```go
pub, _ := licensing.DecodePublicKeyPEM(pubPEM) // pubPEM 用 go:embed 内置
data, _ := os.ReadFile("license.lic")
v, _ := licensing.NewVerifier(pub, licensing.CurrentVersion)
lic, err := v.WithFingerprint(machine.Fingerprint).Verify(data)
if err != nil { /* 授权无效：err 说明原因 */ }
_ = lic.HasFeature("advanced")
n := lic.CapacityOf("nodes")
```

CLI 还有 `verify`（手动校验）和 `fingerprint`（打印当前机器指纹）两个子命令。完整演示见 [examples/license](./examples/license)。

### 安全预期（重要）

离线授权是**君子级防护，不是保险箱**：

- ✅ 有效签名 = 授权确由私钥签发、且未被篡改；机器绑定防止 `.lic` 文件被拷贝到别机。
- ⚠️ **不防二进制补丁**：攻击者可 patch 程序跳过 `Verify()`，或改内存里的 `Expiry`。如需更强保护，请额外加二进制完整性校验 / 混淆。
- ⚠️ **时间旅行**：离线环境下改系统时间可绕过过期。可用 `Verifier.WithMinClock()` 接到一个持久化的"已知最晚时间"来防御时钟回拨。
- 私钥泄露需走轮换流程（升 `Version` 并签发新密钥，旧版授权靠 `WithKey` 兼容）。

## 第三方代码

`buffer/*`、`bytepool`、`bytesconv`、`mathutil`、`queue/msqueue.go` 派生自 gnet（Apache-2.0）；`structured/` 派生自 sourcegraph/conc 与 go-pkgz/pool（MIT）。原始版权声明保留在各源文件头，许可证全文见 [THIRD_PARTY_LICENSES.md](./THIRD_PARTY_LICENSES.md) 和 [`LICENSES/`](./LICENSES)。

## License

go-infra 主体为 MIT；派生文件按其原始许可证保留归属，详见上节。
