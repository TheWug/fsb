#!/bin/bash

user="${1:-fsb_test}"
dbname="${2:-furrysmutbot_test}_readonly"
host="${3:-127.0.0.1}"
port="${4:-5432}"

echo "
Preparing to create roles and databases for fsb testing.
This will create the role $user (password = username),
the database $dbname, and a schema fsb_test in which all
of $user's data will exist.

This should only be run once. But, if you have to run it again,
you should first drop the following items:
ROLE $user
DATABASE $dbname
SCHEMA fsb_test

Press enter to continue and enter your system
password to run DB statements as superuser."

sudo -u postgres psql -p "$port" <<< "
	CREATE ROLE $user LOGIN CONNECTION LIMIT 10 ENCRYPTED PASSWORD '$user';
	ALTER ROLE $user SET search_path TO fsb_test;
	CREATE DATABASE $dbname WITH OWNER $user TEMPLATE template0 ENCODING utf8;"

./reset_test_database.sh "$user" "$2" "$host" "$port"
