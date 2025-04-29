# gh-proxy-go

这是 [gh-proxy](https://github.com/hunshcn/gh-proxy) 的 Go 语言版本。

## 快速开始

```bash
docker run -d -p 8080:8080 xm798/gh-proxy-go:latest
```

## 配置说明

配置文件 `config.json` 支持以下选项：

- `whiteList`: 字符串数组，允许访问的 GitHub 用户/组织列表
- `blackList`: 字符串数组，禁止访问的 GitHub 用户/组织列表
- `forceEnUSForRaw`: 布尔值（默认 `false`），是否强制使用 en-US 语言访问 raw.githubusercontent.com，以规避 429 错误

如需在 Docker 中自定义配置，请使用 `-v /path/to/config.json:/app/config.json` 挂载配置文件。

## 部署详情

[github.moeyy.xyz](https://github.moeyy.xyz/) 正在使用 **gh-proxy-go**，托管在 [BuyVM](https://buyvm.net/) 每月 3.5 美元的 1 核 1G 内存、10Gbps 带宽服务器上。

### 服务器概况

- **日流量处理**：约 3TB
- **CPU 平均使用率**：20%
- **带宽平均占用**：400Mbps

![服务器数据](https://github.com/user-attachments/assets/6fe37f41-aa35-4efc-b0b8-8c3339529326)
![Cloudflare 数据](https://github.com/user-attachments/assets/ae310b1f-96e9-42e9-a77c-0d8c1b8d6344)

---

如有问题或改进建议，欢迎提交 issue 或 PR！
