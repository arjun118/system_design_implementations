.PHONY: migrate-create migrate-up migrate-down migrate-version migrate-force

migrate-create:
	migrate create -ext sql -dir src/migrations -seq $(name)

migrate-up:
	migrate -path src/migrations -database "$(JOBQ_DB_DSN)" up

migrate-down:
	migrate -path src/migrations -database "$(JOBQ_DB_DSN)" down 1

migrate-version:
	migrate -path src/migrations -database "$(JOBQ_DB_DSN)" version

migrate-force:
	migrate -path src/migrations -database "$(JOBQ_DB_DSN)" force -- $(version)
