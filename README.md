# YouTube 视频下载器 (Go WebUI)

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go" alt="Go version">
  <img src="https://img.shields.io/badge/platform-Windows-blue?style=flat" alt="Platform">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat" alt="License">
  <img src="https://img.shields.io/badge/version-2.0.0-orange?style=flat" alt="Version">
</p>

基于 Go 语言构建的 YouTube 视频下载器，提供 **Web 界面** 操作。底层调用 [yt-dlp](https://github.com/yt-dlp/yt-dlp) 完成视频解析、DASH 流下载与 ffmpeg 合并封装。

---

## 功能特性

| 模块 | 说明 |
|------|------|
| **视频解析** | 输入 YouTube 链接，自动提取标题、封面、时长、分辨率列表、多语言音轨 |
| **本地下载** | 选择分辨率 + 音轨组合，调用 yt-dlp 下载并自动合并为 MKV/MP4/WebM |
| **实时进度** | WebSocket 推送下载进度（速度、大小、ETA、分片、合并阶段），前端实时渲染 |
| **并发任务** | 最多 3 个任务并行下载，每个任务独立临时目录互不干扰 |
| **断点续传** | 下载中断后自动重试（最多 2 次），重试时保留临时文件以支持 `--continue` 续传 |
| **合并超时检测** | 合并阶段超时/僵死检测，自动终止并重试 |
| **自动清理** | 下载完成自动清理 per-task 临时目录，仅保留最终合成的视频文件 |
| **灵活配置** | 支持 HTTP/SOCKS5 代理、浏览器 Cookies 直读（Firefox/Chrome/Edge 等）、并发分片数、输出格式 |

---

## 项目原理

```
┌──────────────┐     HTTP/WebSocket      ┌──────────────────────┐
│   浏览器前端   │ ◄──────────────────────► │   Go 后端 (:8899)     │
│  (HTML/JS)    │                         │   net/http           │
└──────────────┘                         │   gorilla/websocket  │
                                          └──────────┬───────────┘
                                                     │
                                        ┌────────────┼────────────┐
                                        │            │            │
                                    进程调用     exec.LookPath   embed.FS
                                        │            │            │
                                ┌───────┴───────┐    │    ┌───────┴───────┐
                                │   yt-dlp.exe   │    │    │  web/static/   │
                                │  ffmpeg.exe    │    │    │  (SPA 前端)     │
                                │  node.exe      │    │    └───────────────┘
                                └───────────────┘    │
                                    工具链自动发现     │
                                （同目录 > PATH）      │
```

### 核心流程

1. **视频信息提取** — 前端输入 URL  → `GET /api/info` → Go 调用 `yt-dlp --dump-json` → 解析 JSON → 返回 `VideoInfo`（标题、分辨率、编码、音轨、文件大小）
2. **下载队列调度** — `POST /api/download` → 生成 `taskID` → 进入并发槽位（最多 3 个并行）→ 拼接 `yt-dlp` 参数 → 创建 per-task 临时目录（`.ytdl_<ts>/`）→ `--paths home:<tempdir>` 隔离文件
3. **进度推送** — yt-dlp stdout 按行解析 → 正则提取百分比/速度/ETA/分片 → `WebSocketHub.Broadcast()` → 前端实时渲染
4. **合并与清理** — yt-dlp 下载完成后自动调用 ffmpeg 合并 DASH 流 → Go 端检测合并阶段（超时/僵死检测）→ 将最终合成视频从临时目录移至保存目录 → `os.RemoveAll` 清理临时目录
5. **重试机制** — 合并超时自动重试（最多 2 次），重试期间保留临时目录以利用 `--continue` 和 `--keep-video` 续传

### 格式选择策略

```
用户选择格式 ID (如: 299+140)
    │
    ▼
有 ffmpeg → 格式回退链: {选中格式}/bestvideo+bestaudio/best
无 ffmpeg → 回退到: best[ext=mp4]/best
    │
    ▼
yt-dlp 下载 → ffmpeg 合并 → 输出 MKV/MP4/WebM
```

---

## 目录结构

```
FetchTubeWeb/
├── main.go                      # 入口：HTTP 服务器、路由注册、中间件
├── go.mod / go.sum              # Go 模块依赖
├── Makefile                     # 构建脚本（开发/生产/清理）
├── build_release.ps1            # PowerShell 一键打包脚本
├── .gitignore
├── .github/
│   └── workflows/
│       └── release.yml          # GitHub Actions 自动构建发布
├── internal/
│   ├── config/
│   │   └── config.go            # JSON 配置持久化（~/.FetchTubeWeb_config.json）
│   ├── handler/
│   │   ├── handler.go           # Server 结构体 + 路由注册
│   │   ├── download.go          # 下载/任务/目录浏览/缩略图 API
│   │   ├── taskmanager.go       # 并发任务队列（最多 3 并行）
│   │   ├── ws.go                # WebSocket Hub（进度广播）
│   │   ├── config.go            # 配置读写 API
│   │   └── browse.go            # 文件系统浏览 API
│   ├── models/
│   │   └── models.go            # 数据模型（VideoInfo, FormatOption, ProgressMsg 等）
│   └── ytdlp/
│       ├── ytdlp.go             # yt-dlp/ffmpeg/node 查找 + 进程调用 + 错误翻译
│       ├── info.go              # yt-dlp JSON 解析 → VideoInfo 模型
│       └── manager.go           # 下载管理器（进度解析、合并超时检测、per-task 临时目录、重试）
├── web/
│   ├── embed.go                 # go:embed 内嵌前端静态资源
│   └── static/
│       ├── index.html           # SPA 主页面
│       ├── css/
│       │   └── style.css        # UI 样式（自适应亮色/暗色主题）
│       └── js/
│           ├── api.js           # REST API 封装
│           ├── ws.js            # WebSocket 客户端（自动重连）
│           └── app.js           # 主逻辑（搜索/下载/队列渲染/配置持久化）
├── docs/
│   ├── tutorial.md              # 开发教程文档
│   ├── pic/                     # 架构图、流程图（SVG）
│   │   ├── architecture.svg
│   │   ├── config-cycle.svg
│   │   ├── directory-structure.svg
│   │   ├── request-flow.svg
│   │   ├── state-machine.svg
│   │   └── ws-communication.svg
│   └── py/
│       └── generate_diagrams.py # 图表生成脚本
└── release_exe/
    ├── yt-dlp.exe               # 开发环境外部工具（不纳入版本管理）
    ├── ffmpeg.exe
    └── node.exe
```

---

## 技术栈

| 层级 | 技术 |
|------|------|
| **后端语言** | Go 1.22+ |
| **HTTP 路由** | `net/http` Go 1.22 增强路由 (`GET /api/xxx`, `POST /api/xxx`) |
| **WebSocket** | `github.com/gorilla/websocket` v1.5.3 |
| **视频引擎** | [yt-dlp](https://github.com/yt-dlp/yt-dlp)（外部可执行文件） |
| **合并工具** | [ffmpeg](https://ffmpeg.org/)（外部可执行文件） |
| **JS 运行时** | [Node.js](https://nodejs.org/)（yt-dlp 解决 YouTube n challenge） |
| **前端** | 原生 HTML/CSS/JS（零框架 SPA） |
| **资源内嵌** | `embed` 标准库 (`//go:embed static/*`) |
| **构建** | Makefile（开发）+ PowerShell 脚本（打包）+ GitHub Actions（发布） |

### 外部工具链

| 工具 | 用途 | 必需 | 下载 |
|------|------|:----:|------|
| **yt-dlp.exe** | 视频元数据提取 + 下载 | **是** | [GitHub Releases](https://github.com/yt-dlp/yt-dlp/releases) |
| **ffmpeg.exe** | DASH 流合并 + 转封装 | 推荐（高质量下载必需） | [gyan.dev](https://www.gyan.dev/ffmpeg/builds/) |
| **node.exe** | YouTube n challenge 验证（`--js-runtimes`） | 推荐 | [nodejs.org](https://nodejs.org/) |

> 程序启动时会自动检测同目录下的这些工具（同目录优先于 PATH），并在启动日志中打印路径。yt-dlp 调用时注入 `PYTHONUTF8=1` 环境变量解决 Windows GBK 编码下中文乱码。

---

## 开发环境搭建

### 前置条件

- **Go 1.22+**：[下载安装](https://go.dev/dl/)
- **yt-dlp.exe**：放置于项目 `release_exe/` 目录或加入 PATH
- （推荐）**ffmpeg.exe**、**node.exe**：放置于 `release_exe/` 目录

### 步骤

```bash
# 1. 克隆仓库
git clone <repo-url>
cd FetchTubeWeb

# 2. 安装 Go 依赖
make deps
# 或手动： go mod tidy && go mod download

# 3. 运行开发服务器 (默认端口 8899)
make run
# 或手动： go run . 8899

# 4. 开发构建（快速）
make build
# 输出： FetchTubeWeb.exe
```

启动后终端打印 banner 和工具链状态，浏览器自动打开 `http://localhost:8899`。

### 自定义端口

```bash
go run . 8080        # 使用 8080 端口
go build -o app.exe . && ./app.exe -port 8080
```

---

## 生产环境构建

### 方式一：Makefile（快速）

```bash
# 生产构建（压缩二进制 + 版本注入）
make release
# 输出： dist/FetchTubeWeb.exe

# 查看构建说明
make dist
```

### 方式二：PowerShell 一键打包脚本（推荐）

`build_release.ps1` 自动完成 **编译 → 收集依赖 → 打包到 `dist/` 目录**。

```powershell
# 在项目根目录执行
.\build_release.ps1

# 如遇到 powershell 权限问题，执行：
powershell -ExecutionPolicy Bypass -File .\build_release.ps1
```

**脚本执行步骤：**

| 步骤 | 操作 | 说明 |
|:----:|------|------|
| 1/4 | `go build -ldflags="-s -w" -o FetchTubeWeb.exe` | 编译压缩版 Go 二进制 |
| 2/4 | 创建 `dist/` 目录 | 清空旧目录后重建 |
| 3/4 | 复制 `FetchTubeWeb.exe` 到 `dist/` | |
| 4/4 | 搜索并复制外部依赖 | 按 "同目录 → PATH" 顺序查找 `yt-dlp.exe`、`ffmpeg.exe`、`node.exe` |

**输出示例：**

```
========================================
  FetchTubeWeb Release Build
========================================

[1/4] Building Go binary...
  [OK] FetchTubeWeb.exe built
[2/4] Creating release directory: D:\...\FetchTubeWeb\dist
  [OK] Directory created
[3/4] Copying files...
  [OK] FetchTubeWeb.exe
[4/4] Finding dependencies...
  [OK] yt-dlp.exe <- D:\...\yt-dlp.exe (15 MB)
  [OK] ffmpeg.exe <- D:\...\ffmpeg.exe (85 MB)
  [OK] node.exe   <- D:\...\node.exe (70 MB)

========================================
  Release: D:\...\FetchTubeWeb\dist
========================================
  FetchTubeWeb.exe  (8.5 MB)
  yt-dlp.exe              (15.0 MB)
  ffmpeg.exe              (85.0 MB)
  node.exe                (70.0 MB)

Done. Zip the dist folder to distribute.
```

> 如果缺少某外部依赖，脚本会标红 `[MISSING]` 并给出下载链接。可手动将 exe 放入项目根目录后重新运行。

### 最终分发

将 `dist/` 目录打包为 zip：

```powershell
Compress-Archive -Path dist\* -DestinationPath FetchTubeWeb_v2.0.0_win64.zip
```

用户解压后直接运行 `FetchTubeWeb.exe`，浏览器自动打开 WebUI。

---

## 使用指南

### 基础流程

1. **粘贴链接** → 在「视频链接」输入框粘贴 YouTube URL
2. **点击搜索** → 等待解析完成，展示视频信息（标题、作者、时长、封面）
3. **选择分辨率** → 点击表格中的目标分辨率行（默认选最高）
4. **选择音轨** → 勾选需要的语言音轨（默认全选）
5. **选择输出格式** → MKV / MP4 / WebM
6. **设置保存目录** → 点击「浏览」选择下载保存位置（调用原生系统文件夹对话框）
7. **点击下载** → 任务加入队列，实时显示进度条、速度、大小、分片信息

### 代理配置

| 代理模式 | 地址示例 | 说明 |
|----------|----------|------|
| 无 | — | 直连 |
| HTTP | `http://127.0.0.1:1080` | HTTP 代理 |
| SOCKS5 | `socks5://127.0.0.1:1080` | SOCKS5 代理 |

### Cookies 配置（解决反爬虫验证）

| 选项 | 说明 |
|------|------|
| **Firefox**（推荐） | 运行时直接读取 Firefox Cookie 数据库，**无需关闭浏览器** |
| Chrome / Edge / Brave / Opera | 需先关闭浏览器（数据库被锁定），才能读取 |
| 文件... | 使用浏览器扩展导出的 `cookies.txt` |

---

## 配置说明

配置自动保存到 `%USERPROFILE%\.FetchTubeWeb_config.json`，应用启动时自动加载。

```json
{
  "local": {
    "last_url": "",
    "proxy_mode": "无",
    "proxy_host": "127.0.0.1",
    "proxy_port": "1080",
    "output_format": "mkv",
    "concurrent_fragments": 8,
    "cookies": "无",
    "cookies_path": "",
    "save_dir": "",
    "keep_temp_files": false
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `local.proxy_mode` | string | `"无"` | 代理模式：`无` / `HTTP` / `SOCKS5` |
| `local.proxy_host` | string | `"127.0.0.1"` | 代理服务器地址 |
| `local.proxy_port` | string | `"1080"` | 代理服务器端口 |
| `local.output_format` | string | `"mkv"` | 输出容器：`mp4` / `webm` / `mkv` |
| `local.concurrent_fragments` | int | `8` | 并发下载分片数（1-32） |
| `local.cookies` | string | `"无"` | Cookie 来源：`Firefox` / `Chrome` / `Edge` / `Brave` / `Opera` / `文件` |
| `local.cookies_path` | string | `""` | cookies.txt 文件路径（仅 `cookies=文件` 时生效） |
| `local.save_dir` | string | `""` | 上次使用的下载保存目录 |
| `local.keep_temp_files` | bool | `false` | 是否保留 per-task 临时目录中的中间文件（调试用） |

---

## API 接口

### 视频 & 下载

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/info?url=&proxy=&cookies=` | 获取视频元信息 |
| `POST` | `/api/download` | 加入下载队列 |
| `GET` | `/api/tasks` | 查询所有任务 |
| `POST` | `/api/tasks/{taskID}/cancel` | 取消任务 |
| `DELETE` | `/api/tasks/{taskID}` | 删除任务记录 |
| `POST` | `/api/tasks/batch-delete` | 批量删除已完成任务 |
| `GET` | `/api/thumbnail?url=` | 代理获取 YouTube 缩略图（绕过 i.ytimg.com CORS 限制） |

### 配置 & 工具

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/config` | 获取当前配置 |
| `PUT` | `/api/config` | 保存配置 |
| `GET` | `/api/health` | 健康检查（含工具链状态 + 可用浏览器列表） |
| `POST` | `/api/open-dir` | 在文件管理器中打开指定目录 |
| `POST` | `/api/pick-folder` | 弹出原生文件夹选择对话框 |
| `GET` | `/api/browse?path=` | 浏览目录结构 |
| `GET` | `/api/drives` | 获取 Windows 驱动器列表 |

### WebSocket

| 路径 | 说明 |
|------|------|
| `GET` `/ws/progress` | 实时进度推送 |

**进度消息 JSON 格式：**

```json
{
  "task_id": "ts_xxx",
  "status": "downloading",
  "percent": 45.2,
  "speed_mbps": 2.5,
  "downloaded_mb": 120.3,
  "total_mb": 266.0,
  "eta_seconds": 58,
  "elapsed_seconds": 42,
  "filename": "Video Title.mp4",
  "fragment_index": 3,
  "fragment_count": 8,
  "merge_elapsed": 12.5,
  "merge_remaining": 287,
  "retry_attempt": 1,
  "retry_max": 2,
  "title": "Video Title",
  "url": "https://...",
  "save_dir": "D:/downloads"
}
```

> `status` 可能的值：`downloading` / `merging` / `finished` / `error` / `cancelled` / `retry` / `queued` / `starting`。完成时额外包含 `avg_speed_mbps`、`final_size_mb`、`download_elapsed`、`merge_elapsed_final` 字段。

---

## GitHub Actions 自动发布

推送版本标签（`v1.0.0`）或手动触发 workflow，自动完成：

1. 下载最新 `yt-dlp.exe`、`ffmpeg.exe`、`node.exe`
2. 编译 `FetchTubeWeb.exe`（注入版本号 `-X main.version`）
3. 打包为 `FetchTubeWeb_v{ver}_win64.zip`
4. 创建 GitHub Release 并上传 zip

**触发方式：**

```bash
git tag v1.0.0
git push origin v1.0.0
```

或在 GitHub Actions 页面手动触发（可指定版本号）。

---

## Makefile 参考

| 目标 | 说明 |
|------|------|
| `make build` | 开发构建 → `FetchTubeWeb.exe` |
| `make release` | 生产构建 → `dist/FetchTubeWeb.exe`（压缩 + 版本注入） |
| `make run` | 启动开发服务器（端口 8899） |
| `make clean` | 清理构建产物 |
| `make deps` | 下载 Go 依赖 |
| `make fmt` | 格式化代码 (`go fmt ./...`) |
| `make vet` | 静态检查 (`go vet ./...`) |
| `make dist` | 生产构建 + 显示分发说明 |
| `make help` | 显示所有可用目标 |

---

<p align="center">
  <b>YouTube 视频下载器</b> — Go 后端 · 浏览器 WebUI · 多任务并行 · 临时目录隔离
</p>
