package estuary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"

	elasticsearch "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
)

type ElasticEndpoint struct {
	index string
	es    *elasticsearch.Client
}

func NewElasticEndpoint(streamConfig *config.WaterFlowsConfig) *ElasticEndpoint {
	cfg := elasticsearch.Config{
		Addresses: []string{fmt.Sprintf("http://%s:%d", streamConfig.Host, streamConfig.Port)},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   10,
			ResponseHeaderTimeout: 10 * time.Second,
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			// TLSClientConfig: &tls.Config{
			// 	MinVersion: tls.VersionTLS11,
			// 	// ...
			// },
		},
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create client :( ")
	}
	return &ElasticEndpoint{
		index: streamConfig.Collection,
		es:    es,
	}

}

func (ee *ElasticEndpoint) WriteEvent(record *events.RecordEvent) {

	// row := make(map[string]interface{})
	// err := ffjson.Unmarshal(record.Data, &row)
	// if err != nil {
	// 	logger.Error().Err(err).Msgf("Error unmarsheling record")
	// }
	var key events.RecordKey
	err := ffjson.Unmarshal(record.OldData, &key)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while Unmarshal key")
		return
	}

	var req esapi.Request
	switch record.Action {
	case "insert":
		req = esapi.IndexRequest{
			Index:      ee.index,
			DocumentID: key.ID,
			Body:       bytes.NewReader(record.Data),
			Refresh:    "true",
		}
	case "update":
		req = esapi.UpdateRequest{
			Index:      ee.index,
			DocumentID: key.ID,
			Body:       bytes.NewReader(record.Data),
			Refresh:    "true",
		}
	case "delete":
		req = esapi.DeleteRequest{
			Index:      ee.index,
			DocumentID: key.ID,
		}
	}
	// Perform the request with the client.
	res, err := req.Do(context.Background(), ee.es)
	if err != nil {
		logger.Error().Err(err).Msg("Error getting response")
	}
	defer res.Body.Close()

	if res.IsError() {
		logger.Info().Str("status", res.Status()).Msgf(" Error indexing document ID=%s", key)
	} else {
		// Deserialize the response into a map.
		var r map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			logger.Error().Err(err).Msg("Error parsing the response body")
		} else {
			// Print the response status and indexed document version.
			logger.Debug().Str("status", res.Status()).Msgf(" %s; version=%d", r["result"], int(r["_version"].(float64)))
		}
	}
}
