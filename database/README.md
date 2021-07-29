## Using the database:

To initialize the testing database for the first time, use the included shell script `./initialize_test_database.sh [user] [dbname] [host] [port]`. Sensible defaults are used if no user or database name is provided.  This script will create the read-only and read-write testing databases and import the currently committed schema into the read only database, and populate it with the test dataset.

For simply resetting the database from the committed schema, use `./reset_test_database.sh [user] [dbname] [host] [port]`. You can do this at any time, however you should be aware that it will immediately and irrevocably destroy all data in the testing schema.

To export the contents and schema of the test database, use `./export_test_database.sh [user] [dbname] [host] [port]`.
