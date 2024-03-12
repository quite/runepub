package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/quite/runepub/internal/book"
)

func failf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func msgf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func main() {
	var (
		longNameFlag  bool
		overwriteFlag bool
		downloadFlag  bool
	)
	descDownload := "Download the zip-file by its titlekey"
	descLongName := "Use long output filename, including author, title etc"
	descOverwrite := "Overwrite existing output file"
	flag.BoolVar(&downloadFlag, "d", false, descDownload)
	flag.BoolVar(&longNameFlag, "l", false, descLongName)
	flag.BoolVar(&overwriteFlag, "f", false, descOverwrite)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  runepub [OPTIONS] ZIP-FILE
  runepub [OPTIONS] -d TITLEKEY

This program tries to convert a book zip-file from https://runeberg.org
into an EPUB file. It expects a typical 'titlekey-txt.zip' file as
input. If the '-d' flag is used, it will instead try to download the
file by its titlekey.

Default output filename: titlekey.epub

Options:
  -d  %s
  -l  %s
  -f  %s
`, descDownload, descLongName, descOverwrite)
	}
	flag.Parse()

	if flag.NArg() != 1 {
		if downloadFlag {
			fmt.Fprintf(os.Stderr, "Pass the titlekey of a book to download.\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "Pass a book zip-file.\n\n")
		}
		flag.Usage()
		os.Exit(2)
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

	msgf("Author: %s\nTitle: %s\nLang: %s\n", b.Author, b.Title, b.Language)
	if b.MaybeMissingBFL {
		msgf("NOTE: Book maybe missing blank first line for new paragraph!\n")
	}
	msgf("Chapters: %s\n", strings.Join(b.Chapters.Titles(), "; "))

	outname := fmt.Sprintf("%s.epub", b.TitleKey)
	if longNameFlag {
		outname = fmt.Sprintf("%s - %s", b.Author, b.Title)
		if b.Year != "" {
			outname += fmt.Sprintf(" (%s)", b.Year)
		}
		outname += fmt.Sprintf(" [runeberg-%s].epub", b.TitleKey)
	}

	if !overwriteFlag {
		if _, err := os.Stat(outname); err == nil || !os.IsNotExist(err) {
			failf("Output file %q exists", outname)
		}
	}

	f, err := os.Create(outname)
	if err != nil {
		failf("Create failed: %s", err)
	}
	defer f.Close()

	if err = b.WriteEPUB(f); err != nil {
		failf("WriteEPUB failed: %s", err)
	}
	msgf("Wrote %s\n", outname)
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
