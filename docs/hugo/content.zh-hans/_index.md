---
title: "gloss"
type: docs
bookToC: false
---

# gloss

> 翻译草稿，尚未经人工校对；如有疑问，以英文版为准。

终端里的可视化分页器：像 `less` 一样，可查看图片、SVG、PDF、STL、3MF、OBJ 和 PLY 网格、Markdown、Mermaid 图表、HTML、纯文本、JSON、Jupyter 笔记本、Word、Excel、Grist 以及 CSV 文件。

[**在浏览器中试用 →**](https://nimbleterminal.github.io/gloss/) 在线终端以 WebAssembly 运行真正的 Go 分页器，每种类型都有一个示例。

## 快速开始

```sh
brew install --cask nimbleterminal/tap/gloss

gloss photo.png report.pdf model.stl      # page through them
gloss --menu *.png                        # a menu to choose among files
gloss --page 3 --output page3.png report.pdf   # a PNG, with no terminal
gloss --info --json model.3mf             # what a file says about itself
```

## 从哪里看起

- **[指南](guide/)**：安装、按键、向用户索取文件、gloss 能打开的格式及其限制，以及如何在智能体中使用。
- **[命令参考](/gloss/docs/command/)**（英文）：所有选项，按用途分组。gloss 没有子命令；它是一个命令，带有成组的选项。
- **[面向 LLM](/gloss/docs/guide/for-llms/)**（英文）：gloss 自带的 skill，用来教智能体了解它的无界面功能（`gloss skill show`）。

手册页 `gloss(1)` 随发布压缩包和 Debian 软件包一起提供。

## 源代码

gloss 采用 MIT 许可证，基于 [NTCharts](https://github.com/NimbleMarkets/ntcharts) 用 Go 编写。源代码、问题和发布版本都在 [GitHub](https://github.com/NimbleTerminal/gloss) 上。
