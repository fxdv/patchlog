.PHONY: build test unit integration e2e regression quality gate clean fmt vet cover bench go

BINARY   := patchlog
COVERDIR := coverage
BENCHDIR := bench

# Detect Go binary
GO_BIN   := $(shell command -v go 2>/dev/null || echo "$(HOME)/.local/go/bin/go")
GO       := $(GO_BIN)
CGO      := CGO_ENABLED=0
CGO_RACE := CGO_ENABLED=1
export PATH := $(dir $(GO_BIN)):$(PATH)

build:
	@bash scripts/build.sh

test: unit integration e2e

unit:
	$(CGO_RACE) $(GO) test ./pkg/... -v -count=1 -race

integration:
	$(CGO) $(GO) test ./tests/integration/... -v -count=1

e2e: build
	$(CGO) $(GO) test ./tests/e2e/... -v -count=1 -tags=e2e

regression: build
	$(CGO) $(GO) test ./tests/e2e/... -v -run TestGolden -count=1

quality: fmt vet build

fmt:
	@output=$$(gofmt -l . 2>/dev/null); \
	if [ -n "$$output" ]; then \
		echo "Files need formatting:"; \
		echo "$$output"; \
		exit 1; \
	fi; \
	echo "Format OK"

vet:
	$(CGO) $(GO) vet ./...

cover:
	@mkdir -p $(COVERDIR)
	$(CGO) $(GO) test ./pkg/... -coverprofile=$(COVERDIR)/coverage.out -covermode=atomic
	$(CGO) $(GO) tool cover -func=$(COVERDIR)/coverage.out | tail -1
	$(CGO) $(GO) tool cover -html=$(COVERDIR)/coverage.out -o $(COVERDIR)/coverage.html
	@echo "HTML report: $(COVERDIR)/coverage.html"

cover-check: cover
	@total=$$($(GO) tool cover -func=coverage/coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	if [ "$$(echo "$$total < 60" | bc -l)" -eq 1 ]; then \
		echo "Coverage $${total}% is below 60% threshold"; \
		exit 1; \
	fi; \
	echo "Coverage $${total}% meets 60% threshold"

bench:
	@mkdir -p $(BENCHDIR)
	$(CGO) $(GO) test ./pkg/... -bench=. -benchmem -count=3 -run=^$$ | tee $(BENCHDIR)/results.txt

gate: quality unit integration
	@echo ""
	@echo "All gates passed"

clean:
	rm -f $(BINARY)
	rm -rf $(COVERDIR) $(BENCHDIR)
