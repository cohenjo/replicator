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
	// Debug logging
	logger.Debug().Str("action", record.Action).Str("schema", record.Schema).Str("collection", record.Collection).Msg("Processing event for Elasticsearch")
	
	// For MySQL data, we receive raw column values as JSON array
	// We need to convert this to a structured JSON object
	var rowData []interface{}
	err := json.Unmarshal(record.Data, &rowData)
	if err != nil {
		logger.Error().Err(err).Msgf("Error unmarshaling MySQL row data")
		return
	}
	
	// Convert array to structured object - use generic approach
	// For now, we'll create a simple indexed structure until schema mapping is configurable
	var structuredData map[string]interface{}
	var documentID string
	
	// Generic approach: create an object with indexed field names
	structuredData = make(map[string]interface{})
	for i, value := range rowData {
		structuredData[fmt.Sprintf("field_%d", i)] = value
	}
	
	// Use the first column as document ID if available, otherwise generate one
	if len(rowData) > 0 {
		documentID = fmt.Sprintf("%v", rowData[0])
	} else {
		documentID = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	
	// Convert structured data back to JSON for Elasticsearch
	structuredJSON, err := json.Marshal(structuredData)
	if err != nil {
		logger.Error().Err(err).Msgf("Error marshaling structured data")
		return
	}
	
	logger.Debug().Str("documentID", documentID).RawJSON("data", structuredJSON).Msg("Prepared data for Elasticsearch")

	// Ensure index exists (Elasticsearch will auto-create, but we can be explicit)
	ee.ensureIndexExists()
	
	var req esapi.Request
	switch record.Action {
	case "insert":
		req = esapi.IndexRequest{
			Index:      ee.index,
			DocumentID: documentID,
			Body:       bytes.NewReader(structuredJSON),
			Refresh:    "true",
		}
	case "update":
		// For updates, we'll use upsert to handle cases where document doesn't exist
		upsertBody := map[string]interface{}{
			"doc":           structuredData,
			"doc_as_upsert": true,
		}
		upsertJSON, _ := json.Marshal(upsertBody)
		req = esapi.UpdateRequest{
			Index:      ee.index,
			DocumentID: documentID,
			Body:       bytes.NewReader(upsertJSON),
			Refresh:    "true",
		}
	case "delete":
		req = esapi.DeleteRequest{
			Index:      ee.index,
			DocumentID: documentID,
			Refresh:    "true",
		}
	}
	// Perform the request with the client.
	res, err := req.Do(context.Background(), ee.es)
	if err != nil {
		logger.Error().Err(err).Msg("Error getting response")
		return
	}
	
	// Ensure we have a valid response before dereferencing
	if res == nil {
		logger.Error().Msg("Received nil response from Elasticsearch")
		return
	}
	defer res.Body.Close()

	if res.IsError() {
		logger.Error().Str("status", res.Status()).Str("documentID", documentID).Msg("Error indexing document")
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

// ensureIndexExists creates the index if it doesn't exist
func (ee *ElasticEndpoint) ensureIndexExists() {
	// Check if index exists
	req := esapi.IndicesExistsRequest{
		Index: []string{ee.index},
	}
	
	res, err := req.Do(context.Background(), ee.es)
	if err != nil {
		logger.Error().Err(err).Msg("Error checking if index exists")
		return
	}
	defer res.Body.Close()
	
	// If index doesn't exist (404), create it
	if res.StatusCode == 404 {
		logger.Info().Str("index", ee.index).Msg("Creating Elasticsearch index")
		
		// Create index with generic mapping (single-node setup)
		indexMapping := map[string]interface{}{
			"settings": map[string]interface{}{
				"number_of_shards":   1,
				"number_of_replicas": 0, // No replicas for single-node setup
			},
			"mappings": map[string]interface{}{
				"dynamic": true, // Allow dynamic field mapping
				"properties": map[string]interface{}{
					// Generic properties - Elasticsearch will auto-detect field types
				},
			},
		}
		
		mappingJSON, _ := json.Marshal(indexMapping)
		
		createReq := esapi.IndicesCreateRequest{
			Index: ee.index,
			Body:  bytes.NewReader(mappingJSON),
		}
		
		createRes, err := createReq.Do(context.Background(), ee.es)
		if err != nil {
			logger.Error().Err(err).Msg("Error creating index")
			return
		}
		defer createRes.Body.Close()
		
		if createRes.IsError() {
			logger.Error().Str("status", createRes.Status()).Msg("Error response when creating index")
		} else {
			logger.Info().Str("index", ee.index).Msg("Successfully created Elasticsearch index")
		}
	} else if res.StatusCode == 200 {
		logger.Debug().Str("index", ee.index).Msg("Index already exists")
	}
}
