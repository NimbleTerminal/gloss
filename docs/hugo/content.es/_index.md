---
title: "gloss"
type: docs
bookToC: false
---

# gloss

> Borrador de traducción, sin revisar por una persona; si hay dudas, vale la versión en inglés.

Un paginador visual para la terminal: como `less`, para imágenes, SVG, PDF, mallas STL, 3MF, OBJ y PLY, Markdown, diagramas Mermaid, HTML, texto plano, JSON, cuadernos Jupyter, Word, Excel, Grist y archivos CSV.

[**Pruébelo en su navegador →**](https://nimbleterminal.github.io/gloss/) La terminal en vivo ejecuta el paginador real en Go como WebAssembly, con una muestra de cada tipo.

## Inicio rápido

```sh
brew install --cask nimbleterminal/tap/gloss

gloss photo.png report.pdf model.stl      # page through them
gloss --menu *.png                        # a menu to choose among files
gloss --page 3 --output page3.png report.pdf   # a PNG, with no terminal
gloss --info --json model.3mf             # what a file says about itself
```

## Dónde mirar

- **[Guía](guide/)**: instalación, las teclas, cómo pedirle un archivo al usuario, los formatos que abre gloss y sus límites, y cómo usarlo desde un agente.
- **[Referencia de comandos](/gloss/docs/command/)** (English): todas las opciones, agrupadas según su función. gloss no tiene subcomandos; es un solo comando con grupos de opciones.
- **[Para LLM](/gloss/docs/guide/for-llms/)** (English): la skill que gloss incluye para enseñar a un agente su superficie sin interfaz (`gloss skill show`).

La página de manual, `gloss(1)`, se incluye en los archivos de cada versión y en el paquete Debian.

## Código fuente

gloss tiene licencia MIT y está hecho en Go sobre [NTCharts](https://github.com/NimbleMarkets/ntcharts). El código fuente, las incidencias y las versiones están en [GitHub](https://github.com/NimbleTerminal/gloss).
