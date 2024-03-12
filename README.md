Simple tool to download Public Domain books from https://runeberg.org
and produce reasonably readable epub files. Works for some books that
I've tried it on (see the `Makefile`).

This repository contains (possibly outdated) copies of the files
`https://runeberg.org/authors/{a,t}.lst` (issue `make update-lists` to
update them). This allows for easy installing by running:

```
go install github.com/quite/runepub/cmd/runepub@latest
```
