package replicator

import (
"context"
"encoding/json"
"fmt"

"github.com/cohenjo/replicator/pkg/config"
"github.com/cohenjo/replicator/pkg/estuary"
"github.com/cohenjo/replicator/pkg/events"
"github.com/rs/zerolog/log"
)

// EstuaryBridge adapts the legacy Endpoint interface to the new EstuaryWriter interface
type EstuaryBridge struct {
	endpoint estuary.Endpoint
	name     string
}

// NewEstuaryBridge creates a new bridge for the given target configuration
func NewEstuaryBridge(targetConfig config.TargetConfig) (*EstuaryBridge, error) {
	// Convert new config format to legacy WaterFlowsConfig format
	legacyConfig := &config.WaterFlowsConfig{
		Type:       string(targetConfig.Type), // Convert TargetType to string
		Host:       targetConfig.Host,
		Port:       targetConfig.Port,
		Collection: targetConfig.Database, // Use database as collection/index name
		Schema:     targetConfig.Database, // MongoDB needs schema field for database name
	}

	// For MongoDB, handle URI and authentication configuration
	if targetConfig.Type == config.TargetTypeMongoDB {
		// Use URI if provided, otherwise construct from host/port
		if targetConfig.URI != "" {
			legacyConfig.MongoURI = targetConfig.URI
		} else if targetConfig.Host != "" && targetConfig.Port > 0 {
			legacyConfig.MongoURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/admin?authSource=admin&directConnection=true",
				targetConfig.Username,
				targetConfig.Password,
				targetConfig.Host,
				targetConfig.Port,
			)
		} else {
			return nil, fmt.Errorf("MongoDB target requires either URI or host/port configuration")
		}
		
		legacyConfig.MongoDatabaseName = targetConfig.Database
		
		// Pass through authentication method and related options
		if targetConfig.Options != nil {
			if authMethod, ok := targetConfig.Options["auth_method"].(string); ok {
				legacyConfig.MongoAuthMethod = authMethod
			}
			if tenantID, ok := targetConfig.Options["tenant_id"].(string); ok {
				legacyConfig.MongoTenantID = tenantID
			}
			if clientID, ok := targetConfig.Options["client_id"].(string); ok {
				legacyConfig.MongoClientID = clientID
			}
			if scopes, ok := targetConfig.Options["scopes"].([]interface{}); ok {
				scopeStrings := make([]string, len(scopes))
				for i, scope := range scopes {
					if scopeStr, ok := scope.(string); ok {
						scopeStrings[i] = scopeStr
					}
				}
				legacyConfig.MongoScopes = scopeStrings
			}
			if refreshBefore, ok := targetConfig.Options["refresh_before_expiry"].(string); ok {
				legacyConfig.MongoRefreshBeforeExpiry = refreshBefore
			}
		}
	}

	// Override collection from options if specified
	if targetConfig.Options != nil {
		if collection, ok := targetConfig.Options["collection"].(string); ok && collection != "" {
			legacyConfig.Collection = collection
			if targetConfig.Type == config.TargetTypeMongoDB {
				legacyConfig.MongoCollectionName = collection
			}
		}
	}

	// Create the appropriate endpoint based on type
	var endpoint estuary.Endpoint
	switch targetConfig.Type {
	case config.TargetTypeElastic:
		endpoint = estuary.NewElasticEndpoint(legacyConfig)
	case config.TargetTypeMySQL:
		endpoint = estuary.NewMySQLEndpoint(legacyConfig)
	case config.TargetTypeMongoDB:
		endpoint = estuary.NewMongoEndpoint(legacyConfig)
	case config.TargetTypeKafka:
		endpoint = estuary.NewKafkaEndpoint(legacyConfig)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", targetConfig.Type)
	}

	return &EstuaryBridge{
		endpoint: endpoint,
		name:     fmt.Sprintf("%s-%s:%d", targetConfig.Type, targetConfig.Host, targetConfig.Port),
	}, nil
}

// WriteEvent implements the EstuaryWriter interface
func (eb *EstuaryBridge) WriteEvent(ctx context.Context, event map[string]interface{}) error {
log.Debug().Str("name", eb.name).Interface("event", event).Msg("EstuaryBridge.WriteEvent called")

// Convert the transformed event data back to RecordEvent format for legacy endpoint
recordEvent, err := eb.convertToRecordEvent(event)
if err != nil {
log.Error().Err(err).Str("name", eb.name).Msg("EstuaryBridge.WriteEvent failed to convert event")
return fmt.Errorf("failed to convert event: %w", err)
}

log.Debug().Str("name", eb.name).Interface("recordEvent", recordEvent).Msg("EstuaryBridge calling endpoint.WriteEvent")

// Call the legacy endpoint's WriteEvent method
eb.endpoint.WriteEvent(recordEvent)

log.Debug().Str("name", eb.name).Msg("EstuaryBridge.WriteEvent completed")
return nil
}

// Close implements the EstuaryWriter interface
func (eb *EstuaryBridge) Close() error {
	// Check if the endpoint has a Close method and call it
	if closer, ok := eb.endpoint.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// convertToRecordEvent converts the transformed event data back to RecordEvent format
func (eb *EstuaryBridge) convertToRecordEvent(event map[string]interface{}) (*events.RecordEvent, error) {
	// Extract fields from the transformed event
	action, _ := event["action"].(string)
	schema, _ := event["schema"].(string)
	collection, _ := event["collection"].(string)
	
	// Handle data field - could be raw JSON bytes or a map
	var dataBytes []byte
	var err error
	
	if data, exists := event["data"]; exists {
		switch d := data.(type) {
		case []byte:
			dataBytes = d
		case string:
			dataBytes = []byte(d)
		default:
			// Convert map/struct to JSON
			dataBytes, err = json.Marshal(d)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal data: %w", err)
			}
		}
	}

	// Handle old_data field
	var oldDataBytes []byte
	if oldData, exists := event["old_data"]; exists && oldData != nil {
		switch od := oldData.(type) {
		case []byte:
			oldDataBytes = od
		case string:
			oldDataBytes = []byte(od)
		default:
			oldDataBytes, err = json.Marshal(od)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal old_data: %w", err)
			}
		}
	}

	return &events.RecordEvent{
		Action:     action,
		Schema:     schema,
		Collection: collection,
		Data:       dataBytes,
		OldData:    oldDataBytes,
	}, nil
}

// String returns a string representation of the bridge
func (eb *EstuaryBridge) String() string {
return eb.name
}
