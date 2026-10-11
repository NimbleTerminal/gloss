package document

import (
	"fmt"
	"time"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark-emoji/definition"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Goldmark callbacks run synchronously and check the budget in both its block
// and inline phases. A timeout unwinds this parse; no abandoned task survives it.
type markdownBudget struct {
	until time.Time
	calls int
}

func (b *markdownBudget) check() {
	b.calls++
	if b.calls > 4<<20 || time.Now().After(b.until) {
		panic("Markdown parser work limit exceeded")
	}
}
func readMarkdownTree(source []byte) (ast.Node, error) {
	return recoverParse("reading the Markdown", func() (ast.Node, error) {
		if len(source) > MaxMarkdownBytes {
			return nil, fmt.Errorf("Markdown exceeds 2 MiB")
		}
		if err := checkMarkdownNesting(string(source)); err != nil {
			return nil, err
		}
		budget := &markdownBudget{until: time.Now().Add(parseDeadline)}
		p := goldmark.New(goldmark.WithExtensions(extension.GFM, extension.DefinitionList, extension.Footnote,
			emoji.New(emoji.WithEmojis(unicodeEmojis{definition.Github()})))).Parser()
		p.AddOptions(budget)
		budget.check()
		root := p.Parse(text.NewReader(source))
		normalizeMarkdownExtensions(root, source, budget.check)
		budget.check()
		return root, nil
	})
}

// GitHub also defines image-only aliases such as :shipit:. Keep those literal:
// rendered Markdown uses Unicode characters and never fetches emoji assets.
type unicodeEmojis struct{ definition.Emojis }

func (e unicodeEmojis) Get(name string) (*definition.Emoji, bool) {
	v, ok := e.Emojis.Get(name)
	return v, ok && v.IsUnicode()
}
func (b *markdownBudget) SetParserOption(c *parser.Config) {
	for i, p := range c.BlockParsers {
		c.BlockParsers[i].Value = &budgetBlock{BlockParser: p.Value.(parser.BlockParser), b: b}
	}
	for i, p := range c.InlineParsers {
		c.InlineParsers[i].Value = &budgetInline{InlineParser: p.Value.(parser.InlineParser), b: b}
	}
	for i, p := range c.ParagraphTransformers {
		c.ParagraphTransformers[i].Value = &budgetParagraph{ParagraphTransformer: p.Value.(parser.ParagraphTransformer), b: b}
	}
	for i, p := range c.ASTTransformers {
		c.ASTTransformers[i].Value = &budgetAST{ASTTransformer: p.Value.(parser.ASTTransformer), b: b}
	}
}

type budgetBlock struct {
	parser.BlockParser
	b *markdownBudget
}

func (p *budgetBlock) Open(n ast.Node, r text.Reader, c parser.Context) (ast.Node, parser.State) {
	p.b.check()
	return p.BlockParser.Open(n, r, c)
}
func (p *budgetBlock) Continue(n ast.Node, r text.Reader, c parser.Context) parser.State {
	p.b.check()
	return p.BlockParser.Continue(n, r, c)
}
func (p *budgetBlock) Close(n ast.Node, r text.Reader, c parser.Context) {
	p.b.check()
	p.BlockParser.Close(n, r, c)
}
func (p *budgetBlock) SetOption(n parser.OptionName, v any) {
	if s, ok := p.BlockParser.(parser.SetOptioner); ok {
		s.SetOption(n, v)
	}
}

type budgetInline struct {
	parser.InlineParser
	b *markdownBudget
}

func (p *budgetInline) Parse(n ast.Node, r text.Reader, c parser.Context) ast.Node {
	p.b.check()
	return p.InlineParser.Parse(n, r, c)
}
func (p *budgetInline) CloseBlock(n ast.Node, r text.Reader, c parser.Context) {
	p.b.check()
	if s, ok := p.InlineParser.(parser.CloseBlocker); ok {
		s.CloseBlock(n, r, c)
	}
}
func (p *budgetInline) SetOption(n parser.OptionName, v any) {
	if s, ok := p.InlineParser.(parser.SetOptioner); ok {
		s.SetOption(n, v)
	}
}

type budgetParagraph struct {
	parser.ParagraphTransformer
	b *markdownBudget
}

func (p *budgetParagraph) Transform(n *ast.Paragraph, r text.Reader, c parser.Context) {
	p.b.check()
	p.ParagraphTransformer.Transform(n, r, c)
	p.b.check()
}

type budgetAST struct {
	parser.ASTTransformer
	b *markdownBudget
}

func (p *budgetAST) Transform(n *ast.Document, r text.Reader, c parser.Context) {
	p.b.check()
	p.ASTTransformer.Transform(n, r, c)
	p.b.check()
}
