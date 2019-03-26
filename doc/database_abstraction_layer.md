# Database abstraction layer

One major challenge of implementing SQLFlow is achieving compatibility across various database backend. While both [Go's  `database/sql`](https://golang.org/pkg/database/sql/) and [Python's Database API](https://www.python.org/dev/peps/pep-0249/) have provided good database abstraction layers, there are still several inconsistencies among different databases requires special care.

This design documentation exam all interactions between SQLFlow and Databases, and explains how SQLFlow forms a relatively unified interface across different databases.

## Standard SQL

SQLFlow supports execution of all standard SQL statements. For example, if you want to join two tables, you may want to write:

- MySQL: `SELECT pet.name, comment FROM pet, event WHERE pet.name =event.name;` with keyword `WHERE` .
- Hive: `SELECT pet.name, comment FROM pet JOIN event ON (pet.name =event.name)` with keyword `JOIN` and `ON`.
- ODPS and SQLite uses either `INNER JOIN` or `OUTER JOIN`.

SQLFlow forwards the statement and lets the underlying database driver handle the SQL complications.

## Extended SQL

#### Describe Table

Different databases support different build-in statements to get a table schema. For example

- MySQL: `DESCRIBE/DESC my_table;`
- Hive: `DESCRIBE FORMATTED my_table;`
- ODPS: `DESC my_table;`
- SQLite: `PRAGMA table_info([my_table]);`

Their result formats are very different from each other. SQFlow avoids dealing with each database separately by running  `SELECT * FROM my_table LIMIT 1;` and inferring columnType through [DatabaseTypeName](https://golang.org/pkg/database/sql/#ColumnType.DatabaseTypeName) provided by the underlying driver.

#### Prepare prediction table

1. Drop previous prediction table `DROP TABLE IF EXISTS my_table;`
2. Create table with schema `CREATE TABLE my_table (name1, type1, name2 type2);`

Most databases support both statements.

#### Gerate Python Program

##### Translate columnType to tensorflow feature column type

After retrieving columnType through [DatabaseTypeName](https://golang.org/pkg/database/sql/#ColumnType.DatabaseTypeName), tensorflow feature column type can be derived via a mapping such as `{"FLOAT", "DOUBLE"} -> tf.numeric_column`.

##### Load data from database

Thanks to the Python database API, loading data from databases follows a similar API.

```python
conn = mysql.connector.connect(user='scott', password='password',
                               host='127.0.0.1',
                               database='employees')
conn = sqlite3.connect('path/to/your/sqlite/file')
conn = pyhive.connect('localhost')

cursor = conn.cursor()
cursor.execute('select * from my_table;')
```

##### Insert prediction result into the prediction table

Python database API provides `execute_many(sql, value)`  to insert multiple values at once. So one can prepare the following insertion statement.

```sql
-- MySQL, SQLite
INSERT INTO table_name VALUES (value1, value2, value3, ...);
-- Hive, ODPS
INSERT INTO TABLE table_name VALUES (value1, value2, value3, ...);
```

#### Save Model

SQLFlow saves trained ML model by dumping the serialized the model directory into a table. It first creates a table by `CREATE TABLE IF NOT EXISTS %s (id INT AUTO_INCREMENT, block BLOB, PRIMARY KEY (id))` and insert blobs by `INSERT INTO %s (block) VALUES(?)`.

Note that Hive and ODPS doesn't have `BLOB` type, we need to use `BINARY` (docs at [ODPS](https://help.aliyun.com/document_detail/27821.html?spm=a2c4g.11186623.6.577.768231deoru03E), [Hive](https://cwiki.apache.org/confluence/display/Hive/LanguageManual+Types#LanguageManualTypes-MiscTypes)) instead.

Also, note that Hive and ODPS doesn't support `AUTO_INCREMENT`, we need to implemented auto increment logic in `sqlfs`.

#### Load Model

SQLFlow loads trained ML model by reading rows in a table and deserializing the blob to a model directory.

It reads rows by running `SELECT block FROM %s ORDER BY id`, which is supported by most databases.
