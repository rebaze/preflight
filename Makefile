.PHONY: build check packaging security

PREFLIGHT_CONFTEST ?= conftest

build:
	go build -trimpath -o bin/preflight ./cmd/preflight

check:
	go test -race ./...
	go vet ./...
	"$(PREFLIGHT_CONFTEST)" verify --policy policy
	python3 -m unittest discover -s tools -p '*_test.py'
	$(MAKE) build

packaging:
	actionlint
	shellcheck .github/actions/setup-conftest/install.sh
	goreleaser check
	goreleaser release --snapshot --clean --skip=publish,sign,announce
	python3 tools/check-packages.py dist

security:
	go -C tools/security mod tidy -diff
	go -C tools/security mod verify
	go -C tools/security tool govulncheck -C ../.. -format text ./...
	go -C tools/security tool govulncheck -format text golang.org/x/vuln/cmd/govulncheck
