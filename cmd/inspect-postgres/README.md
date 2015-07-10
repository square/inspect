### inspect-postgres

**inspect-postgres** is a command line utility that gives a brief overview of: version, uptime, queries made, and database sizes.

#### Usage

./bin/inspect-postgres

```
--------------------------
Version: 5.1234
Queries made: 123456
Uptime: 543210
Database sizes:
    database_name: 0.54 GB
    other_database_name: 12.31 GB

```

### Server

**inspect-postgres** can be run in server mode to run continuously and expose all metrics via HTTP JSON api

./bin/inspect-postgres -server -address :12345

```
[
{"type": "counter", "name": "postgresstat.Queries", "value": 9342251, "rate": 31.003152},
{"type": "counter", "name": "postgrestablestat.database_name.table_name.RowsRead", "value": 0, "rate": 0.000000},
{"type": "counter", "name": "postgrestablestat.database_name.table_name.RowsChanged", "value": 0, "rate": 0.000000},
{"type": "counter", "name": "postgrestablestat.database_name.other_table_name.RowsChanged", "value": 0, "rate": 0.000000},
{"type": "counter", "name": "postgrestablestat.database_name.table_name.RowsChangedXIndexes", "value": 0, "rate": 0.000000},
... truncated
{"type": "counter", "name": "postgresstat.SortMergePasses", "value": 0, "rate": 0.000000}]
```
