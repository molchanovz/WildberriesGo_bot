NAME := tradebot

GOFLAGS=-mod=vendor

PKG := `go list ${GOFLAGS} -f {{.Dir}} ./...`

ifeq ($(RACE),1)
	GOFLAGS+=-race
endif

LINT_VERSION := v2.1.6

MAIN := ${NAME}/cmd/${NAME}

tools:
	#@go install github.com/vmkteam/mfd-generator@latest
	#@go install github.com/vmkteam/zenrpc/v2/zenrpc@latest
	#@go install github.com/vmkteam/colgen/cmd/colgen@latest
	@curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin ${LINT_VERSION}

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

build:
	@CGO_ENABLED=0 go build $(GOFLAGS) -o ${NAME} $(MAIN)

run:
	@echo "Compiling"
	@go run $(GOFLAGS) $(MAIN) -config=cfg/local.toml -dev

generate:
	@go generate ./pkg/rpc
	@go generate ./pkg/vt

test:
	@echo "Running tests"
	@go test -count=1 $(GOFLAGS) -coverprofile=coverage.txt -covermode count $(PKG)

test-short:
	@go test $(GOFLAGS) -v -test.short -test.run="Test[^D][^B]" -coverprofile=coverage.txt -covermode count $(PKG)

mod:
	@go mod tidy
	@go mod vendor
	@git add vendor

db:
	@dropdb --if-exists tradebot
	@createdb tradebot
	@psql -f docs/tradebot.sql tradebot
	@psql -f docs/init.sql tradebot

NS := "tradebot"

MAPPING := "tradebot:cabinets,orders,stocks,users,reviews,products"

mfd-xml:
	@mfd-generator xml -c "postgres://sergey:1719@localhost:5432/tradebot?sslmode=disable" -m ./docs/model/tradebot.mfd -n $(MAPPING)
mfd-model:
	@mfd-generator model -m ./docs/model/tradebot.mfd -p db -o ./pkg/db
mfd-repo: --check-ns
	@mfd-generator repo -m ./docs/model/tradebot.mfd -p db -o ./pkg/db -n $(NS)
mfd-db-test:
	@mfd-generator dbtest -m docs/model/$(NAME).mfd -o ./pkg/db/test -x $(NAME)/pkg/db
mfd-vt-xml:
	@mfd-generator xml-vt -m ./docs/model/tradebot.mfd
mfd-vt-rpc: --check-ns
	@mfd-generator vt -m docs/model/tradebot.mfd -o pkg/vt -p vt -x tradebot/pkg/db -n $(NS)
mfd-xml-lang:
	#TODO: add namespaces support for xml-lang command
	@mfd-generator xml-lang  -m ./docs/model/tradebot.mfd
mfd-vt-template: --check-ns type-script-client
	@mfd-generator template -m docs/model/tradebot.mfd  -o ../gold-vt-master/ -n $(NS)

type-script-client: generate
	@go run $(GOFLAGS) $(MAIN) -config=cfg/local.toml -ts_client > ../gold-vt-master/src/services/api/factory.ts


--check-ns:
ifeq ($(NS),"NONE")
	$(error "You need to set NS variable before run this command. For example: NS=common make $(MAKECMDGOALS) or: make $(MAKECMDGOALS) NS=common")
endif
