package main

import (
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/replicator"
)

func main() {

	config.LoadConfiguration()
	r := replicator.Replicator{}
	r.Config()
	r.Flow()
}
