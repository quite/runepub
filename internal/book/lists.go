package book

import (
	"bufio"
	_ "embed" // for go:embed
	"fmt"
	"os"
	"strings"
)

var authors = map[string]author{}

type author struct {
	FullName string
}

var titles = map[string]title{}

type title struct {
	title string
	year  string
}

//go:embed a.lst
var authorsData []byte

//go:embed t.lst
var titlesData []byte

func init() {
	authors = make(map[string]author)
	scanner := bufio.NewScanner(strings.NewReader(decodeISO8859_1(authorsData)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 7 {
			fmt.Fprintf(os.Stderr, "Bad line in embedded authors data: %s\n", line)
			os.Exit(1)
		}
		authors[parts[6]] = author{FullName: parts[3] + " " + parts[2]}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Scan failed: %s\n", err)
		os.Exit(1)
	}

	titles = make(map[string]title)
	scanner = bufio.NewScanner(strings.NewReader(decodeISO8859_1(titlesData)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || len(line) == 0 {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) != 9 {
			fmt.Fprintf(os.Stderr, "Bad line in embedded titles data: %s\n", line)
			os.Exit(1)
		}
		titles[parts[1]] = title{title: parts[0], year: parts[4]}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Scan failed: %s\n", err)
		os.Exit(1)
	}
}
