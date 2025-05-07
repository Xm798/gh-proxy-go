# gh-proxy-go

这是 [gh-proxy](https://github.com/hunshcn/gh-proxy) 的 Go 语言版本。

## 快速开始

```bash
docker run -d -p 8080:8080 xm798/gh-proxy-go:latest
```

## 配置说明

配置文件 `config.json` 支持以下选项：

- `host`: 默认 "0.0.0.0"，服务器监听地址
- `port`: 默认 8080，服务器监听端口
- `whiteList`: 允许访问的 GitHub 用户/组织列表
- `blackList`: 禁止访问的 GitHub 用户/组织列表
- `forceEnUSForRaw`: 默认 `false`，是否强制使用 en-US 语言访问 raw.githubusercontent.com，以规避中文用户可能的 429 错误
- `sizeLimit`: 默认 10240，文件大小限制（单位：MB）

注意：`host` 和 `port` 配置不支持热重载，需要重启服务才能生效。其他配置项支持热重载。

如需在 Docker 中自定义配置，请使用 `-v /path/to/config.json:/app/config.json` 挂载配置文件。
