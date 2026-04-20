# DFSwitch

跨平台桌面客户端，一键将 [Sub2API](https://github.com/sub2api/sub2api) 的 API Key 写入本地 AI 工具的配置文件。

## 支持的工具

| 工具 | 配置方式 | 支持的平台 |
|------|---------|-----------|
| Claude Code | `~/.claude/settings.json` | anthropic, minimax, custom |
| Gemini CLI | `~/.gemini/.env` | gemini |
| ChatBox | App 配置文件 | openai |
| Cherry Studio | App 配置文件 | openai, anthropic, minimax, custom |
| OpenCode | `~/.config/opencode/opencode.json` | openai, custom |
| OpenClaw | `~/.openclaw/openclaw.json` | openai, custom |
| Cursor | VS Code 配置文件 | openai |

## 功能

- **一键导入** — 选择 Key、勾选目标工具，一步写入所有配置
- **自动同步** — 后台定时刷新配置，Key 失效自动感知
- **系统托盘** — 常驻后台，关闭窗口不退出
- **原子写入** — 备份原配置，不丢失用户已有设置
- **平台感知** — 根据工具协议自动处理 URL 拼接（`/v1`、`/v1beta` 等）
- **自动更新** — 检测 GitHub Release 并原地升级
- **2FA 支持** — 登录时支持 TOTP 二步验证

## 下载

前往 [Releases](https://github.com/HellsKnight0129/dfswitch/releases) 下载对应平台的安装包：

- **macOS Apple Silicon** — `dfswitch-darwin-arm64.zip`
- **macOS Intel** — `dfswitch-darwin-amd64.zip`
- **Windows** — `dfswitch-windows-amd64.zip`

## 本地构建

### 环境要求

- Go 1.24+
- Node.js 20+
- [Wails CLI v2.12.0](https://wails.io)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
```

### 开发模式

```bash
cd web && npm install && cd ..
wails dev
```

### 生产构建

```bash
# macOS
wails build -platform darwin/arm64 -trimpath -clean

# Windows
wails build -platform windows/amd64 -trimpath -clean
```

或使用快速脚本（纯 Go 交叉编译，不需要 Wails CLI）：

```bash
./build.sh
```

## 使用方式

1. 启动 DFSwitch
2. 输入 Sub2API 服务器地址，登录账号
3. 选择要使用的 API Key
4. 勾选目标工具，点击"一键导入"
5. 开启自动同步（可选）

## 发布

推送 tag 自动触发 GitHub Actions 构建和发布：

```bash
git tag v0.2.0
git push origin v0.2.0
```

## License

MIT
