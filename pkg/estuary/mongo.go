package estuary

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/pquerna/ffjson/ffjson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/mgo.v2/bson"
)

type MongoEndpoint struct {
	db             string
	collectionName string
	client         *mongo.Client
	collection     *mongo.Collection
}

func NewMongoEndpoint(streamConfig *config.WaterFlowsConfig) (endpoint MongoEndpoint) {
	// Set client options with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Use MongoURI if provided, otherwise build from individual components
	var uri string
	if streamConfig.MongoURI != "" {
		uri = streamConfig.MongoURI
	} else {
		// Fallback to global config (legacy behavior)
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/admin", config.Global.DBUser, config.Global.DBPasswd, streamConfig.Host, streamConfig.Port)
	}
	
	fmt.Printf("[DEBUG] Connecting to MongoDB with URI: %s\n", uri)
	
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		logger.Error().Err(err).Msg("connection failure")
		panic(fmt.Sprintf("Failed to connect to MongoDB: %v", err))
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Msg("connection ping failure")
		panic(fmt.Sprintf("Failed to ping MongoDB: %v", err))
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
				if objectID, err := primitive.ObjectIDFromHex(oidStr); err == nil {
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

	row := make(map[string]interface{})
	err := ffjson.Unmarshal(record.Data, &row)
	if err != nil {
		logger.Error().Err(err).Msgf("Error while unmarshal record")
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
		
		var key events.RecordKey
		err = ffjson.Unmarshal(record.OldData, &key)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Unmarshal key for delete")
			return
		}

		filter, err := bson.Marshal(key)
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
		
		var key events.RecordKey
		err = ffjson.Unmarshal(record.OldData, &key)
		if err != nil {
			logger.Error().Err(err).Msgf("Error while Unmarshal key for update")
			return
		}

		filter, err := bson.Marshal(key)
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
