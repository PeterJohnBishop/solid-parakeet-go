# solid-parakeet-go

A free-form go learning repo.

## Postgres

docker exec -it <container_name_or_id> psql -U <postgres_user> -d <database_name> -c "TRUNCATE TABLE calendar_dates, shapes, stop_times CASCADE;"

OR

docker exec -it <container_name_or_id> psql -U <postgres_user> -d <database_name>

TRUNCATE TABLE calendar_dates, shapes, stop_times CASCADE;
\q
