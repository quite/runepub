build:
	go build -o . ./cmd/...

update-lists:
	curl -L -fsS -o internal/book/a.lst https://runeberg.org/authors/a.lst
	curl -L -fsS -o internal/book/t.lst https://runeberg.org/authors/t.lst

books=drglas korkarlen dubbelmord kalocain

all:
	for t in ${books}; do ./test $$t; done
all-check:
	for t in ${books}; do ./test -c $$t; done

proper: build
	for t in ${books}; do ./runepub -d -l -f $$t; done
