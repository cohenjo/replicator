# Replicator
[![Go Report Card](https://goreportcard.com/badge/github.com/cohenjo/replicator)](https://goreportcard.com/report/github.com/cohenjo/replicator)
[![GoDoc](https://godoc.org/github.com/cohenjo/replicator?status.svg)](https://godoc.org/github.com/cohenjo/replicator)

## General
Replicator is a go package aimed to test out different options to take change stream off DB engines.
It's inspired by [canal](https://github.com/siddontang/go-mysql#canal) which provides a change stream off MySQL.

Mongo has [Change Streams](https://docs.mongodb.com/manual/changeStreams/#change-streams) - so this should be doable easily.  
For Kafka I use [sarama](https://github.com/Shopify/sarama) - Kafka doesn't really have a change stream, but we use it as a bus to distribute change events cross data-centers.   
PG also uses binary logs (WALS) to transfer replication - so that's also on the agenda.

Once we got an event for a record change (insert/update/delete) we transform it using [kazaam](https://github.com/qntfy/kazaam) and propogate the change to the registered endpoints.    

We support field mapping, field filtering, and maybe transformations.
This is based off [kazaam](https://github.com/qntfy/kazaam).

## Getting started

Generate a configuration file containing input streams, output estuaries and transformation to do on the records.
you can define as many input/output paths.
the transformation is using kazaam internally - so can do what kazaam can do...

```
go get -u github.com/cohenjo/replicator
```

## Performance Status

Current implemenatation is rather "local" in design - it reads from the source streams, transforms and writes to endpoints.
If the deployment has remote endpoints it might be better to use a replicated kafka topic with [snappy](https://github.com/golang/snappy) or similar algorithem.


## Features

 - [x] MySQL - input/output
 - [x] MongoDB - input/output
 - [x] KAFKA - input/output
 - [ ] ElasticSearch - output
 - [ ] Metrics - expose metric of input/output records

## Alternatives

[gollum](https://github.com/trivago/gollum) - very robust system but lacking DB suport.  
[debezium](https://debezium.io) - currently more around trditional db systems (MySQL, Oracle, SQL Server, MongoDB and PostgreSQL)



## built using
- [go-mysql](https://github.com/siddontang/go-mysql)
- [sqlx](https://github.com/jmoiron/sqlx)
- [mongo driver](https://github.com/mongodb/mongo-go-driver)
- [kazaam](https://github.com/qntfy/kazaam)
- [ffjson](https://github.com/pquerna/ffjson)
- [sarama](https://github.com/Shopify/sarama)

## License
`reflector` is licensed under MIT License
Some of the components used are Licensed under Apache License, Version 2.0
Please review before using in comurcial environments.