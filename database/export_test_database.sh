#!/bin/bash

user="${1:-fsb_test}"
dbname="${2:-furrysmutbot_test}_readonly"
host="${3:-127.0.0.1}"
port="${4:-5432}"

echo "Enter the database password for $user (default = $user) twice, once to export the schema and the second to export the data."
pg_dump -U "$user" -h "$host" -p "$port" -d "$dbname" --schema-only --no-owner | tee schema.psql
pg_dump -U "$user" -h "$host" -p "$port" -d "$dbname" --data-only | tee dataset.psql
