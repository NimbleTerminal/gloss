# Markdown extensions

A footnote can explain a detail without interrupting the paragraph.[^detail]
Repeated references use the same number.[^detail] Another note comes next.[^other]

> [!NOTE]
> Useful **information**, with a [link](https://example.org).

> [!TIP]
> Try both normal text and typeset mode with `t`.

> [!IMPORTANT]
> Both views keep the same content.

> [!WARNING]
> Read this before proceeding.
>
> - Lists stay inside the callout.
> - So does `inline code`.

> [!CAUTION]
> A potentially destructive action needs care.

Ordinary code stays literal: `[^detail]` and `[!NOTE]`.

[^detail]: Footnotes support **formatting** and multiple paragraphs.

    This second paragraph belongs to the first note.

[^other]: Another explanation, with `code`.
