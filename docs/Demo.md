# Demo

## MySQL Setup

```sql
(db-mysql-dbdev0a.42):[test]> create table users(id binary(16) PRIMARY KEY, name varchar(200));
Query OK, 0 rows affected (0.01 sec)

(db-mysql-dbdev0a.42):[test]> insert into users values(unhex(replace(uuid(),'-','')), 'Andromeda');
Query OK, 1 row affected (0.00 sec)

(db-mysql-dbdev0a.42):[test]> select hex(id),name from users;
+----------------------------------+-----------+
| hex(id)                          | name      |
+----------------------------------+-----------+
| 2C7016D75EB111E98C24127733938304 | Andromeda |
+----------------------------------+-----------+
1 row in set (0.00 sec)
```

The matching stream/estuary configuration:
```json
{"Type":"MYSQL","Host": "db-mysql-dbdev0a.42", "Port":3306, "Schema":"test","Collection":"users"}
```


```js
db.usernames.createIndex( { "id": 1 }, { unique: true } )
```

The matching stream/estuary configuration:
```json
{"Type":"MONGO", "Host":  "db-mongo-replicator0a.42", "Port":27017, "Schema":"testings", "Collection":"usernames"}
```

Let's create the Index in elastic:  
```json
put users
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
            "name" : { "type" : "text" }
        }
        }
    }
}
```

The matching estuary configuration:
```json
{"Type":"ELASTIC", "Host": "localhost", "Port":9200 , "Collection":"users"}
```