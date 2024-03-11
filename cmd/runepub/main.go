package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/go-shiori/go-epub"
	"github.com/quite/runepub/internal/book"
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

func failf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func msgf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func main() {
	var (
		simpleNameFlag bool
		overwriteFlag  bool
		downloadFlag   bool
	)
	flag.BoolVar(&simpleNameFlag, "n", false, "Output 'titlekey.epub' instead of 'Author - Title (year) [runeberg-titlekey].epub'")
	flag.BoolVar(&overwriteFlag, "f", false, "Overwrite existing output file")
	flag.BoolVar(&downloadFlag, "d", false, "First download book zip (titlekey-txt.zip) from runeberg.org (given titlekey)")
	flag.Parse()

	if flag.NArg() != 1 {
		if downloadFlag {
			failf("Pass the titlekey of a book to download, for example: drglas")
		} else {
			failf("Pass a book zip, for example: drglas-txt.zip")
		}
	}
	src := flag.Args()[0]

	if downloadFlag {
		if err := download(src); err != nil {
			failf("download failed: %s", err)
		}
		src = fmt.Sprintf("%s-txt.zip", src)
	}

	zipData, err := os.ReadFile(src)
	if err != nil {
		failf("ReadFile failed: %s", err)
	}

	b, err := book.New(zipData)
	if err != nil {
		failf("book.New failed: %s", err)
	}

	e, err := epub.NewEpub(b.Title)
	if err != nil {
		failf("NewEpub failed: %s", err)
	}

	e.SetAuthor(b.Author)
	e.SetLang(b.Language)
	e.SetIdentifier(b.URL)

	msgf("Author: %s\nTitle: %s\nLang: %s\n", b.Author, b.Title, b.Language)
	if b.MaybeMissingBFL {
		msgf("NOTE: Book maybe missing blank first line for new paragraph!\n")
	}

	cssPath, err := e.AddCSS(dataURI(css, "text/css"), "style.css")
	if err != nil {
		failf("AddCSS failed: %s", err)
	}

	sections := []string{}
	for _, ch := range b.Chapters {
		if _, err = e.AddSection(ch.Body, ch.Title, "", cssPath); err != nil {
			failf("AddSection failed: %s", err)
		}
		sections = append(sections, ch.Title)
	}
	msgf("Sections: %s\n", strings.Join(sections, ";"))

	var outname string
	if simpleNameFlag {
		outname = fmt.Sprintf("%s.epub", b.TitleKey)
	} else {
		outname = fmt.Sprintf("%s - %s", b.Author, b.Title)
		if b.Year != "" {
			outname += fmt.Sprintf(" (%s)", b.Year)
		}
		outname += fmt.Sprintf(" [runeberg-%s].epub", b.TitleKey)
	}
	if !overwriteFlag {
		if _, err := os.Stat(outname); err == nil || !os.IsNotExist(err) {
			failf("Output filename %q exists", outname)
		}
	}

	if err = e.Write(outname); err != nil {
		failf("Write failed: %s", err)
	}
	msgf("Wrote %s\n", outname)
}

func dataURI(s string, mimeType string) string {
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString([]byte(s)))
}

func download(titleKey string) error {
	zipFname := fmt.Sprintf("%s-txt.zip", titleKey)

	if _, err := os.Stat(zipFname); err == nil {
		msgf("Not downloading existing %s\n", zipFname)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("unhandled Stat err: %w", err)
	}

	msgf("Downloading %s ...\n", zipFname)
	url := fmt.Sprintf("https://runeberg.org/download.pl?mode=txtzip&work=%s", titleKey)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("Get failed: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(zipFname)
	if err != nil {
		return fmt.Errorf("Create failed: %w", err)
	}
	defer f.Close()

	if _, err = io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("Copy failed: %w", err)
	}

	return nil
}
