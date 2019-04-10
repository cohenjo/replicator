# KAFKA Docker

To simplify tests we'll use Kafka in a docker with zookeeper:
we use this image from docker hub by spotify [spotify/kafka](https://hub.docker.com/r/spotify/kafka)

# Pre-Reqs:
install the sarma kafka go client tools - they mimic the ones from Kafka but without the cup of JAVA
```
go get github.com/Shopify/sarama/tools/...
```

## Run

```bash
# docker run -p 2181:2181 -p 9092:9092 --env ADVERTISED_HOST=localhost --env ADVERTISED_PORT=9092 spotify/kafka
task docker:start

export KAFKA_PEERS=localhost:9092
kafka-console-producer -topic=test -value=value
kafka-console-consumer -topic=test -offset=oldest
```
