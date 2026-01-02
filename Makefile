bin = idx

test:
	go test -v ./...

test/ldapserver:
	docker run --rm -p 10389:10389 -p 10636:10636 ghcr.io/rroemhild/docker-test-openldap:master

install:
	go install -tags 'sqlite3' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

release:
	goreleaser release --clean

run: build
	./dist/$(bin)

audit:
	go mod verify
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	go test -race -vet=off ./...

db/generate:
	cd ./db && sqlc generate

db/reset:
	rm idx.db idx.db-shm idx.db-wal || true
