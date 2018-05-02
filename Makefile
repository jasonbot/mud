OUTFILES := $(patsubst cmd/%.go,bin/%,$(wildcard cmd/*.go))

bin/%: cmd/%.go
	go build -o $@ $<

all: clean dep $(OUTFILES)

dep:
	dep ensure

clean:
	rm bin/* || true