package estuary

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cohenjo/replicator/pkg/auth"
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type MongoEndpoint struct {
	db             string
	collectionName string
	client         *mongo.Client
	collection     *mongo.Collection
}

func NewMongoEndpoint(streamConfig *config.WaterFlowsConfig) (endpoint MongoEndpoint) {
	// Set timeout context
	// ctx, cancel :=  context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()
	ctx := context.Background()
	
	// Log configuration for debugging
	logger.Debug().
		Str("mongo_uri", streamConfig.MongoURI).
		Str("host", streamConfig.Host).
		Int("port", streamConfig.Port).
		Str("auth_method", streamConfig.MongoAuthMethod).
		Msg("Creating MongoDB endpoint with configuration")
	
	// Prepare auth config
	authConfig := &auth.MongoAuthConfig{}
	
	// Use MongoURI if provided, otherwise build from individual components
	if streamConfig.MongoURI != "" {
		authConfig.ConnectionURI = streamConfig.MongoURI
		logger.Debug().Str("uri", authConfig.ConnectionURI).Msg("Using provided MongoDB URI")
	} else if streamConfig.Host != "" && streamConfig.Port > 0 {
		// Build connection string from host/port - only use global config if available
		if config.Global != nil && config.Global.DBUser != "" && config.Global.DBPasswd != "" {
			authConfig.ConnectionURI = fmt.Sprintf("mongodb://%s:%s@%s:%d/admin", 
				config.Global.DBUser, 
				config.Global.DBPasswd, 
				streamConfig.Host, 
				streamConfig.Port)
			logger.Debug().Str("host", streamConfig.Host).Int("port", streamConfig.Port).Msg("Using host/port with global credentials")
		} else {
			authConfig.ConnectionURI = fmt.Sprintf("mongodb://%s:%d/admin", 
				streamConfig.Host, 
				streamConfig.Port)
			logger.Debug().Str("host", streamConfig.Host).Int("port", streamConfig.Port).Msg("Using host/port without credentials")
		}
	} else {
		logger.Error().
			Str("mongo_uri", streamConfig.MongoURI).
			Str("host", streamConfig.Host).
			Int("port", streamConfig.Port).
			Msg("MongoDB endpoint requires either MongoURI or Host/Port configuration")
		panic("MongoDB endpoint configuration is invalid: missing URI or host/port")
	}
	
	// Set authentication method from config, default to connection string
	authConfig.AuthMethod = "connection_string"
	if streamConfig.MongoAuthMethod != "" {
		authConfig.AuthMethod = streamConfig.MongoAuthMethod
		logger.Debug().Str("auth_method", authConfig.AuthMethod).Msg("Using specified authentication method")
	}
	
	// Set Entra authentication parameters if available
	authConfig.TenantID = streamConfig.MongoTenantID
	authConfig.ClientID = streamConfig.MongoClientID
	authConfig.Scopes = streamConfig.MongoScopes
	
	// Log Entra configuration if using Entra authentication
	if authConfig.AuthMethod == "entra" {
		logger.Debug().
			Str("tenant_id", authConfig.TenantID).
			Str("client_id", authConfig.ClientID).
			Strs("scopes", authConfig.Scopes).
			Msg("Configured Entra authentication")
	}
	
	// Parse refresh_before_expiry duration if provided
	if streamConfig.MongoRefreshBeforeExpiry != "" {
		if duration, err := time.ParseDuration(streamConfig.MongoRefreshBeforeExpiry); err == nil {
			authConfig.RefreshBeforeExpiry = duration
			logger.Debug().Dur("refresh_before_expiry", duration).Msg("Set token refresh timing")
		} else {
			logger.Warn().Err(err).Str("duration", streamConfig.MongoRefreshBeforeExpiry).Msg("Failed to parse refresh_before_expiry, using default")
		}
	}
	
	// Validate Entra configuration if using Entra authentication
	if authConfig.AuthMethod == "entra" {
		if len(authConfig.Scopes) == 0 {
			logger.Error().Msg("Entra authentication requires at least one scope")
			panic("Invalid Entra configuration: missing scopes")
		}
	}
	
	client, err := auth.NewMongoClientWithAuth(ctx, authConfig)
	if err != nil {
		logger.Error().Err(err).
			Str("uri", authConfig.ConnectionURI).
			Str("auth_method", authConfig.AuthMethod).
			Msg("connection failure")
		panic(fmt.Sprintf("Failed to connect to MongoDB: %v", err))
	}

	fmt.Println("Connected to MongoDB!")
	
	// Determine database and collection names
	dbName := streamConfig.Schema
	if streamConfig.MongoDatabaseName != "" {
		dbName = streamConfig.MongoDatabaseName
	}
	
	collectionName := streamConfig.Collection
	if streamConfig.MongoCollectionName != "" {
		collectionName = streamConfig.MongoCollectionName
	}
	
	collection := client.Database(dbName).Collection(collectionName)
	return MongoEndpoint{
		db:             dbName,
		collectionName: collectionName,
		client:         client,
		collection:     collection,
	}
}

// convertExtendedJSON converts MongoDB Extended JSON format to native Go types
func convertExtendedJSON(data map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for key, value := range data {
		result[key] = convertValue(value)
	}
	
	return result
}

// convertValue recursively converts MongoDB Extended JSON values to native types
func convertValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		// Handle MongoDB Extended JSON types
		if oid, ok := v["$oid"]; ok {
			// Convert ObjectId
			if oidStr, isString := oid.(string); isString {
				if objectID, err := bson.ObjectIDFromHex(oidStr); err == nil {
					return objectID
				}
			}
			return v // Return as-is if conversion fails
		}
		
		if numberInt, ok := v["$numberInt"]; ok {
			// Convert NumberInt
			if numStr, isString := numberInt.(string); isString {
				if num, err := strconv.Atoi(numStr); err == nil {
					return num
				}
			}
			return v // Return as-is if conversion fails
		}
		
		if numberLong, ok := v["$numberLong"]; ok {
			// Convert NumberLong
			if numStr, isString := numberLong.(string); isString {
				if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
					return num
				}
			}
			return v // Return as-is if conversion fails
		}
		
		if numberDouble, ok := v["$numberDouble"]; ok {
			// Convert NumberDouble
			if numStr, isString := numberDouble.(string); isString {
				if num, err := strconv.ParseFloat(numStr, 64); err == nil {
					return num
				}
			}
			return v // Return as-is if conversion fails
		}
		
		if date, ok := v["$date"]; ok {
			// Convert Date
			if dateStr, isString := date.(string); isString {
				if parsedTime, err := time.Parse(time.RFC3339, dateStr); err == nil {
					return parsedTime
				}
			}
			return v // Return as-is if conversion fails
		}
		
		// Recursively convert nested objects
		return convertExtendedJSON(v)
		
	case []interface{}:
		// Recursively convert array elements
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertValue(item)
		}
		return result
		
	default:
		// Return primitive types as-is
		return v
	}
}

func (std MongoEndpoint) WriteEvent(record *events.RecordEvent) {

	// Guard against empty or nil Data to prevent JSON unmarshal errors
	if len(record.Data) == 0 {
		logger.Warn().
		Str("action", record.Action).
		Str("schema", record.Schema).
		Str("collection", record.Collection).
		Msg("Skipping event with empty Data field")
		return
	}

	row := make(map[string]interface{})
	err := ffjson.Unmarshal(record.Data, &row)
	if err != nil {
		logger.Error().Err(err).
		Str("action", record.Action).
		Str("schema", record.Schema).
		Str("collection", record.Collection).
		Int("data_size", len(record.Data)).
		Msgf("Error while unmarshal record")
		return
	}
	
	// Convert MongoDB Extended JSON to native types
	row = convertExtendedJSON(row)
	
	logger.Debug().Str("action", record.Action).Str("name", std.collection.Name()).Msgf("write event: %+v", row)

	switch record.Action {
	case "insert":
		// For inserts, we don't need to parse OldData since it's empty
		// The document already contains all necessary data including _id
		insertResult, err := std.collection.InsertOne(context.TODO(), row)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while inserting document")
			return
		}
		logger.Debug().Msgf("Inserted a single document: %s", insertResult.InsertedID)
		
	case "delete":
		// For deletes, we need the key from OldData to identify the document
		if len(record.OldData) == 0 {
			logger.Error().Msg("Delete operation requires OldData with document key")
			return
		}

		// Parse the document key directly as it contains _id
		var documentKey map[string]interface{}
		err = ffjson.Unmarshal(record.OldData, &documentKey)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Unmarshal document key for delete")
			return
		}

		// Convert Extended JSON format if needed
		documentKey = convertExtendedJSON(documentKey)

		filter, err := bson.Marshal(documentKey)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Marshal filter")
			return
		}

		deleteResult, err := std.collection.DeleteMany(context.TODO(), filter)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to delete record")
			return
		}
		logger.Debug().Int("DeletedCount", int(deleteResult.DeletedCount)).Msg("record deleted properly")
		

	case "update":
		// For updates, we need the key from OldData to identify the document
		if len(record.OldData) == 0 {
			logger.Error().Msg("Update operation requires OldData with document key")
			return
		}

		// Parse the document key directly as it contains _id
		var documentKey map[string]interface{}
		err = ffjson.Unmarshal(record.OldData, &documentKey)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Unmarshal document key for update")
			return
		}

		// Convert Extended JSON format if needed
		documentKey = convertExtendedJSON(documentKey)

		filter, err := bson.Marshal(documentKey)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Marshal filter")
			return
		}

		// Use $set operator for updates to properly update the document
		update := map[string]interface{}{
			"$set": row,
		}

		updateResult, err := std.collection.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			logger.Error().Err(err).Msgf("Failed to update record")
			return
		}
		logger.Debug().Int("MatchedCount", int(updateResult.MatchedCount)).Int("ModifiedCount", int(updateResult.ModifiedCount)).Msg("record Updated properly")
		
	default:
		logger.Error().Str("action", record.Action).Msg("Unknown action type")
	}
}
