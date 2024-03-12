package book

import (
	"encoding/base64"
	"fmt"
	"io"

	"github.com/go-shiori/go-epub"
)

const css = `
p {
  text-indent: 0;
  margin-top: 0;
}

p + p {
  margin-top: 1.5ex;
}

h1, h2, h3,
p.center, div.center {
  text-align: center;
}

hr {
  border: 1px solid black;
}

span.spaced {
  letter-spacing: 0.1rem;
}

span.smallcaps {
  font-variant: small-caps;
}

span.big {
  font-size: 130%;
}

span.footnote {
  font-size: 80%;
}

td._c {
  text-align: center;
}
td._r {
  text-align: right;
}
`

func (b *Book) WriteEPUB(w io.Writer) error {
	e, err := epub.NewEpub(b.Title)
	if err != nil {
		return fmt.Errorf("NewEpub failed: %w", err)
	}

	e.SetAuthor(b.Author)
	e.SetLang(b.Language)
	e.SetIdentifier(b.URL)

	dataURI := fmt.Sprintf("data:%s;base64,%s", "text/css", base64.StdEncoding.EncodeToString([]byte(css)))

	cssPath, err := e.AddCSS(dataURI, "style.css")
	if err != nil {
		return fmt.Errorf("AddCSS failed: %w", err)
	}

	for _, ch := range b.Chapters {
		if _, err = e.AddSection(ch.Body, ch.Title, "", cssPath); err != nil {
			return fmt.Errorf("AddSection failed: %w", err)
		}
	}

	if _, err = e.WriteTo(w); err != nil {
		return fmt.Errorf("WriteTo failed: %w", err)
	}

	return nil
}
