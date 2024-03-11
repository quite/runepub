package process

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

// RunebergTxt tries to add (non-closed) <p> at the right places in a
// Runeberg txt-file.
func RunebergTxt(body string) (string, error) {
	var out string
	betweenParagraphs := false
	sawChapterTag := false

	// Note that we're relying on chapter and table (closing) tags
	// sitting at the beginning of a line.
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "<chapter") {
			if sawChapterTag {
				return "", fmt.Errorf("already got chapter tag")
			}
			sawChapterTag = true
			continue
		}
		if sawChapterTag {
			// TODO check that this p contains same as chapter tags
			// name attrib? and same as title from articles.lst?

			// What we have here is probably the title and a p-end
			// (due to the heavy ReplaceAll above). But the title
			// might be wrapped in some hX-tag
			if !strings.HasPrefix(line, "<h") {
				line = "<h1>" + line + "</h1>"
			}
			sawChapterTag = false
			// Don't introduce a new paragraph if we just dealt with <chapter>
			betweenParagraphs = false
		}
		if strings.HasPrefix(line, "</chapter") {
			continue
		}

		if line == "" {
			betweenParagraphs = true
			continue
		}

		// <table> must not be inside a <p>
		if betweenParagraphs && !strings.HasPrefix(line, "<table") {
			out += "\n<p>"
		}

		if strings.HasPrefix(line, "<table") {
			// We close the previous <p>aragraph here, otherwise Go's
			// Parser+Render ends up putting the table tag inside the
			// previous p tag, which is not permitted since they are
			// both block-level elements.
			if betweenParagraphs {
				out += "</p>\n"
			}
			out += "\n"
		}

		betweenParagraphs = false

		out += line + "\n"
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Scan failed: %w", err)
	}

	return out, nil
}

// RunebergHtml processes the Runeberg html-file, including trying
// closing the p-tags
func RunebergHtml(body string) (string, error) {
	body = preprocessRunebergHtml(body)

	doc, _ := html.Parse(bytes.NewReader([]byte(body)))
	bodyNode, err := getBody(doc)
	if err != nil {
		return "", err
	}

	processNodes(doc)

	body, err = render(bodyNode)
	if err != nil {
		return "", err
	}

	body = strings.TrimPrefix(body, "<body>")
	body = strings.TrimSuffix(body, "</body>")
	body = fmt.Sprintf("\n%s\n", body)

	// Ensure newlines after closing p-tag
	body = strings.ReplaceAll(body, "</p><", "</p>\n\n<")

	// Ensure space btw text and inline tags (want: `text <strong>`);
	// making some typographic exceptions (remember \s==whitespace,
	// \S==non-whitespace)
	re := regexp.MustCompile(`([^\s>’»])(<[a-z0-9_ ="]+>)`)
	body = re.ReplaceAllString(body, "${1} ${2}")

	// Ensure newline before hard linebreak
	re = regexp.MustCompile(`(.+)(<br/>)`)
	body = re.ReplaceAllString(body, "${1}\n${2}")

	if strings.Contains(body, `=""`) {
		return "", fmt.Errorf(`Found ="" in body, meaning there were htmlish tag with unhandled attribute`)
	}

	return body, nil
}

func preprocessRunebergHtml(s string) string {
	s = strings.ReplaceAll(s, "<sp>", `<span class="spaced">`)
	s = strings.ReplaceAll(s, "</sp>", "</span>")

	s = strings.ReplaceAll(s, "<sc>", `<span class="smallcaps">`)
	s = strings.ReplaceAll(s, "</sc>", "</span>")

	s = strings.ReplaceAll(s, "<big>", `<span class="big">`)
	s = strings.ReplaceAll(s, "</big>", "</span>")

	s = strings.ReplaceAll(s, "<footnote>", `<span class="footnote"> [fotnot: `)
	s = strings.ReplaceAll(s, "</footnote>", "]</span>")

	// Apparently XHTML and thus EPUB doesn't have (many) named entities
	// ensp 8194 0x2002 (approx 2 spaces)
	// emsp 8195 0x2003 (approx 4 spaces)
	s = strings.ReplaceAll(s, "<tab>", "\u2003\u2003")

	var re *regexp.Regexp

	re = regexp.MustCompile(`(<table|<td) ([a-z]+)>`)
	s = re.ReplaceAllString(s, `${1} class="_${2}">`)

	re = regexp.MustCompile(`(<td) ([1-9]+)([a-z]+)>`)
	s = re.ReplaceAllString(s, `${1} colspan="${2}" class="_${3}">`)

	s = strings.ReplaceAll(s, "\n<td", "\n<tr><td")

	return s
}

func getBody(doc *html.Node) (*html.Node, error) {
	var body *html.Node
	var dig func(*html.Node)

	dig = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "body" {
			body = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			dig(child)
		}
	}

	dig(doc)

	if body != nil {
		return body, nil
	}

	return nil, errors.New("Missing <body> in the node tree")
}

func processNodes(n *html.Node) {
	if n.Type == html.ElementNode && (n.Data == "p" || n.Data == "div") {
		// Add class="center" if we removed align=
		oldLen := len(n.Attr)
		n.Attr = slices.DeleteFunc(n.Attr, func(attr html.Attribute) bool {
			return attr.Key == "align"
		})
		if len(n.Attr) < oldLen {
			i := slices.IndexFunc(n.Attr, func(attr html.Attribute) bool {
				return attr.Key == "class"
			})
			if i > -1 {
				if n.Attr[i].Val != "" {
					n.Attr[i].Val += ","
				}
				n.Attr[i].Val += "center"
			} else {
				n.Attr = append(n.Attr, html.Attribute{Namespace: "", Key: "class", Val: "center"})
			}
		}
	}

	// Remove trailing newlines from text inside <p>
	if n.Type == html.TextNode {
		if n.Parent.Type == html.ElementNode && n.Parent.Data == "p" {
			n.Data = strings.TrimRight(n.Data, "\n")
		}
	}

	// Prefix relative URLs with runberg.org
	if n.Type == html.ElementNode && n.Data == "a" {
		i := slices.IndexFunc(n.Attr, func(attr html.Attribute) bool {
			return attr.Key == "href"
		})
		if i > -1 {
			url := n.Attr[i].Val
			if strings.HasPrefix(url, "/") {
				n.Attr[i].Val = "https://runeberg.org" + url
			}
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		processNodes(child)
	}
}

func render(n *html.Node) (string, error) {
	var buf bytes.Buffer

	if err := html.Render(io.Writer(&buf), n); err != nil {
		return "", fmt.Errorf("Render failed: %w", err)
	}

	return buf.String(), nil
}
