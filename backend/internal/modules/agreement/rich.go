package agreement

import (
	"strconv"
	"strings"
	"unicode"

	"baituber.cn/yanxuan/v2/internal/platform/app"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func ValidatedHTML(markup string) string {
	_, rich, err := agreementBody(markup)
	if err != nil {
		return ""
	}

	return rich
}

// Both clients render this same attribute-free HTML. Older clients receive the derived text.
func agreementBody(markup string) (string, string, error) {
	root := &html.Node{
		Type:     html.ElementNode,
		Data:     "div",
		DataAtom: atom.Div,
	}
	nodes, err := html.ParseFragment(strings.NewReader(markup), root)
	if err != nil {
		return "", "", app.Invalid("协议正文格式无效")
	}

	var text plainBody
	var rich strings.Builder
	for _, node := range nodes {
		if err := text.appendNode(node); err != nil {
			return "", "", err
		}

		if err := html.Render(&rich, node); err != nil {
			return "", "", err
		}
	}

	if !text.visible {
		return "", "", app.Invalid("请输入协议正文")
	}

	return strings.TrimRight(text.out.String(), "\n"), rich.String(), nil
}

type plainBody struct {
	out     strings.Builder
	visible bool
}

func (body *plainBody) appendNode(node *html.Node) error {
	if node.Type == html.TextNode {
		// Quill exports ordinary spaces as nonbreaking spaces in semantic HTML.
		node.Data = strings.ReplaceAll(node.Data, "\u00a0", " ")
		body.out.WriteString(node.Data)
		if strings.TrimFunc(node.Data, func(r rune) bool { return unicode.IsSpace(r) || unicode.Is(unicode.Cf, r) }) != "" {
			body.visible = true
		}

		return nil
	}

	if node.Type != html.ElementNode || node.Namespace != "" || len(node.Attr) != 0 {
		return app.Invalid("协议正文含有不支持的格式")
	}

	newline := false
	switch node.DataAtom {
	case atom.P, atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Li, atom.Br:
		newline = true
	case atom.Strong, atom.Em, atom.Ins, atom.Ol, atom.Ul:
	case atom.U:
		node.Data = "ins"
		node.DataAtom = atom.Ins
	default:
		return app.Invalid("协议正文含有不支持的格式")
	}

	index := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.DataAtom == atom.Li {
			index++
			switch node.DataAtom {
			case atom.Ol:
				body.out.WriteString(strconv.Itoa(index) + ". ")
			case atom.Ul:
				body.out.WriteString("• ")
			}
		}

		if err := body.appendNode(child); err != nil {
			return err
		}
	}

	if newline {
		body.out.WriteByte('\n')
	}

	return nil
}
