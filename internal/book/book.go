package book

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/quite/runepub/internal/process"
	"golang.org/x/text/encoding/charmap"
)

// TODO Handles both Metadata+Articles.lst with html-files (drglas)
// and with Pages/txt-files (korkarlen, dubbelmord). Well, only those
// specific books have been tried so far.
//
// Things are very messy, especially dealing with runebergs htmlish
// and lack of closing tags.

type Book struct {
	Title           string
	TitleKey        string
	Author          string
	Language        string
	URL             string
	Chapters        Chapters
	Year            string
	MaybeMissingBFL bool
}

type Chapters []Chapter

func (chs Chapters) Titles() []string {
	var titles []string
	for _, ch := range chs {
		titles = append(titles, ch.Title)
	}
	return titles
}

type Chapter struct {
	Title string
	Body  string // HTML to be wrapped in a body tag
	pages []string
}

func New(zipData []byte) (*Book, error) {
	var err error

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("NewReader failed: %w", err)
	}

	b := &Book{}

	if err = b.getMetadata(r); err != nil {
		return nil, err
	}

	if err = b.getFrontmatter(r); err != nil {
		return nil, err
	}

	var hasPages bool
	for _, file := range r.File {
		if file.Name == "Pages.lst" {
			hasPages = true
			break
		}
	}

	if hasPages {
		if err = b.getChapters(r); err != nil {
			return nil, err
		}
	} else {
		if err = b.getChaptersNoPages(r); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func (b *Book) getFrontmatter(fs fs.FS) error {
	f, err := fs.Open("index.html")
	if err != nil {
		return fmt.Errorf("Open failed: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("ReadAll failed: %w", err)
	}

	ch := Chapter{Title: "Titelsida"}
	body := string(data)
	body += fmt.Sprintf(`<hr/><p>Denna bok i EPUB-format har skapats från källfiler från Projekt Runeberg: <a href="%[1]s">%[1]s</a>.`, b.URL)
	body, err = process.RunebergHtml(body)
	if err != nil {
		return err
	}
	ch.Body = body

	b.Chapters = append(b.Chapters, ch)

	return nil
}

func (b *Book) getMetadata(fs fs.FS) error {
	f, err := fs.Open("Metadata")
	if err != nil {
		return fmt.Errorf("Open failed: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := decodeISO8859_1(scanner.Bytes())
		k, v, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		v = strings.Trim(v, " \t")
		switch k {
		case "TITLE":
			b.Title = v
		case "TITLEKEY":
			b.TitleKey = v
			b.URL = fmt.Sprintf("https://runeberg.org/%s/", b.TitleKey)
			if title, ok := titles[b.TitleKey]; ok {
				b.Year = title.year
			}
		case "AUTHORKEY":
			if author, ok := authors[v]; ok {
				b.Author = author.FullName
			} else {
				return fmt.Errorf("unknown AUTHORKEY: %s", v)
			}
		case "LANGUAGE":
			b.Language = v
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Scan failed: %w", err)
	}

	fields := []string{"Title", "TitleKey", "Author", "Language"}
	for _, f := range fields {
		if b.getStringField(f) == "" {
			return fmt.Errorf("%s not found in metadata", f)
		}
	}

	return nil
}

func (b *Book) getStringField(field string) string {
	v := reflect.ValueOf(b)
	f := reflect.Indirect(v).FieldByName(field)
	return string(f.String())
}

func decodeISO8859_1(in []byte) string {
	decoder := charmap.ISO8859_1.NewDecoder()
	out, err := decoder.Bytes(in)
	if err != nil {
		log.Panic(err)
	}
	return string(out)
}

func (b *Book) getChapters(fs fs.FS) error {
	// TODO Is Articles.lst in ISO-8859-1?
	f, err := fs.Open("Articles.lst")
	if err != nil {
		return fmt.Errorf("Open failed: %w", err)
	}
	defer f.Close()

	var chs []Chapter

	seqSingleRE := regexp.MustCompile(`^[0-9]{4}$`)
	seqRangeRE := regexp.MustCompile(`^[0-9]{4}-[0-9]{4}$`)
	// Expecting lines in Articles.lst like: |titeln|0005-0013
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "index|") || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 3 {
			return fmt.Errorf("Line does not have 3 fields: %q", line)
		}

		seq := parts[2]

		ch := Chapter{
			Title: parts[1],
		}

		switch {
		case seqSingleRE.MatchString(seq):
			ch.pages = append(ch.pages, seq)
		case seqRangeRE.MatchString(seq):
			parts := strings.Split(seq, "-")
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			if end <= start {
				return fmt.Errorf("Not handling sequence: %q", seq)
			}
			for i := start; i <= end; i++ {
				ch.pages = append(ch.pages, fmt.Sprintf("%04d", i))
			}
		default:
			return fmt.Errorf("Not handling sequence: %q", seq)
		}

		chs = append(chs, ch)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Scan failed: %w", err)
	}

	// TODO Do we need to lookup each article's pages in Pages.lst?
	// TODO skip having this extra loop here?
	var knownMissingBFL bool // BlankFirstLine
	var anyBFL bool
	for idx := range chs {
		var body string

		for _, page := range chs[idx].pages {
			f, err := fs.Open(path.Join("Pages", page+".txt"))
			if err != nil {
				return fmt.Errorf("Open failed: %w", err)
			}

			data, err := io.ReadAll(f)
			f.Close()
			if err != nil {
				return fmt.Errorf("ReadAll failed: %w", err)
			}

			if len(data) == 0 {
				return fmt.Errorf("Page file %s is empty", page)
			}

			// Strip the CRs ASAP
			s := strings.ReplaceAll(string(data), "\r", "")

			first := []rune(s)[0]
			isLowercase := func(r rune) bool { return unicode.IsLetter(r) && unicode.IsLower(r) }
			switch b.TitleKey {
			case "korkarlen":
				// We know that these books do not have a blank line
				// first on a page when it begins with a new
				// paragraph. Try to deal with it by inserting a blank
				// line when the first letter is not lowercase.
				if first != '\n' && !isLowercase(first) {
					body += "\n"
				}
				knownMissingBFL = true
			default:
				// We assume correct formatting, meaning that
				// pages-txts have a blank line first when the page
				// begins with a new paragraph. But note any
				// BlankFirstLine as heuristics for detecting if they
				// are missing entirely.
				if first == '\n' {
					anyBFL = true
				}
				// Still might be missing blank line before table tag
				if strings.HasPrefix(s, "<table") {
					body += "\n"
				}
			}

			body += s
		}

		body, err = process.RunebergTxt(body)
		if err != nil {
			return err
		}

		body, err := process.RunebergHtml(body)
		if err != nil {
			return err
		}

		chs[idx].Body = body
	}

	if !knownMissingBFL && !anyBFL {
		b.MaybeMissingBFL = true
	}

	if len(chs) == 0 {
		return fmt.Errorf("Got no chapters from Articles.lst")
	}

	b.Chapters = append(b.Chapters, chs...)

	return nil
}

func (b *Book) getChaptersNoPages(fs fs.FS) error {
	// TODO Is Articles.lst in ISO-8859-1?
	f, err := fs.Open("Articles.lst")
	if err != nil {
		return fmt.Errorf("Open failed: %w", err)
	}
	defer f.Close()

	var chs []Chapter

	re := regexp.MustCompile(`<h1>([^<]+)</h1>`)

	// Expecting lines in Articles.lst like: htmlbasename|Titel|
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fname, _, found := strings.Cut(scanner.Text(), "|")
		if !found || fname == "index" || strings.HasPrefix(fname, "#") {
			continue
		}

		f, err := fs.Open(fname + ".html")
		if err != nil {
			return fmt.Errorf("Open failed: %w", err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return fmt.Errorf("ReadAll failed: %w", err)
		}

		body := string(data)

		// TODO could get title from Articles.lst?
		match := re.FindStringSubmatch(body)
		if len(match) != 2 {
			return fmt.Errorf("no title found in %s", fname)
		}

		body, err = process.RunebergHtml(body)
		if err != nil {
			return err
		}

		chs = append(chs, Chapter{
			Title: match[1],
			Body:  body,
		})
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("Scan failed: %w", err)
	}

	if len(chs) == 0 {
		return fmt.Errorf("Got no chapters from Articles.lst")
	}

	b.Chapters = append(b.Chapters, chs...)

	return nil
}
