# go-infra

Go 基础设施工具集，用于构建生产级应用。

## 包

| 包 | 说明 |
|---|---|
| [lifecycle](./lifecycle) | 应用生命周期管理：顺序启动、LIFO 关闭、信号处理、优雅停机 |
| [license](./license) | 离线软件授权：Ed25519 签名、机器绑定、Feature/Capacity 两层授权、密钥轮换 |

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

## 授权（license 包）

极简、零依赖的**离线**软件授权方案。用 Ed25519 数字签名而非加密来保证防篡改与身份认证：授权文件是明文 JSON，可被审计；防伪造靠的是签名，不是隐藏内容。

### 核心概念

- **签发 / 验签分离**：`license-tool` 持有**私钥**签发授权；你的程序只**内置公钥**验签，无法伪造。
- **数据模型**：`License` 含 `ID / Product / Subject / Features[] / Capacity / NotBefore / Expiry / Machine / Version`。`Features` 是功能开关列表，`Capacity` 是数量上限（`map[string]int64`），二者构成 Feature + Capacity 两层授权。
- **确定性签名**：签名字节 = `License` 结构体的 JSON 编码。Go 的 `json` 按字段声明顺序输出、并对 map 的 key 排序，因此签发端与验签端产生完全一致字节。
- **机器绑定**：`Machine` 声明允许的机器指纹列表；`strict` 要求精确匹配（取不到指纹则失败），`loose` 在取不到指纹时放行。指纹由 `license/machine` 计算（主板序列号 → `/etc/machine-id` → hostname 回退链，sha256 归一化并拒占位符）。
- **密钥轮换**：`Version` 字段选择验签用的公钥；`Verifier.WithKey(pub, v)` 注册多版本公钥，换密钥时旧授权仍有效。

### 快速开始

签发（在你这边，持有私钥）：

```bash
go run ./cmd/license-tool genkey -priv private.pem -pub public.pem
# 写一个 template.json（字段见 License 结构体），然后：
go run ./cmd/license-tool sign -key private.pem -in template.json -out license.lic
```

验签（在程序里，只内置公钥）：

```go
pub, _ := license.DecodePublicKeyPEM(pubPEM) // pubPEM 用 go:embed 内置
data, _ := os.ReadFile("license.lic")
lic, err := license.NewVerifier(pub, license.CurrentVersion).
    WithFingerprint(machine.Fingerprint).
    Verify(data)
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

## License

MIT
