OUTFILES := $(patsubst cmd/%.go,bin/%,$(wildcard cmd/*.go))

bin/%: cmd/%.go
	go build -o $@ $<

all: $(OUTFILES) dep

dep:
	dep ensure

clean:
	rm bin/*