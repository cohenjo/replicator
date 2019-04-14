# Demo

## MySQL Setup

```sql
(db-mysql-dbdev0a.42):[test]> create table names(id binary(16) PRIMARY KEY, name varchar(200), year int, gender varchar(200), count int);
Query OK, 0 rows affected (0.01 sec)

```

The matching stream/estuary configuration:
```json
{"Type":"MYSQL","Host": "db-mysql-dbdev0a.42", "Port":3306, "Schema":"test","Collection":"names"}
```

start the loader:
```bash
name_loader -h db-mysql-dbdev0a.42 -user dbschema -pass ****
```


```js
db.NationalNames.createIndex( { "id": 1 }, { unique: true } )
```

The matching stream/estuary configuration:
```json
{"Type":"MONGO", "Host":  "db-mongo-replicator0a.42", "Port":27017, "Schema":"testings", "Collection":"NationalNames"}
```

Let's create the Index in elastic:  
```json
put names_index
{
    "settings": {
        "index": {
        "number_of_shards": "2",
        "number_of_replicas": "0"
        }
    },
    "mappings" : {
        "_doc" : {
        "properties" : {
            "id" : { "type" : "text" },
            "name" : { "type" : "text" },
            "year" : { "type" : "integer" },
            "gender" : { "type" : "text" },
            "count" : { "type" : "integer" }
        }
        }
    }
}
```

The matching estuary configuration:
```json
{"Type":"ELASTIC", "Host": "localhost", "Port":9200 , "Collection":"names_index"}
```