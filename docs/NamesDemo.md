# Demo

We'll do a simple demo to distribute names from MySQL to other data sources.
names taken from https://www.kaggle.com/kaggle/us-baby-names#NationalNames.csv

## Setup
As required by replicator we need to have our schema pre-defined.
we'll create the schema on the variouse data sources.

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

## Query
We'll Domenstrate the replication by quering over the different data sources.

### MySQL:
```sql
select hex(id),name,year,count from names where year=1880 and name='Elizabeth';
```

### Mongo:
```js
db.NationalNames.aggregate([ 
    { $match: {"year" : {$lt: 2000} } },  
    { $group: { _id: "$gender", total: { $sum: "$count" } } },  
    { $sort: { total: -1 } } 
    ])
```
Returns the number of males and females born before the year 2000  

### Elastic:
```json
GET names_index/_search
{
    "query": {
        "fuzzy" : {
            "name" : {
                "value": "Anne",
                "fuzziness": 2
            }
        }
    }
}
```
(will return Anna - only 1 typo - fuzzy search)

### Kafka:
```bash
./bin/kafka-console-consumer -topic db-replicator -brokers localhost:9092
```