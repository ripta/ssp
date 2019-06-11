BUILD_DATE := `date -u +%Y%m%d`
VERSION := $(shell git describe --tags --dirty 2>/dev/null || echo v0.0.1)

PKGS = $(or $(PKG),$(shell go list ./...))
FIXFILES = $(shell goimports -l $(shell go list -f '{{.Dir}}' ./...))

V = 0
Q = $(if $(filter 1,$V),,@)

all: debug

clean:
	$Q rm bin/*

# Debug build: enable race detection, don't strip symbols
debug: fmt vet
	$Q go build -v -ldflags "-X main.BuildVersion=$(VERSION) -X main.BuildDate=$(BUILD_DATE)" -o bin/ssp -race ./cmd/ssp

dep-godoc:
	$Q go get -v golang.org/x/tools/cmd/godoc

dep-goimports:
	$Q go get -v golang.org/x/tools/cmd/goimports

doc: dep-godoc
	godoc -http=:8080 -index

docker-build:
	$Q docker build -t ripta/ssp:latest --build-arg BUILD_DATE=$(BUILD_DATE) --build-arg VERSION=$(VERSION) .

fmt: dep-goimports
	$Q for src in $(FIXFILES); \
		do \
			goimports -w $$src; \
		done

install: all
	$Q [ -e "$(GOPATH)/bin" ] || mkdir $(GOPATH)/bin
	$Q for bin in bin/*; \
		do \
			[ -e "$(GOPATH)/$$bin" ] && rm "$(GOPATH)/$$bin"; \
			cp -pv $$bin $(GOPATH)/bin; \
		done

release: fmt vet
	$Q go build -v -ldflags "-s -w -X main.BuildVersion=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.BuildEnvironment=prod" -o bin/ssp ./cmd/ssp
	$Q codesign -f --deep -s 'Ripta Pasay' bin/ssp

run: fmt
	$Q go run cmd/ssp/*.go

test:
	$Q go test -timeout 20s $(PKGS)

vet:
	$Q go vet ./...
