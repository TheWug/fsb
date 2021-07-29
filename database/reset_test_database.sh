#!/bin/bash

user="${1:-fsb_test}"
dbname="${2:-furrysmutbot_test}_readonly"
host="${3:-127.0.0.1}"
port="${4:-5432}"

echo -n "Enter the database password for $user (default: their username) to refresh into $dbname.  "
psql -U "$user" -d "$dbname" -h "$host" -p "$port" <<< "
	DROP SCHEMA fsb_test;
	CREATE SCHEMA AUTHORIZATION fsb_test;
"

echo -n "Enter the database password for $user (default: their username) to import committed schema into $dbname.  "
sed -e 's/fsb_prod\./fsb_test./g' schema.psql | psql -U "$user" -d "$dbname" -h "$host" -p "$port"

echo -n "Enter the database password for $user (default: their username) to import committed test dataset into $dbname.  "
sed -e 's/fsb_prod\./fsb_test./g' dataset.psql | psql -U "$user" -d "$dbname" -h "$host" -p "$port"
