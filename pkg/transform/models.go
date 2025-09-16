package transform

import (
	"context"
	"time"
)

// TransformationRule represents a single transformation rule
type TransformationRule struct {
	Name         string                 `json:"name" yaml:"name"`
	Description  string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled      bool                   `json:"enabled" yaml:"enabled"`
	Priority     int                    `json:"priority" yaml:"priority"` // Lower number = higher priority
	Conditions   []Condition            `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Actions      []Action               `json:"actions" yaml:"actions"`
	ErrorHandling ErrorHandlingPolicy   `json:"error_handling" yaml:"error_handling"`
	Metadata     map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Condition represents a condition for applying a transformation
type Condition struct {
	Field    string      `json:"field" yaml:"field"`         // JSONPath or field name
	Operator string      `json:"operator" yaml:"operator"`   // eq, ne, gt, lt, contains, exists, etc.
	Value    interface{} `json:"value" yaml:"value"`         // Value to compare against
	Type     string      `json:"type,omitempty" yaml:"type,omitempty"` // string, number, boolean, null
}

// Action represents a transformation action
type Action struct {
	Type     string                 `json:"type" yaml:"type"`         // "kazaam", "jq", "lua", "javascript"
	Spec     string                 `json:"spec" yaml:"spec"`         // Transformation specification
	Target   string                 `json:"target,omitempty" yaml:"target,omitempty"` // Target field for output
	Config   map[string]interface{} `json:"config,omitempty" yaml:"config,omitempty"` // Action-specific config
}

// ErrorHandlingPolicy defines how errors should be handled during transformation
type ErrorHandlingPolicy struct {
	Strategy        ErrorStrategy `json:"strategy" yaml:"strategy"`                 // fail_fast, skip, retry, dead_letter
	MaxRetries      int           `json:"max_retries,omitempty" yaml:"max_retries,omitempty"`
	RetryDelay      time.Duration `json:"retry_delay,omitempty" yaml:"retry_delay,omitempty"`
	DeadLetterTopic string        `json:"dead_letter_topic,omitempty" yaml:"dead_letter_topic,omitempty"`
	LogErrors       bool          `json:"log_errors" yaml:"log_errors"`
	Metrics         bool          `json:"metrics" yaml:"metrics"`
}

// ErrorStrategy represents different error handling strategies
type ErrorStrategy string

const (
	ErrorStrategyFailFast   ErrorStrategy = "fail_fast"   // Stop processing on first error
	ErrorStrategySkip       ErrorStrategy = "skip"        // Skip failed transformations
	ErrorStrategyRetry      ErrorStrategy = "retry"       // Retry failed transformations
	ErrorStrategyDeadLetter ErrorStrategy = "dead_letter" // Send failed events to dead letter queue
	ErrorStrategyContinue   ErrorStrategy = "continue"    // Continue with partial/failed transformations
)

// TransformationConfig represents the configuration for transformation engine
type TransformationConfig struct {
	Engine        string                 `json:"engine" yaml:"engine"`                 // "kazaam", "jq", "lua", "javascript"
	Rules         []TransformationRule   `json:"rules" yaml:"rules"`
	GlobalConfig  map[string]interface{} `json:"global_config,omitempty" yaml:"global_config,omitempty"`
	ErrorHandling ErrorHandlingPolicy    `json:"error_handling" yaml:"error_handling"`
	Metrics       MetricsConfig          `json:"metrics" yaml:"metrics"`
	Validation    ValidationConfig       `json:"validation" yaml:"validation"`
}

// MetricsConfig represents configuration for transformation metrics
type MetricsConfig struct {
	Enabled           bool          `json:"enabled" yaml:"enabled"`
	CollectionInterval time.Duration `json:"collection_interval" yaml:"collection_interval"`
	DetailedMetrics   bool          `json:"detailed_metrics" yaml:"detailed_metrics"`
}

// ValidationConfig represents configuration for transformation validation
type ValidationConfig struct {
	ValidateInput  bool   `json:"validate_input" yaml:"validate_input"`
	ValidateOutput bool   `json:"validate_output" yaml:"validate_output"`
	InputSchema    string `json:"input_schema,omitempty" yaml:"input_schema,omitempty"`
	OutputSchema   string `json:"output_schema,omitempty" yaml:"output_schema,omitempty"`
}

// TransformationResult represents the result of a transformation operation
type TransformationResult struct {
	Success       bool                   `json:"success"`
	Input         map[string]interface{} `json:"input,omitempty"`
	Output        map[string]interface{} `json:"output,omitempty"`
	AppliedRules  []string               `json:"applied_rules,omitempty"`
	Errors        []TransformationError  `json:"errors,omitempty"`
	Warnings      []string               `json:"warnings,omitempty"`
	ExecutionTime time.Duration          `json:"execution_time"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// TransformationError represents an error that occurred during transformation
type TransformationError struct {
	Rule        string    `json:"rule,omitempty"`
	Action      string    `json:"action,omitempty"`
	Field       string    `json:"field,omitempty"`
	ErrorType   string    `json:"error_type"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
	Recoverable bool      `json:"recoverable"`
}

// TransformationMetrics represents metrics for transformation operations
type TransformationMetrics struct {
	TotalTransformations   int64         `json:"total_transformations"`
	SuccessfulTransforms   int64         `json:"successful_transforms"`
	FailedTransforms       int64         `json:"failed_transforms"`
	SkippedTransforms      int64         `json:"skipped_transforms"`
	SuccessRate            float64       `json:"success_rate"`
	AverageExecutionTime   time.Duration `json:"average_execution_time"`
	MaxExecutionTime       time.Duration `json:"max_execution_time"`
	MinExecutionTime       time.Duration `json:"min_execution_time"`
	BytesProcessed         int64         `json:"bytes_processed"`
	BytesGenerated         int64         `json:"bytes_generated"`
	RuleMetrics            map[string]RuleMetrics `json:"rule_metrics"`
	LastTransformationAt   *time.Time    `json:"last_transformation_at,omitempty"`
	ErrorMetrics           ErrorMetrics  `json:"error_metrics"`
}

// RuleMetrics represents metrics for a specific transformation rule
type RuleMetrics struct {
	Name              string        `json:"name"`
	Executions        int64         `json:"executions"`
	Successes         int64         `json:"successes"`
	Failures          int64         `json:"failures"`
	AverageTime       time.Duration `json:"average_time"`
	LastExecutionAt   *time.Time    `json:"last_execution_at,omitempty"`
}

// ErrorMetrics represents error-related metrics
type ErrorMetrics struct {
	TotalErrors        int64              `json:"total_errors"`
	ErrorsByType       map[string]int64   `json:"errors_by_type"`
	ErrorsByRule       map[string]int64   `json:"errors_by_rule"`
	RecoverableErrors  int64              `json:"recoverable_errors"`
	FatalErrors        int64              `json:"fatal_errors"`
	DeadLetterEvents   int64              `json:"dead_letter_events"`
	LastErrorAt        *time.Time         `json:"last_error_at,omitempty"`
}

// Transformer represents the main transformation engine interface
type Transformer interface {
	// Transform applies transformations to input data
	Transform(ctx context.Context, input map[string]interface{}) (*TransformationResult, error)
	
	// TransformBatch applies transformations to a batch of input data
	TransformBatch(ctx context.Context, inputs []map[string]interface{}) ([]TransformationResult, error)
	
	// ValidateRules validates transformation rules
	ValidateRules(rules []TransformationRule) error
	
	// AddRule adds a transformation rule
	AddRule(rule TransformationRule) error
	
	// RemoveRule removes a transformation rule by name
	RemoveRule(name string) error
	
	// GetRules returns all transformation rules
	GetRules() []TransformationRule
	
	// GetMetrics returns transformation metrics
	GetMetrics() TransformationMetrics
	
	// Reset resets all metrics
	ResetMetrics() error
}

// RuleEngine represents an interface for rule evaluation
type RuleEngine interface {
	// EvaluateConditions evaluates whether conditions are met
	EvaluateConditions(ctx context.Context, data map[string]interface{}, conditions []Condition) (bool, error)
	
	// ExecuteAction executes a transformation action
	ExecuteAction(ctx context.Context, data map[string]interface{}, action Action) (map[string]interface{}, error)
	
	// ValidateCondition validates a condition
	ValidateCondition(condition Condition) error
	
	// ValidateAction validates an action
	ValidateAction(action Action) error
}

// DefaultTransformationConfig returns a default transformation configuration
func DefaultTransformationConfig() TransformationConfig {
	return TransformationConfig{
		Engine: "kazaam",
		Rules:  []TransformationRule{},
		ErrorHandling: ErrorHandlingPolicy{
			Strategy:   ErrorStrategySkip,
			MaxRetries: 3,
			RetryDelay: 1 * time.Second,
			LogErrors:  true,
			Metrics:    true,
		},
		Metrics: MetricsConfig{
			Enabled:            true,
			CollectionInterval: 30 * time.Second,
			DetailedMetrics:    false,
		},
		Validation: ValidationConfig{
			ValidateInput:  false,
			ValidateOutput: false,
		},
	}
}

// DefaultErrorHandling returns a default error handling policy
func DefaultErrorHandling() ErrorHandlingPolicy {
	return ErrorHandlingPolicy{
		Strategy:   ErrorStrategySkip,
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		LogErrors:  true,
		Metrics:    true,
	}
}

// Validate validates a transformation rule
func (r *TransformationRule) Validate() error {
	if r.Name == "" {
		return ErrInvalidRuleName
	}
	
	if len(r.Actions) == 0 {
		return ErrNoActions
	}
	
	for _, condition := range r.Conditions {
		if err := condition.Validate(); err != nil {
			return err
		}
	}
	
	for _, action := range r.Actions {
		if err := action.Validate(); err != nil {
			return err
		}
	}
	
	return r.ErrorHandling.Validate()
}

// Validate validates a condition
func (c *Condition) Validate() error {
	if c.Field == "" {
		return ErrInvalidConditionField
	}
	
	if c.Operator == "" {
		return ErrInvalidConditionOperator
	}
	
	return nil
}

// Validate validates an action
func (a *Action) Validate() error {
	if a.Type == "" {
		return ErrInvalidActionType
	}
	
	if a.Spec == "" {
		return ErrInvalidActionSpec
	}
	
	return nil
}

// Validate validates an error handling policy
func (e *ErrorHandlingPolicy) Validate() error {
	switch e.Strategy {
	case ErrorStrategyFailFast, ErrorStrategySkip, ErrorStrategyRetry, ErrorStrategyDeadLetter, ErrorStrategyContinue:
		// Valid strategies
	default:
		return ErrInvalidErrorStrategy
	}
	
	if e.Strategy == ErrorStrategyRetry && e.MaxRetries <= 0 {
		return ErrInvalidMaxRetries
	}
	
	if e.Strategy == ErrorStrategyDeadLetter && e.DeadLetterTopic == "" {
		return ErrInvalidDeadLetterTopic
	}
	
	return nil
}