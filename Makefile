# datavase — development tasks
#
# The integration tests need a real MariaDB. `make db-up` starts one on a
# non-standard port so it can never collide with a local MySQL install.

BINARY      := dv
DB_NAME     := datavase-test-db
DB_PORT     := 13306
DB_PASSWORD := datavase-test
DB_DATABASE := datavase_test
DB_IMAGE    := mariadb:11.4

.PHONY: build test test-integration lint db-up db-down db-shell clean

## build: compile the binary without CGO, as a single static executable
build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/dv

## test: run the unit tests (no database required)
test:
	go test ./...

## test-integration: run every test, including those needing MariaDB
test-integration:
	go test -tags integration ./...

lint:
	go vet ./...
	gofmt -l .

## db-up: start the test database and wait until it accepts connections
db-up:
	@docker rm -f $(DB_NAME) >/dev/null 2>&1 || true
	docker run -d --name $(DB_NAME) \
		-e MARIADB_ROOT_PASSWORD=$(DB_PASSWORD) \
		-e MARIADB_DATABASE=$(DB_DATABASE) \
		-p $(DB_PORT):3306 \
		$(DB_IMAGE) >/dev/null
	@printf 'waiting for MariaDB'
	@for i in $$(seq 1 60); do \
		if docker exec $(DB_NAME) mariadb-admin ping -uroot -p$(DB_PASSWORD) --silent >/dev/null 2>&1; then \
			echo ' ready on port $(DB_PORT)'; exit 0; \
		fi; \
		printf '.'; sleep 1; \
	done; \
	echo ' timed out'; docker logs --tail 30 $(DB_NAME); exit 1

## db-down: remove the test database
db-down:
	@docker rm -f $(DB_NAME) >/dev/null 2>&1 || true
	@echo 'removed $(DB_NAME)'

## db-shell: open a MariaDB shell against the test database
db-shell:
	docker exec -it $(DB_NAME) mariadb -uroot -p$(DB_PASSWORD) $(DB_DATABASE)

clean:
	rm -f $(BINARY)
