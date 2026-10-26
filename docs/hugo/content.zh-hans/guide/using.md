---
title: "使用 gloss"
weight: 15
bookToC: false
---

# 使用 gloss

> 翻译草稿，尚未经人工校对；如有疑问，以英文版为准。

gloss 显示你指定的文件、某个文件夹的内容，或通过管道传入的内容。不带文件时它以空白状态打开：把文件拖到它上面，或按 `o` 浏览。用 `http://` 或 `https://` 地址代替文件时，会先下载，再像显示对应文件一样显示它；参见[打开网址](#打开网址)。

```sh
gloss photo.png drawing.svg report.pdf model.stl
gloss --menu photo.png report.pdf model.stl       # a menu to choose among them
gloss --preview photo.png report.pdf model.stl    # the menu, with a preview pane
```

在支持的终端上会自动选用 Kitty 图形协议，否则退回到彩色半块字符。程序使用备用屏幕，
退出时恢复终端。使用 `-X`（`--no-alt-screen`，与 `less` 相同）则改在主屏幕上绘制：
不会动滚动历史，退出时最后一屏停留在绘制的位置。选择可以等到退出时再做：
`q` 按启动时的方式退出，而 `Q` 无论如何都会把最后一屏留在滚动历史中，
所以值得保留的图片会留在下一条命令旁边，不需要的则不会留下。图形的直接传输
可通过 SSH 工作；不需要共享文件系统或外部转换器。在 tmux 中，请用
`set -g allow-passthrough on` 启用透传。

```sh
gloss --page 12 report.pdf
gloss --dpi 300 report.pdf        # higher PDF raster resolution
gloss --render glyph photo.png   # universal terminal rendering
gloss --render kitty photo.png   # override capability detection
gloss --3d software model.stl    # bypass GPU initialization
gloss --3d wireframe model.stl
cat drawing.svg | gloss
cat model.stl | gloss --type stl -
gloss -- -filename.png
gloss -X photo.png               # keep the scrollback; the picture stays after q
gloss --serve report.pdf         # show the viewer on a web page instead
gloss --serve --pick             # ask the user for a file; print its path
gloss --pick --prompt "The invoice, please"   # and say what for
gloss --glob images --glob meshes ~/models    # open every match under a folder
gloss                            # no file yet; drop files or press o to browse
gloss ~/Pictures                 # browse a folder for a file to open
gloss https://example.org/a.pdf  # download a web address and show it
```

## 打印一屏视图

`--print` 把初始内容视口绘制到标准输出后退出。它在被捕获的 shell 输出中也能工作，
包括编程智能体里的 `!` 命令，不需要 TTY：

```sh
gloss --print photo.png
gloss --print --print-width 100 --print-height 30 report.pdf
gloss --print --page 2 --cols name,total sales.xlsx
cat notes.md | gloss --print -
```

图片使用带 ANSI 颜色的 Unicode 半块字符。文本、Markdown 和表格使用常规的终端渲染。
没有查看器界面、输入循环、光标移动、终端探测或 Kitty 协议。GIF 和动画 WebP
停留在第一帧。网格使用软件渲染的快照。会剥离 ANSI 样式的宿主将丢失图片颜色；
若要让智能体检查像素，请改用 PNG 导出及其读取图片的工具。

视口默认为标准输出终端的大小；输出被捕获时为 **80 列 × 24 行**。
`--print-width 1..512` 和 `--print-height 1..256` 可以覆盖它；终端尺寸以这些值为上限。
每个指定的文件打印一屏，之间以一个空行分隔。`--page` 选择一页或一张工作表
（或一个排版导出切片），`--cols`/`--coln` 过滤表格。
支持 `--typeset`、行号和截断长行。自动主题为浅色，不探测也不读取已保存的偏好；
需要深色命令输出时请显式使用 `--theme dark`。

`--print` 不能与其他输出模式、交互式查看器选项或 `--render kitty` 组合使用。
它只把内容写到标准输出，把错误写到标准错误；成功时退出码为 0，出错时为 1；
出错会在已打印的视图之后中止整批处理。`-X` 仍然是需要终端的交互式主屏幕查看器。

## 视频的帧

视频的第一帧常常是黑的。`--at` 指定想要的时刻，格式如 `1:23`、`1:02:03`、`83.5`
或 `1m23s`，最多 16 个，以逗号分隔；`--frames N` 在视频中均匀取 N 帧，
即 N 个等长部分各自的中点。多帧会合成一张图，每帧的时间标在它的角上；
也可以用 `--output-dir` 每帧一个文件，`--print` 同样会显示它们。由同一个 ffmpeg
绘制全部帧，且都不会被放大。使用 `--json` 时，每个条目列出各帧的时间（秒）
以及当时显示的字幕。在查看器中，给出一个 `--at` 会让视频从那里开始。

```sh
gloss --frames 9 --output sheet.png clip.mp4
gloss --at 1:23 --output frame.png clip.mp4
gloss --at 0:05,0:30,1:00 --output-dir frames clip.mp4
gloss --text clip.mp4             # its subtitles, each after its times
gloss --at 1:23 clip.mp4          # play from 1:23
```

## 把链接显示为二维码

`--qr TEXT` 把 TEXT（通常是一个网址）显示为二维码，代替文件，供手机或平板扫描。
凡是能显示图片的地方都能用：查看器、`--serve`（一个网页，没有终端的智能体可以用它给你看）、
`--output` 和 `--print`。查看器和网页会在二维码上方显示该文本，除非 `--prompt`
另有说明。文本只是被编码：不会下载任何内容，`GLOSS_NO_FETCH` 也不适用。

```sh
gloss --serve --qr http://192.168.1.20:8000/
gloss --qr https://example.com/ --output qr.png
```

`localhost` 地址的二维码在其他设备上没有用；请给出该设备能够访问的地址。

## 打开网址

任何 `http` 或 `https` URL 参数都会在 gloss 启动前下载，并取代文件的位置，
所以凡是文件能用的地方它都能用：查看器、菜单、`--serve`，以及无界面模式
`--info`、`--text`、`--grep`、`--output` 和 `--print`。写出地址本身就是请求，
所以不需要任何选项。下载限时一分钟，文件上限为 128 MiB，最多跟随十次重定向，
且绝不从 https 跳转到 http，不发送 cookie 或已存储的凭据，并且必须是 gloss
能显示的格式；`--type` 和 `--accept` 对它的作用与对文件相同。其他方案（如 `ftp://`）
是错误，下载失败也是错误，与无法打开的文件完全一样，只是 `--info` 会把它连同
其他结果一并报告，列在它的地址下，然后继续。文件以地址末尾命名，并在 gloss
退出时删除，除非它已被选取。分离式的 `--serve` 或 `--pick` 在服务器启动前下载，
下载到会话的私有文件夹中，被选取的下载随后保留在那里供调用方使用。只有文件参数
会被下载；传给选项的地址（如 `--qr` 或 `--prompt`）原样传递。

下载时以 `gloss/` 加版本号表明身份，与 `gloss --version` 的输出一致。宿主若想用
别的名字，可通过 `--user-agent` 指定；未给出该选项时，则用环境中的
`GLOSS_USER_AGENT`：一行，不超过 1 KiB，不含控制字符。用 `--fetch` 在文档内部
打开的地址发送的是同一个名字。

URL 参数就是一次网络请求。传入了并非自己选择的名字的脚本，可以在环境中把
`GLOSS_NO_FETCH` 设为任意非空值来禁止这一点：此后地址参数会失败，`--fetch` 会被拒绝。
不要把密码放进地址里；它们会作为 Basic 认证发送，并留在你的 shell 历史中。
名字以 `http://` 开头的本地文件无法在命令行中指定；本来也没有这样的文件。

选项使用 `pflag` 的 GNU 语法，可以出现在文件名之前或之后。`--page=3` 和 `-p3`
都可以；布尔型短选项可以合并（`-mP`）。用 `--` 结束选项解析。
`--type image|svg|pdf|stl|3mf|obj|ply|geojson|gpx|pmtiles|docx|xlsx|grist|csv|json|ipynb|html|text|markdown|mermaid|video`
（`jsonl` 和 `ndjson` 表示 `json`，`md` 表示 `markdown`）对所有输入覆盖自动检测。
内容检测支持没有扩展名的文件。`-` 把标准输入一次性读入临时文件，退出时删除；
键盘输入来自控制终端。交互式输出必须是终端；图片导出可以在脚本中使用。
`gloss --help` 列出所有选项。

不支持的文件、目录和其他非常规条目会在标准错误中报告并跳过，因此 glob 可以包含
无关的条目。目录不会被遍历。如果一个都不剩，gloss 以错误退出，除非指定的是
文件夹：那样它会在那里打开文件浏览器。受支持的文件仍然受大小限制约束；
`--type` 明确强制指定输入格式。

常用短选项：`-h` 帮助，`-V` 版本，`-m` 菜单，`-P` 预览，`-X` 主屏幕，
`-N` 行号，`-S` 截断长行，`-p` 页，`-d` DPI，`-r` 渲染模式，`-t` 类型，
`-o` 输出 PNG，`-O` 输出目录，`-s` 图片最大边长。

```sh
gloss report.pdf -p3 -o page.png --vision-profile openai-high
gloss photo.png drawing.svg model.stl -mP
```

## 选择视图

- 把 Markdown 当作终端文本阅读，或按 `t` 得到排版页面，带本地图片、动画 GIF 和
  WebP，以及高亮的代码。文本、源代码和 JSON 也支持 `t`。用 `f` 选择字体，
  用 `T` 选择主题；参见 [Markdown](/gloss/docs/guide/markdown/)（英文）。
- 用 `/` 搜索文档文本、表格单元格或 PDF 文本。在源代码和文本中，`#` 显示行号，
  `:` 跳到某一行，`S` 切换长行是否换行。
- 用 Space 暂停单独的 GIF 或动画 WebP，用 `<`/`>` 调整速度，拖动或逐帧步进。
  用 Space 播放视频（MP4、MOV、WebM、MKV、OGV、AVI 等），用 ←/→ 或进度条跳转；
  所需条件参见[格式](/gloss/docs/guide/formats/)（英文）。可以环绕查看 STL、3MF、
  OBJ 和 PLY 网格，用 `c` 选择 3MF 部件。可以平移和缩放 GeoJSON 或 GPX 地图，
  或 PMTiles 存档，它们都是离线绘制的。
- 按 `e` 导出当前图片或排版视口。脚本可以在没有终端的情况下使用 `--output`；
  网格可以把多个视图导出到一张图中。
- 用 `u` 把表格中的 URL 显示为二维码，或用 `--pick-web` 让手机上传文件和回复。
  局域网设置和智能体会话参见[文件交接](/gloss/docs/guide/handoff/)（英文）。

[控制参考](/gloss/docs/guide/controls/)（英文）涵盖各种媒体类型的按键。

## 用其他应用打开

gloss 提供哪些应用、每种文件的默认应用是哪一个，都来自你的操作系统：macOS 上是
Launch Services，Linux 和 BSD 上是 XDG 桌面条目和 MIME 默认值，Windows 上是注册表。
gloss 自己不添加任何目录。

在原生查看器中，`w` 用默认应用打开当前文件，`W` 打开 **Open with** 选择器，
`E` 编辑文件。在文件菜单或缩略图网格中，这些键作用于高亮的文件。在文件浏览器中，
Alt+Enter 打开 **Open with** 选择器而不选中该条目；目录只提供 Reveal。如果你的终端
不报告 Alt+Enter，请用 Enter 打开受支持的文件再按 `W`，或配置终端的按键绑定。

选择器列出 System default、系统已知的可打开此类文件的应用、Reveal in Finder/Explorer
（Linux 上是所在文件夹）以及 Copy path。用方向键选择，Enter 打开。在某个应用上按 `d`，
可将它保存为这种文件和这个动作的默认应用；这不会启动它。在 Linux 上，为一般类型
注册的应用也会提供给其子类型，所以纯文本编辑器会出现在源代码、Markdown 和 JSON 中。
Esc 关闭选择器；已保存的选择会保留。

`E` 用解析出的编辑应用进行编辑。对于文本、Markdown、JSON、HTML、SVG、CSV、Mermaid、
OBJ 和笔记本文件，它是 `$VISUAL`，VISUAL 为空时退回到 `$EDITOR`，除非你保存了别的
默认值，前提是它指向一个已安装的程序；否则，以及对于其他文件，它是系统的默认编辑器。
没有任何可用者时，会改为打开 **Edit with** 选择器。二进制文件绝不会交给终端编辑器，
而且当没有应用能编辑某个文件时，gloss 会这样告诉你。选择器中的 Tab 在 **Open with**
和 **Edit with** 之间切换；默认值按文件种类和动作分别保存，所以打开 Markdown 文件的
应用可以不同于编辑它的应用。在 **Open with** 中，选择 System default 并按 `d`
可移除覆盖设置。例如，运行 `EDITOR=nano gloss README.md`，或在 shell 中设置
`VISUAL='nvim -f'`。

终端应用（终端编辑器，或在 Linux 上标有 `Terminal=true` 的桌面条目）会接管终端：
gloss 挂起它的界面，等待该应用结束，然后重新加载文件，尽可能保持源代码视图和滚动位置。
菜单和浏览器回到原来的选择。请使用在你完成之前一直留在前台的编辑器。

在选择器中按 `m` 可管理所有格式下已安装的应用：方向键选择，Space 隐藏或恢复应用并
立即保存，Enter 或 Esc 回到选择器。被隐藏的应用仍然保留在这个清单中。System default
和 Reveal 不能隐藏，终端应用不会列出。隐藏已保存的默认应用，会改用系统默认，
直到该应用被恢复。

偏好设置保存在操作系统用户配置目录下的 `gloss/open-with-v1.json` 中。其中只有应用
ID，没有可执行的命令。不可用的已保存应用会退回到系统默认。如果该文件不存在，
在新文件尚不存在期间，gloss 会读取旧的 `gloss/open-with.json`，包括其中的短 ID
（`vscode`、`preview` 等），并保持其不变；你的选择会保存到新文件中。如果
`open-with-v1.json` 已损坏，gloss 会报告，并在你修复或删除它之前不再保存。
在 Linux 上，请避免让 gloss 成为它自己的默认打开方式。

`gloss --apps` 按格式和动作列出可用的应用及实际生效的默认值；`--apps --json` 为脚本
提供同样的列表，而不打开任何东西。不支持自定义 GUI 命令。浏览器演示、通过 `--serve`
提供的查看器以及网页选择器都无法启动宿主应用。

打开来自标准输入或下载的文件并不会延长它的寿命。如果想保留它，请在 gloss 退出前
从另一个应用中保存一份副本。打开只是提交一个启动请求；gloss 无法得知另一个应用是否
显示了该文件。

文件浏览器有自己的帮助：在输入过滤条件之前按 `?`，或在输入时按 `F1`。它列出导航、
补全、布局、类型过滤、排序、查找和用应用打开的控制。方向键和 PageUp/PageDown 滚动帮助；
Esc 或 F1 回到原来的文件夹、过滤条件和选择。已有过滤条件中的 `?` 仍然是文件名搜索的一部分。
