# Replicator
[![Go Report Card](https://goreportcard.com/badge/github.com/cohenjo/replicator)](https://goreportcard.com/report/github.com/cohenjo/replicator)
[![GoDoc](https://godoc.org/github.com/cohenjo/replicator?status.svg)](https://godoc.org/github.com/cohenjo/replicator)

![replicator logo](docs/replicator_long_logo.png)

## General
Replicator is a go package that replicates data between multiple data sources using change streams.
It can replicate data between any sources, including MySQL, MongoDB, Kafka, Elastic and others in the future.

Replicator uses MySQL [replication](https://github.com/siddontang/go-mysql#replication) to read a MySQL change stream as a replica. (including AWS RDS)  
Mongo includes [Change Streams](https://docs.mongodb.com/manual/changeStreams/#change-streams), so this is a cinch.
For Kafka, Replicator uses [sarama](https://github.com/Shopify/sarama). Kafka doesn't really have a change stream, but we use it as a bus to distribute change events across data centres. 
PG uses binary logs (WALS) to transfer replication, so that's technically feasible, but not yet implemented.  
AWS DynamoDB provides change stream API and official [AWS-SDK-go](https://github.com/aws/aws-sdk-go) and even [example code](https://github.com/aws/aws-sdk-go/blob/master/service/dynamodbstreams/examples_test.go)  


Once Replicator receives an event for a record change, such as insert, update, delete, we transform it using [kazaam](https://github.com/qntfy/kazaam) and propagate the change to the registered database endpoints.
We support field mapping, field filtering, and transformations. For example, you can change column names or field names during replication.

Metrics on input/output records are exposed using Prometheus.

## General Flow

![data flow](docs/replicator_flow.png)

## Getting started

### Quick Start
For complete step-by-step instructions to run Replicator locally with working examples, see the **[Local Setup Guide](docs/local-setup-guide.md)**.

The guide includes:
- ✅ **MySQL to Elasticsearch** - Real-time binlog replication with search indexing
- ✅ **MongoDB to MongoDB** - Change stream replication between MongoDB instances
- Docker setup, configuration, and testing procedures
- Troubleshooting and monitoring

### Installation
```bash
go get -u github.com/cohenjo/replicator
```

### Configuration
Generate a configuration file containing input streams, output estuaries, and the transformations you want to perform on the records.
You can define multiple input/output paths.
Note: transformations are done using kazaam, so features and limitations are those of kazaam.

The schema must exist before you start the replicator. Also, Replicator does not replicate schema change events.

You should have a unique ID named `id` 

## Azure Entra Authentication 🔒

Replicator now supports **Azure Entra authentication** for MongoDB Cosmos DB using workload identity! This provides enterprise-grade security without managing secrets.

### Features
- ✅ **Workload Identity**: No secrets in configuration  
- ✅ **Token Management**: Automatic refresh and caching
- ✅ **Scope Validation**: Prevents configuration mistakes
- ✅ **Backwards Compatible**: Existing connections unchanged

### Configuration Example
```yaml
streams:
  - name: "cosmos-stream"
    source:
      type: "mongodb"
      uri: "mongodb://cosmos-cluster.mongo.cosmos.azure.com:10255/"
      database: "production"
      options:
        auth_method: "entra"                                    # Enable Entra auth
        tenant_id: "12345678-1234-1234-1234-123456789012"     # Azure tenant
        client_id: "87654321-4321-4321-4321-210987654321"     # App registration
        scopes: ["https://cosmos.azure.com/.default"]          # Cosmos DB scope
        refresh_before_expiry: "5m"                            # Token refresh buffer
```

### Azure Setup Requirements
1. **AKS Cluster**: With workload identity enabled
2. **App Registration**: Azure Entra application with MongoDB permissions  
3. **Cosmos DB**: Azure Cosmos DB for MongoDB vCore with AAD authentication
4. **Role Assignment**: Cosmos DB Data Contributor role for the application

For complete setup instructions, see [MongoDB Entra Implementation Guide](docs/MONGODB_ENTRA_IMPLEMENTATION.md).


## Performance Status

Current implemenatation is rather "local" in design - it reads from the source streams, transforms and writes to endpoints.
If the deployment has remote endpoints it might be better to use a replicated kafka topic with [snappy](https://github.com/golang/snappy) or similar algorithem.


## Features

 - [x] MySQL - input/output
 - [x] MongoDB - input/output
 - [x] KAFKA - input/output
 - [x] ElasticSearch - output
 - [x] Metrics - expose metric of to prometheus
 - [ ] Support of all CRUD ops
 - [ ] Grafana Dashboard - extend dashboard
 - [ ] Load tool 
 - [ ] Demo with all functionality

## Alternatives

[gollum](https://github.com/trivago/gollum) - very robust system but lacking DB suport.  
[debezium](https://debezium.io) - currently more around trditional db systems (MySQL, Oracle, SQL Server, MongoDB and PostgreSQL)



## built using
- [go-mysql](https://github.com/siddontang/go-mysql)
- [sqlx](https://github.com/jmoiron/sqlx)
- [mongo go driver](https://github.com/mongodb/mongo-go-driver)
- [kazaam](https://github.com/qntfy/kazaam)
- [ffjson](https://github.com/pquerna/ffjson)
- [sarama](https://github.com/Shopify/sarama)
- [elasticsearch go driver](github.com/elastic/go-elasticsearch)
- [prometheus client](https://github.com/prometheus/client_golang/)

## License
`reflector` is licensed under MIT License. 
Some of the components used are Licensed under Apache License, Version 2.0
Please review before using in commercial environments.
