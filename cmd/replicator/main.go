package main

import (
	"github.com/cohenjo/replicator/pkg/streams"
)

func main() {
	streams.MongoStream()
	streams.MySQLStream()
}
