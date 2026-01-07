bin = idx

test:
	go test -short -v ./...

test/full:
	go test -v ./...


test/integration:
	go test -v -tags=integration ./...

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

testhelper/smb:
	$(eval SMB_TEST_DIR := $(shell mktemp -d))
	@echo 'psql_uri = "postgresql+psycopg2://foox:bary@a-db:5432/mydatabase"' > $(SMB_TEST_DIR)/foo.py
	@echo '1' > $(SMB_TEST_DIR)/id_rsa
	@cd $(SMB_TEST_DIR) && zip secrets.zip id_rsa
	@cd $(SMB_TEST_DIR) && rm id_rsa
	# Create deep nested folder structure for testing folder caching
	# depth 0: root files (foo.py, secrets.zip)
	# depth 1: level1/
	# depth 2: level1/level2/
	# depth 3: level1/level2/level3/
	# depth 4: level1/level2/level3/level4/
	@mkdir -p $(SMB_TEST_DIR)/level1/level2/level3/level4
	@echo 'shallow_secret = "sk_live_shallow123456789012"' > $(SMB_TEST_DIR)/level1/shallow.txt
	@echo 'deep_secret = "sk_live_deep12345678901234"' > $(SMB_TEST_DIR)/level1/level2/level3/deep.txt
	@echo 'deeper_secret = "sk_live_deeper1234567890"' > $(SMB_TEST_DIR)/level1/level2/level3/level4/deeper.txt
	@echo '.env content' > $(SMB_TEST_DIR)/level1/level2/level3/level4/.env
	docker run --rm \
		--name idx-smb-test \
		-p 445:445 \
		-v $(SMB_TEST_DIR):/share \
		dperson/samba \
		-u "testuser;testpass123" \
		-s "testshare;/share;yes;no;no;testuser"
