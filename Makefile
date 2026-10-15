VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/navi-crwn/pixelhop/internal/app.Version=$(VERSION)

.PHONY: run build test release clean
run:            ## start a dev server on :8080
	go run ./cmd/pixelhop
build:          ## build ./pixelhop
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o pixelhop ./cmd/pixelhop
test:
	go vet ./... && go test ./...
release:        ## cross-compile into dist/
	@for t in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
	  os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
	  echo "  $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/pixelhop-$$os-$$arch$$ext ./cmd/pixelhop; \
	done
clean:
	rm -rf pixelhop dist
