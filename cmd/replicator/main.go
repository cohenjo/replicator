package main

import (
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/metrics"
	"github.com/cohenjo/replicator/pkg/replicator"
)

func main() {

	config.LoadConfiguration()
	go metrics.SetupMetrics()
	r := replicator.Replicator{}
	r.Config()
	r.Flow()
}
