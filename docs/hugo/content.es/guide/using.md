---
title: "Usar gloss"
weight: 15
bookToC: false
---

# Usar gloss

> Borrador de traducción, sin revisar por una persona; si hay dudas, vale la versión en inglés.

gloss muestra los archivos que le indicas, o el contenido de una carpeta, o lo que le envíes por una tubería. Sin ningún archivo se abre vacío: suelta archivos sobre él, o pulsa `o` para explorar. Una dirección `http://` o `https://` en lugar de un archivo se descarga primero y luego se muestra como el archivo que es; consulta [Abrir una dirección web](#abrir-una-dirección-web).

```sh
gloss photo.png drawing.svg report.pdf model.stl
gloss --menu photo.png report.pdf model.stl       # a menu to choose among them
gloss --preview photo.png report.pdf model.stl    # the menu, with a preview pane
```

Los gráficos Kitty se eligen automáticamente en las terminales compatibles, con
caracteres de medio bloque en color como alternativa. El programa usa la pantalla
alternativa y restaura la terminal al salir. Con `-X` (`--no-alt-screen`, como en
`less`) dibuja en la pantalla principal: el historial de desplazamiento no se
toca, y la última vista queda donde se dibujó cuando sales. La decisión puede
esperar hasta que salgas: `q` sale como se lanzó, y `Q` sale dejando la última
vista en el historial en cualquier caso, de modo que una imagen que vale la pena
conservar se queda junto al siguiente comando, y una que no, no se queda. El
transporte directo de gráficos funciona por SSH; no hace falta un sistema de
archivos compartido ni un conversor externo. En tmux, activa el paso directo con
`set -g allow-passthrough on`.

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

## Imprimir una vista

`--print` dibuja en la salida estándar el área de contenido inicial y termina.
Funciona en la salida capturada del intérprete de comandos, incluidos los
comandos con `!` en los agentes de programación, sin TTY:

```sh
gloss --print photo.png
gloss --print --print-width 100 --print-height 30 report.pdf
gloss --print --page 2 --cols name,total sales.xlsx
cat notes.md | gloss --print -
```

Las imágenes usan medios bloques Unicode con color ANSI. El texto, el Markdown y
las tablas usan el dibujado normal de la terminal. No hay interfaz del visor,
bucle de entrada, movimiento del cursor, consultas a la terminal ni protocolo
Kitty. Los GIF y los WebP animados se quedan en el primer fotograma. Las mallas
usan una captura por software. Un anfitrión que elimine los estilos ANSI perderá
los colores de las imágenes; para que un agente inspeccione píxeles, usa la
exportación a PNG y su herramienta de lectura de imágenes.

El área por omisión es el tamaño de la terminal de la salida estándar, o **80
columnas × 24 filas** cuando la salida se captura. `--print-width 1..512` y
`--print-height 1..256` la modifican; las dimensiones de la terminal se limitan a
esos valores. Cada archivo indicado imprime una vista, separadas por una línea en
blanco. `--page` elige una página u hoja (o un fragmento de exportación
tipográfica), y `--cols`/`--coln` filtran las tablas.
`--typeset`, los números de línea y las líneas recortadas son compatibles. El
tema automático es claro, sin sondeo ni lectura de preferencias guardadas; usa
`--theme dark` de forma explícita para una salida oscura en la línea de comandos.

`--print` no puede combinarse con otro modo de salida, con opciones del visor
interactivo ni con `--render kitty`. Escribe solo contenido en la salida estándar
y los errores en la salida de error, con código de salida 0 si todo va bien y 1 si
hay un error; un error detiene el lote después de las vistas que ya se hayan
impreso. `-X` sigue siendo el visor interactivo en la pantalla principal, que
requiere una terminal.

## Fotogramas de un vídeo

El primer fotograma de un vídeo suele ser negro. `--at` indica los momentos que
se quieren, como `1:23`, `1:02:03`, `83.5` o `1m23s`, hasta 16, separados por
comas; `--frames N` reparte N de ellos a lo largo del vídeo, en el centro de cada
una de N partes iguales. Varios forman una hoja con la hora de cada fotograma en
su esquina, o un archivo por fotograma con `--output-dir`, y `--print` también
los muestra. Un solo ffmpeg los dibuja todos, y ninguno se amplía. Con `--json`,
cada entrada lista las horas de sus fotogramas en segundos y el subtítulo que se
muestra en cada una. En el visor, un único `--at` hace que el vídeo empiece ahí.

```sh
gloss --frames 9 --output sheet.png clip.mp4
gloss --at 1:23 --output frame.png clip.mp4
gloss --at 0:05,0:30,1:00 --output-dir frames clip.mp4
gloss --text clip.mp4             # its subtitles, each after its times
gloss --at 1:23 clip.mp4          # play from 1:23
```

## Mostrar un enlace como código QR

`--qr TEXTO` muestra TEXTO, normalmente una dirección web, como un código QR en
lugar de un archivo, para que lo escanee un teléfono o una tableta. Funciona
dondequiera que funcione una imagen: el visor, `--serve` (una página web, donde
un agente sin terminal puede mostrártelo), `--output` y `--print`. El visor y la
página muestran el texto sobre el código, salvo que `--prompt` diga otra cosa. El
texto solo se codifica: no se descarga nada, y `GLOSS_NO_FETCH` no se aplica.

```sh
gloss --serve --qr http://192.168.1.20:8000/
gloss --qr https://example.com/ --output qr.png
```

Un código para una dirección `localhost` no sirve en otro dispositivo; indica la
dirección a la que el dispositivo pueda llegar.

## Abrir una dirección web

Cualquier argumento que sea una URL `http` o `https` se descarga antes de que
gloss arranque y ocupa el lugar del archivo, así que funciona donde funciona un
archivo: el visor, el menú, `--serve` y los modos sin interfaz `--info`,
`--text`, `--grep` y `--output`, y `--print`. Indicar la dirección ya es la
petición, así que no hace falta ninguna opción. La descarga dispone de un minuto
y del límite de 128 MiB por archivo, sigue como máximo diez redirecciones y nunca
de https a http, no envía cookies ni credenciales almacenadas, y debe ser un
formato que gloss muestre; `--type` y `--accept` se aplican como a un archivo.
Otros esquemas, como `ftp://`, son errores, y también lo es una descarga que
falla, exactamente igual que un archivo que no se puede abrir, salvo que `--info`
la comunica entre las demás, bajo su dirección, y continúa. El archivo recibe el
nombre del final de la dirección y se elimina cuando gloss termina, a menos que
se haya elegido. Un `--serve` o `--pick` desacoplado descarga antes de que arranque
el servidor, en la carpeta privada de la sesión, donde una descarga elegida se
queda después para quien llamó. Solo se descargan los argumentos que son
archivos; una dirección dada a una opción, como `--qr` o `--prompt`, se pasa tal
cual.

Las descargas se identifican como `gloss/` y la versión, tal como la imprime
`gloss --version`. Un anfitrión que quiera otro nombre lo indica con
`--user-agent`, o con `GLOSS_USER_AGENT` en el entorno cuando no se da la opción:
una línea de como máximo 1 KiB sin caracteres de control. Se envía el mismo
nombre para las direcciones que se abren dentro de documentos con `--fetch`.

Un argumento URL es una petición de red. Un guion que pase nombres que no eligió
puede prohibirlo definiendo `GLOSS_NO_FETCH` con cualquier valor no vacío en el
entorno: entonces los argumentos de dirección fallan y `--fetch` se rechaza. No
pongas contraseñas en las direcciones; se enviarían como autenticación básica y
quedarían en el historial de tu intérprete de comandos. Un archivo local cuyo
nombre empiece por `http://` no se puede nombrar en la línea de comandos; no hay
ninguno.

Las opciones usan la sintaxis GNU de `pflag` y pueden aparecer antes o después
de los nombres de archivo. Tanto `--page=3` como `-p3` funcionan; las opciones
cortas booleanas se pueden agrupar (`-mP`). Usa `--` para terminar el análisis de
opciones. `--type image|svg|pdf|stl|3mf|obj|ply|geojson|gpx|pmtiles|docx|xlsx|grist|csv|json|ipynb|html|text|markdown|mermaid|video` (`jsonl` y `ndjson` significan `json`, `md` significa `markdown`) sustituye la detección para todas las
entradas. La detección por contenido admite archivos sin extensión. Un `-` lee
la entrada estándar una vez en un archivo temporal, que se elimina al salir; la
entrada del teclado viene de la terminal de control. La salida interactiva debe
ser una terminal; la exportación de imágenes funciona en guiones.
`gloss --help` lista las opciones.

Los archivos no admitidos, los directorios y otras entradas que no son archivos
regulares se comunican en la salida de error y se omiten, de modo que los
patrones glob pueden incluir entradas no relacionadas. Los directorios no se
recorren. Si no queda ninguno, gloss termina con un error, salvo que se haya
nombrado una carpeta: entonces abre el explorador de archivos allí. Los archivos
admitidos siguen sujetos a los límites de tamaño; `--type` fuerza explícitamente
el formato de una entrada.

Opciones cortas habituales: `-h` ayuda, `-V` versión, `-m` menú, `-P` vista
previa, `-X` pantalla principal, `-N` números de línea, `-S` recortar líneas
largas, `-p` página, `-d` DPI, `-r` modo de dibujado, `-t` tipo, `-o` PNG de
salida, `-O` directorio de salida y `-s` borde máximo de imagen.

```sh
gloss report.pdf -p3 -o page.png --vision-profile openai-high
gloss photo.png drawing.svg model.stl -mP
```

## Elegir una vista

- Lee Markdown como texto de terminal, o pulsa `t` para una página tipográfica
  con imágenes locales, GIF y WebP animados, y código resaltado. El texto, el
  código fuente y JSON también admiten `t`. Elige fuentes con `f` y temas con
  `T`; consulta [Markdown](/gloss/docs/guide/markdown/) (English).
- Busca texto de documentos, celdas de tablas o texto de PDF con `/`. En código
  fuente y texto, `#` muestra números de línea, `:` va a una línea y `S`
  alterna el ajuste de líneas largas.
- Pausa un GIF o WebP animado independiente con Space, cambia la velocidad con
  `<`/`>`, y desplázate por los fotogramas o avanza de uno en uno. Reproduce un
  vídeo (MP4, MOV, WebM, MKV, OGV, AVI y más) con Space, avanza con ←/→ o con
  la barra; consulta [Formatos](/gloss/docs/guide/formats/) (English) para saber
  qué necesita. Orbita mallas STL, 3MF, OBJ y PLY y elige piezas de 3MF con `c`.
  Desplaza y amplía un mapa GeoJSON o GPX, o un archivo PMTiles, dibujados sin conexión.
- Pulsa `e` para exportar la imagen actual o el área tipográfica. Los guiones usan
  `--output` sin terminal; las mallas pueden exportar varias vistas en una hoja.
- Muestra la URL de una tabla como QR con `u`, o usa `--pick-web` para que el
  teléfono suba archivos y envíe respuestas. Consulta el
  [traspaso de archivos](/gloss/docs/guide/handoff/) (English) para la
  configuración de la red local y las sesiones de agentes.

La [referencia de controles](/gloss/docs/guide/controls/) (English) recoge las
teclas de cada tipo de medio.

## Abrir en otra aplicación

Las aplicaciones que ofrece gloss, y cuál es la predeterminada para cada tipo de
archivo, vienen de tu sistema operativo: Launch Services en macOS, las entradas
de escritorio XDG y los valores MIME por omisión en Linux y BSD, el registro en
Windows. gloss no añade ningún catálogo propio.

En el visor nativo, `w` abre el archivo actual en su aplicación predeterminada,
`W` abre un selector **Open with** y `E` edita el archivo. En el menú de archivos
o en la cuadrícula de miniaturas estas teclas actúan sobre el archivo resaltado.
En el explorador de archivos, Alt+Enter abre el selector **Open with** sin elegir
la entrada; los directorios solo ofrecen Reveal (mostrar en la carpeta). Si tu terminal no
comunica Alt+Enter, abre un archivo compatible con Enter y pulsa `W`, o configura
la asignación de teclas de la terminal.

El selector lista System default, las aplicaciones que tu sistema
conoce para este tipo de archivo, Reveal in Finder/Explorer (la carpeta que lo
contiene en Linux) y Copy path. Usa las flechas para elegir y Enter para abrir.
Pulsa `d` sobre una aplicación para guardarla como predeterminada para este tipo
de archivo y verbo; esto no la lanza. En Linux, una aplicación registrada para un
tipo general se ofrece también para sus subtipos, así que un editor de texto
plano aparece para código fuente, Markdown y JSON. Esc cierra el selector; una
elección guardada se conserva.

`E` edita con la aplicación resuelta para editar. Para texto, Markdown, JSON,
HTML, SVG, CSV, Mermaid, OBJ y cuadernos, es `$VISUAL`, con `$EDITOR` como
alternativa cuando VISUAL está vacío, salvo que hayas guardado otra
predeterminada, siempre que nombre un programa instalado; en otro caso, y para
los demás archivos, es el editor predeterminado del sistema. Cuando no hay
ninguno, se abre en su lugar el selector **Edit with**. Un archivo binario nunca
se envía al editor de terminal, y cuando ninguna aplicación puede editar un
archivo, gloss lo dice. Tab en el selector alterna entre **Open with** y
**Edit with**; las predeterminadas se guardan por tipo de archivo y por verbo,
así que la aplicación con la que abres un archivo Markdown puede ser distinta de
aquella en la que lo editas. En **Open with**, elige System default y
pulsa `d` para quitar una sustitución. Por ejemplo, ejecuta
`EDITOR=nano gloss README.md` o define `VISUAL='nvim -f'` en tu intérprete de comandos.

Una aplicación de terminal (un editor de terminal, o en Linux una entrada de
escritorio marcada con `Terminal=true`) toma el control de la terminal: gloss
suspende su interfaz, espera a la aplicación y luego recarga el archivo,
conservando en lo posible la vista del código fuente y la posición de
desplazamiento. El menú y el explorador vuelven a su selección anterior. Usa un
editor que permanezca en primer plano hasta que termines.

Pulsa `m` en el selector para gestionar las aplicaciones instaladas en todos los
formatos: las flechas seleccionan, Space oculta o restaura una aplicación y
guarda de inmediato, y Enter o Esc vuelve al selector. Las aplicaciones ocultas
siguen apareciendo en esta lista. System default y Reveal no se pueden ocultar, y las aplicaciones de terminal no aparecen. Ocultar
una predeterminada guardada usa la predeterminada del sistema hasta que esa
aplicación se restaure.

Las preferencias están en `gloss/open-with-v1.json` dentro de la carpeta de
configuración de usuario de tu sistema operativo. Contienen identificadores de
aplicación, nunca comandos ejecutables. Una aplicación guardada que no esté
disponible recurre a la predeterminada del sistema. Si falta ese archivo, gloss
lee el antiguo `gloss/open-with.json` mientras el nuevo no exista, incluidos sus
identificadores cortos (`vscode`, `preview`, ...), y lo deja intacto; tus
elecciones se guardan en el archivo nuevo. Si `open-with-v1.json` está dañado,
gloss lo comunica y no guarda hasta que lo repares o lo borres. En Linux, evita
que gloss sea su propio abridor predeterminado.

`gloss --apps` lista las aplicaciones disponibles por formato y verbo, con la
predeterminada efectiva; `--apps --json` ofrece el mismo listado para guiones sin
abrir nada. No se admiten comandos personalizados de aplicaciones gráficas. La
demostración en el navegador, el visor servido y el selector web no pueden lanzar
aplicaciones del anfitrión.

Abrir un archivo de la entrada estándar o descargado no prolonga su vida útil.
Guarda una copia desde la otra aplicación antes de que gloss termine si quieres
conservarlo. Abrir envía una petición de lanzamiento; gloss no puede saber si la
otra aplicación mostró el archivo.

El explorador de archivos tiene su propia ayuda: pulsa `?` antes de escribir un
filtro, o `F1` mientras escribes. Lista los controles de navegación,
autocompletado, disposiciones, filtros por tipo, ordenación, búsqueda y apertura
con aplicaciones. Las flechas y PageUp/PageDown desplazan la ayuda; Esc o F1 vuelve
a la misma carpeta, filtro y selección. Un `?` dentro de un filtro existente
sigue formando parte de la búsqueda de nombres de archivo.
