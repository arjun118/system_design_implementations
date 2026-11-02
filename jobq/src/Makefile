.PHONY: migrate-create migrate-up migrate-down migrate-version migrate-force


MIGRATE=docker compose run --rm migrate \
	-path ./migrations \
	-database "$(DATABASE_URL)"

migrate-create:
	migrate create -ext=.sql -dir ./migrations -seq $(name)

migrate-up:
	$(MIGRATE) up

migrate-down:
	$(MIGRATE)down 1

migrate-version:
	$(MIGRATE) version

migrate-force:
	$(MIGRATE) force -- $(version)

# tests

test_integration_phase1:
	go test -v -tags=integration ./tests/integration/... -run ^TestPhase1$
