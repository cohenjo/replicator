package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/cohenjo/replicator/pkg/events"
	"github.com/sirupsen/logrus"
)

// CosmosDBStreamProvider implements the Stream interface for Azure Cosmos DB change feed
type CosmosDBStreamProvider struct {
	config         *CosmosDBConfig
	client         *azcosmos.Client
	container      *azcosmos.ContainerClient
	eventSender    chan<- events.RecordEvent
	stopChannel    chan struct{}
	logger         *logrus.Logger
	isRunning      bool
	continuationToken string
	pollInterval   time.Duration
	backoffFactor  float64
	maxBackoff     time.Duration
	retryAttempts  int
	maxRetries     int
}

// CosmosDBConfig holds Cosmos DB specific configuration
type CosmosDBConfig struct {
	// Cosmos DB connection settings
	Endpoint      string `json:"endpoint"`      // Cosmos DB account endpoint
	DatabaseName  string `json:"database"`     // Database name
	ContainerName string `json:"container"`    // Container name
	
	// Authentication settings (using Managed Identity)
	UseManagedIdentity bool `json:"use_managed_identity"` // Use Azure Managed Identity
	
	// Change feed settings
	StartFromBeginning bool          `json:"start_from_beginning"` // Start from beginning or now
	MaxItemCount       int           `json:"max_item_count"`       // Maximum items per page
	PollInterval       time.Duration `json:"poll_interval"`        // Polling interval for change feed
	
	// Filtering options
	IncludeOperations []string `json:"include_operations"` // Operations to include (create, replace, delete)
	ExcludeOperations []string `json:"exclude_operations"` // Operations to exclude
	
	// Performance settings
	MaxRetries    int           `json:"max_retries"`     // Maximum retry attempts
	RetryDelay    time.Duration `json:"retry_delay"`     // Initial retry delay
	MaxBackoff    time.Duration `json:"max_backoff"`     // Maximum backoff time
	RequestTimeout time.Duration `json:"request_timeout"` // Request timeout
}

// NewCosmosDBStreamProvider creates a new Cosmos DB stream provider
func NewCosmosDBStreamProvider(eventSender chan<- events.RecordEvent, logger *logrus.Logger) *CosmosDBStreamProvider {
	return &CosmosDBStreamProvider{
		eventSender:   eventSender,
		logger:        logger,
		stopChannel:   make(chan struct{}),
		pollInterval:  5 * time.Second,  // Default poll interval
		backoffFactor: 2.0,              // Exponential backoff factor
		maxBackoff:    5 * time.Minute,  // Maximum backoff time
		maxRetries:    5,                // Default max retries
	}
}

// Listen starts listening to Cosmos DB change feed
func (c *CosmosDBStreamProvider) Listen(ctx context.Context) error {
	c.logger.Info("Starting Cosmos DB stream provider")
	
	// Parse configuration from global config
	if err := c.parseCosmosDBConfig(); err != nil {
		return fmt.Errorf("failed to parse Cosmos DB configuration: %w", err)
	}
	
	// Connect to Cosmos DB
	if err := c.connect(ctx); err != nil {
		return fmt.Errorf("failed to connect to Cosmos DB: %w", err)
	}
	
	defer c.cleanup()
	
	c.isRunning = true
	c.retryAttempts = 0
	currentBackoff := c.config.RetryDelay
	
	c.logger.WithFields(logrus.Fields{
		"endpoint":   c.config.Endpoint,
		"database":   c.config.DatabaseName,
		"container":  c.config.ContainerName,
		"poll_interval": c.pollInterval,
	}).Info("Connected to Cosmos DB, starting change feed monitoring")
	
	// Main change feed polling loop
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping Cosmos DB stream provider")
			return ctx.Err()
		case <-c.stopChannel:
			c.logger.Info("Stop signal received, stopping Cosmos DB stream provider")
			return nil
		default:
			// Read change feed
			if err := c.readChangeFeed(ctx); err != nil {
				if c.isFatalError(err) {
					c.logger.WithError(err).Error("Fatal error reading Cosmos DB change feed")
					return err
				}
				
				// Handle retryable errors with exponential backoff
				c.retryAttempts++
				if c.retryAttempts > c.maxRetries {
					c.logger.WithError(err).Error("Maximum retry attempts exceeded")
					return fmt.Errorf("maximum retry attempts exceeded: %w", err)
				}
				
				c.logger.WithFields(logrus.Fields{
					"error": err.Error(),
					"retry_attempt": c.retryAttempts,
					"backoff_duration": currentBackoff,
				}).Warn("Error reading change feed, retrying with backoff")
				
				// Wait with exponential backoff
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(currentBackoff):
					currentBackoff = time.Duration(float64(currentBackoff) * c.backoffFactor)
					if currentBackoff > c.maxBackoff {
						currentBackoff = c.maxBackoff
					}
				}
				continue
			}
			
			// Reset retry counters on successful read
			c.retryAttempts = 0
			currentBackoff = c.config.RetryDelay
			
			// Wait for next poll interval
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-c.stopChannel:
				return nil
			case <-time.After(c.pollInterval):
				// Continue to next iteration
			}
		}
	}
}

// parseCosmosDBConfig parses Cosmos DB configuration from global config
func (c *CosmosDBStreamProvider) parseCosmosDBConfig() error {
	globalConfig := config.GetConfig()
	if globalConfig == nil {
		return fmt.Errorf("global configuration not available")
	}
	
	// Extract Cosmos DB configuration from global config
	cosmosConfig := &CosmosDBConfig{
		UseManagedIdentity: true, // Default to managed identity
		MaxItemCount:       100,  // Default page size
		PollInterval:       5 * time.Second,
		MaxRetries:         5,
		RetryDelay:         1 * time.Second,
		MaxBackoff:         5 * time.Minute,
		RequestTimeout:     30 * time.Second,
	}
	
	// Parse from WaterFlowsConfig if available
	if wfc := globalConfig.WaterFlowsConfig; wfc != nil {
		if wfc.CosmosEndpoint != "" {
			cosmosConfig.Endpoint = wfc.CosmosEndpoint
		}
		if wfc.CosmosDatabaseName != "" {
			cosmosConfig.DatabaseName = wfc.CosmosDatabaseName
		}
		if wfc.CosmosContainerName != "" {
			cosmosConfig.ContainerName = wfc.CosmosContainerName
		}
		if wfc.CosmosStartFromBeginning {
			cosmosConfig.StartFromBeginning = wfc.CosmosStartFromBeginning
		}
		if wfc.CosmosMaxItemCount > 0 {
			cosmosConfig.MaxItemCount = int(wfc.CosmosMaxItemCount)
		}
		if wfc.CosmosPollInterval > 0 {
			cosmosConfig.PollInterval = time.Duration(wfc.CosmosPollInterval) * time.Millisecond
			c.pollInterval = cosmosConfig.PollInterval
		}
		if len(wfc.CosmosIncludeOperations) > 0 {
			cosmosConfig.IncludeOperations = wfc.CosmosIncludeOperations
		}
		if len(wfc.CosmosExcludeOperations) > 0 {
			cosmosConfig.ExcludeOperations = wfc.CosmosExcludeOperations
		}
	}
	
	// Validate required fields
	if cosmosConfig.Endpoint == "" {
		return fmt.Errorf("cosmos DB endpoint is required")
	}
	if cosmosConfig.DatabaseName == "" {
		return fmt.Errorf("cosmos DB database name is required")
	}
	if cosmosConfig.ContainerName == "" {
		return fmt.Errorf("cosmos DB container name is required")
	}
	
	c.config = cosmosConfig
	c.maxRetries = cosmosConfig.MaxRetries
	c.maxBackoff = cosmosConfig.MaxBackoff
	
	c.logger.WithFields(logrus.Fields{
		"endpoint":   cosmosConfig.Endpoint,
		"database":   cosmosConfig.DatabaseName,
		"container":  cosmosConfig.ContainerName,
		"poll_interval": cosmosConfig.PollInterval,
		"max_item_count": cosmosConfig.MaxItemCount,
	}).Debug("Parsed Cosmos DB configuration")
	
	return nil
}

// connect establishes connection to Cosmos DB using managed identity
func (c *CosmosDBStreamProvider) connect(ctx context.Context) error {
	c.logger.Info("Connecting to Cosmos DB using managed identity")
	
	// Create Azure credential using managed identity
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}
	
	// Create Cosmos client options
	clientOptions := &azcosmos.ClientOptions{}
	
	// Create Cosmos DB client
	client, err := azcosmos.NewClient(c.config.Endpoint, cred, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to create Cosmos DB client: %w", err)
	}
	
	c.client = client
	
	// Get container client
	c.container, err = client.NewContainer(c.config.DatabaseName, c.config.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}
	
	// Test connection by getting container properties
	_, err = c.container.Read(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to read container properties (connection test failed): %w", err)
	}
	
	c.logger.WithFields(logrus.Fields{
		"endpoint":  c.config.Endpoint,
		"database":  c.config.DatabaseName,
		"container": c.config.ContainerName,
	}).Info("Successfully connected to Cosmos DB")
	
	return nil
}

// readChangeFeed reads and processes changes from Cosmos DB change feed
func (c *CosmosDBStreamProvider) readChangeFeed(ctx context.Context) error {
	// Cosmos DB change feed is implemented via SQL queries with special headers
	// We'll simulate a change feed by querying documents ordered by _ts (timestamp)
	
	query := "SELECT * FROM c ORDER BY c._ts"
	
	// Create query options
	opt := azcosmos.QueryOptions{}
	
	if c.continuationToken != "" {
		opt.ContinuationToken = &c.continuationToken
	}
	
	// Execute query
	queryPager := c.container.NewQueryItemsPager(query, azcosmos.PartitionKey{}, &opt)
	
	if !queryPager.More() {
		// No more results
		return nil
	}
	
	// Get next page
	response, err := queryPager.NextPage(ctx)
	if err != nil {
		return fmt.Errorf("failed to read change feed page: %w", err)
	}
	
	// Process items if any
	if len(response.Items) > 0 {
		for _, item := range response.Items {
			if err := c.processChangeItem(item); err != nil {
				c.logger.WithError(err).Error("Failed to process change item")
				// Continue processing other items
			}
		}
		
		c.logger.WithFields(logrus.Fields{
			"items_processed": len(response.Items),
		}).Debug("Processed change feed items")
	}
	
	// Update continuation token for next iteration
	if response.ContinuationToken != nil {
		c.continuationToken = *response.ContinuationToken
	}
	
	return nil
}

// processChangeItem processes a single change feed item
func (c *CosmosDBStreamProvider) processChangeItem(item []byte) error {
	// Parse the change item to determine operation type
	var document map[string]interface{}
	if err := json.Unmarshal(item, &document); err != nil {
		return fmt.Errorf("failed to parse change item: %w", err)
	}
	
	// Determine operation type based on document metadata
	operationType := c.determineOperationType(document)
	
	// Check if operation should be filtered
	if c.shouldFilterOperation(operationType) {
		c.logger.WithFields(logrus.Fields{
			"operation": operationType,
		}).Debug("Filtered out operation")
		return nil
	}
	
	// Marshal document to JSON for Data field
	docBytes, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("failed to marshal document: %w", err)
	}
	
	// Create record event
	recordEvent := events.RecordEvent{
		Action:     operationType,
		Schema:     c.config.DatabaseName,
		Collection: c.config.ContainerName,
		Data:       docBytes,
	}
	
	// Send event
	select {
	case c.eventSender <- recordEvent:
		c.logger.WithFields(logrus.Fields{
			"action":     recordEvent.Action,
			"schema":     recordEvent.Schema,
			"collection": recordEvent.Collection,
		}).Debug("Sent Cosmos DB change event")
	default:
		c.logger.Warn("Event channel is full, dropping change event")
	}
	
	return nil
}

// determineOperationType determines the type of operation from a Cosmos DB document
func (c *CosmosDBStreamProvider) determineOperationType(document map[string]interface{}) string {
	// Cosmos DB change feed doesn't explicitly indicate operation type
	// We can infer it from document metadata or use "upsert" as default
	// since Cosmos DB change feed primarily shows creates and updates
	
	// Check for _ts (timestamp) - newer documents are likely creates/updates
	if ts, ok := document["_ts"].(float64); ok {
		// If document is very recent, it's likely a create
		now := time.Now().Unix()
		if now-int64(ts) < 5 { // Within 5 seconds
			return "create"
		}
	}
	
	// Default to update for existing documents
	return "update"
}

// shouldFilterOperation checks if operation should be filtered based on configuration
func (c *CosmosDBStreamProvider) shouldFilterOperation(operation string) bool {
	// Check include filter
	if len(c.config.IncludeOperations) > 0 {
		included := false
		for _, op := range c.config.IncludeOperations {
			if strings.EqualFold(op, operation) {
				included = true
				break
			}
		}
		if !included {
			return true
		}
	}
	
	// Check exclude filter
	if len(c.config.ExcludeOperations) > 0 {
		for _, op := range c.config.ExcludeOperations {
			if strings.EqualFold(op, operation) {
				return true
			}
		}
	}
	
	return false
}

// isFatalError determines if an error is fatal and should stop the stream
func (c *CosmosDBStreamProvider) isFatalError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Check for authentication/authorization errors (fatal)
	fatalKeywords := []string{
		"unauthorized",
		"forbidden", 
		"authentication failed",
		"invalid credentials",
		"access denied",
		"resource not found",
		"database not found",
		"container not found",
	}
	
	for _, keyword := range fatalKeywords {
		if strings.Contains(strings.ToLower(errStr), keyword) {
			return true
		}
	}
	
	// Retryable errors (network, throttling, etc.)
	retryableKeywords := []string{
		"timeout",
		"connection",
		"network",
		"throttled",
		"rate limited",
		"too many requests",
		"service unavailable",
		"internal server error",
	}
	
	for _, keyword := range retryableKeywords {
		if strings.Contains(strings.ToLower(errStr), keyword) {
			return false
		}
	}
	
	// Default to retryable for unknown errors
	return false
}

// cleanup performs cleanup operations
func (c *CosmosDBStreamProvider) cleanup() {
	c.logger.Info("Cleaning up Cosmos DB stream provider")
	c.isRunning = false
	
	// Close client connections if needed
	// The Azure SDK handles connection cleanup automatically
}

// Stop stops the Cosmos DB stream provider
func (c *CosmosDBStreamProvider) Stop() {
	c.logger.Info("Cosmos DB stream provider stopped")
	if c.isRunning {
		close(c.stopChannel)
	}
}

// StreamType returns the type of stream
func (c *CosmosDBStreamProvider) StreamType() string {
	return "cosmosdb"
}