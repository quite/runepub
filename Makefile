build: fetch-lists
	go build -o . ./cmd/...

fetch-lists:
	[ -e internal/book/a.lst ] || curl -L -fsS -o internal/book/a.lst https://runeberg.org/authors/a.lst
	[ -e internal/book/t.lst ] || curl -L -fsS -o internal/book/t.lst https://runeberg.org/authors/t.lst

books=drglas korkarlen dubbelmord kalocain

all:
	for t in ${books}; do ./test $$t; done
all-check:
	for t in ${books}; do ./test -c $$t; done

proper: build
	for t in ${books}; do ./runepub -f -d $$t; done
