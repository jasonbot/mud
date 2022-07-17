OUTFILES := $(patsubst cmd/%/main.go,bin/%,$(wildcard cmd/*/main.go))

bin/%: cmd/%/main.go
	go build -o $@ $<

all: clean mod $(OUTFILES)

mod:
	go mod download

clean:
	rm bin/* || true
