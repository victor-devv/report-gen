db_login:
	psql ${DATABASE_URL}

migration:
	migrate create -ext sql -dir migrations -seq $(name)

migrate:
	migrate -database ${DATABASE_URL} -path migrations up