# mvn-search

本地 Maven 仓库依赖搜索 TUI，基于 [Bubble Tea v2](https://github.com/charmbracelet/bubbletea)。

![](https://img.shields.io/badge/go-1.26%2B-00ADD8) ![](https://img.shields.io/badge/terminal-TUI-4E5D4C)

在 `~/.m2/repository` 中检索 GAV 坐标，多关键字交集过滤，方向键选中后一键复制 Maven / Gradle 依赖片段。

## 快速开始

```bash
go build -o mvn-search .
./mvn-search
```

## 用法

在输入框中敲关键字即可过滤（默认聚焦，空格分隔多个关键字，取**交集**）：

| 按键 | 行为 |
|---|---|
| 可打印字符 / Backspace | 编辑查询（`space` 输入空格） |
| `↑` `↓`（`Ctrl+P` `Ctrl+N`） / `PgUp` `PgDn` `Home` `End` | 进入**操作视图**并移动选中 |
| `Space`（操作视图） | 展开 / 收起当前包的版本列表 |
| `Esc`（操作视图） | 退出操作视图（`space` 恢复为空格输入） |
| `Esc`（输入视图） | 清空查询 |
| `Enter` | 复制 **Maven** 依赖 XML |
| `Ctrl+G` | 复制 **Gradle (Groovy)** 依赖 |
| `Ctrl+R` | 重建索引缓存 |
| `Ctrl+C` | 退出 |

标题栏的 `INPUT` / `OPS` 标明当前模式；状态栏提示 `space types` / `space expand`。

### 命令行参数

| 参数 | 说明 |
|---|---|
| `-repo <path>` | 覆盖仓库根（默认 `~/.m2/repository`，或环境变量 `MVN_SEARCH_REPO`） |
| `-rebuild` | 强制重建索引缓存 |

### 复制示例

Maven：

```xml
<dependency>
    <groupId>com.google.guava</groupId>
    <artifactId>guava</artifactId>
    <version>33.5.0-jre</version>
</dependency>
```

Gradle：

```gradle
implementation 'com.google.guava:guava:33.5.0-jre'
```

剪贴板优先走 `wl-copy` / `xclip` / `xsel` / `pbcopy`（若存在），否则回退 **OSC 52**（SSH / tmux 下可用）。

## 索引缓存

- 路径：`$XDG_CACHE_HOME/mvn-search/index.json`，未设置 XDG 时为 `~/.cache/mvn-search/index.json`
- 首次启动扫描并写入缓存；之后命中缓存直接加载
- 仓库有新增依赖后用 `Ctrl+R` 或 `-rebuild` 刷新

## 行为说明

- **多关键字交集**：每个关键字须在 `groupId` / `artifactId` / `version` 任一处命中（大小写不敏感子串），全部关键字同时满足才入选。
- **路径前缀树**：`groupId` 按 `.` 分段建树，子包自动挂在父路径下（如 `com.illtamer.plugin.*` 挂在 `com.illtamer.plugin` 下），再挂 `artifactId` / `version`。单链路径折叠为 `a.b.c`，单包折叠为 `group:artifact`。
  ```
  org.junit.jupiter  ·4  −
  │  ├─ junit-jupiter  ·4  −
  ▸ │  │  ├─ 5.12.2
    │  │  └─ 5.10.0
    │  └─ junit-jupiter-api  ·7  +
    org.mockito:mockito-junit-jupiter  ·2  +
  ```
  默认：组展开显示包名、包收起隐藏版本；`Space` 在操作视图逐层收纳/展开（`·N` 数量，`+`/`−` 状态）。单包组会折叠为 `group:artifact`。空查询保持扁平 GAV。
- **操作视图**：方向键进入；`Space` 收纳/展开；`Esc` 回到输入模式。
- **排序**：artifactId 精确 > 前缀 > 包含 > groupId > version；同 GA 按版本号降序（数字段感知，`33.10` > `33.9`）。
- **识别规则**：版本目录下存在 `{artifactId}-{version}.pom` 或主 jar（忽略 `-sources` / `-javadoc` 等 classifier 单独成条）；jar 与 pom 同 GAV 时合并为一条并优先标记 `jar`。

## 开发

```bash
GOTOOLCHAIN=local go vet ./...
GOTOOLCHAIN=local go test ./...
```

依赖：

```
charm.land/bubbletea/v2
charm.land/bubbles/v2
charm.land/lipgloss/v2
```
