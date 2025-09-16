package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/qntfy/kazaam/v4"
	"github.com/rs/zerolog/log"
)

// Engine represents the main transformation engine
type Engine struct {
	config     TransformationConfig
	rules      []TransformationRule
	ruleEngine RuleEngine
	metrics    *EngineMetrics
	mutex      sync.RWMutex
}

// EngineMetrics tracks transformation engine metrics
type EngineMetrics struct {
	TotalTransformations   int64                   `json:"total_transformations"`
	SuccessfulTransforms   int64                   `json:"successful_transforms"`
	FailedTransforms       int64                   `json:"failed_transforms"`
	SkippedTransforms      int64                   `json:"skipped_transforms"`
	AverageExecutionTime   time.Duration           `json:"average_execution_time"`
	MaxExecutionTime       time.Duration           `json:"max_execution_time"`
	MinExecutionTime       time.Duration           `json:"min_execution_time"`
	RuleMetrics            map[string]*RuleMetrics `json:"rule_metrics"`
	LastTransformationAt   *time.Time              `json:"last_transformation_at,omitempty"`
	mutex                  sync.RWMutex
}

// KazaamRuleEngine implements RuleEngine using Kazaam
type KazaamRuleEngine struct {
	transformers map[string]*kazaam.Kazaam
	mutex        sync.RWMutex
}

// NewEngine creates a new transformation engine
func NewEngine(config TransformationConfig) *Engine {
	return &Engine{
		config:     config,
		rules:      config.Rules,
		ruleEngine: NewKazaamRuleEngine(),
		metrics:    NewEngineMetrics(),
	}
}

// NewEngineMetrics creates new engine metrics
func NewEngineMetrics() *EngineMetrics {
	return &EngineMetrics{
		RuleMetrics: make(map[string]*RuleMetrics),
	}
}

// NewKazaamRuleEngine creates a new Kazaam-based rule engine
func NewKazaamRuleEngine() *KazaamRuleEngine {
	return &KazaamRuleEngine{
		transformers: make(map[string]*kazaam.Kazaam),
	}
}

// Transform applies transformations to input data
func (e *Engine) Transform(ctx context.Context, input map[string]interface{}) (*TransformationResult, error) {
	startTime := time.Now()
	result := &TransformationResult{
		Input:         input,
		Output:        input, // Start with input as default
		AppliedRules:  []string{},
		Errors:        []TransformationError{},
		Warnings:      []string{},
		Timestamp:     startTime,
		Metadata:      make(map[string]interface{}),
	}

	e.mutex.RLock()
	rules := make([]TransformationRule, len(e.rules))
	copy(rules, e.rules)
	e.mutex.RUnlock()

	// Sort rules by priority (lower number = higher priority)
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[i].Priority > rules[j].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}

	currentData := input
	allSuccess := true

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Check if conditions are met
		if len(rule.Conditions) > 0 {
			conditionsMet, err := e.ruleEngine.EvaluateConditions(ctx, currentData, rule.Conditions)
			if err != nil {
				transformErr := TransformationError{
					Rule:        rule.Name,
					ErrorType:   "condition_evaluation",
					Message:     fmt.Sprintf("Failed to evaluate conditions: %v", err),
					Timestamp:   time.Now(),
					Recoverable: true,
				}
				result.Errors = append(result.Errors, transformErr)
				
				if e.handleError(rule.ErrorHandling, transformErr) != nil {
					allSuccess = false
					continue
				}
			}
			if !conditionsMet {
				continue
			}
		}

		// Apply actions
		ruleSuccess := true
		for _, action := range rule.Actions {
			transformedData, err := e.ruleEngine.ExecuteAction(ctx, currentData, action)
			if err != nil {
				transformErr := TransformationError{
					Rule:        rule.Name,
					Action:      action.Type,
					ErrorType:   "action_execution",
					Message:     fmt.Sprintf("Failed to execute action: %v", err),
					Timestamp:   time.Now(),
					Recoverable: true,
				}
				result.Errors = append(result.Errors, transformErr)
				
				if e.handleError(rule.ErrorHandling, transformErr) != nil {
					ruleSuccess = false
					allSuccess = false
					break
				}
			} else {
				currentData = transformedData
			}
		}

		if ruleSuccess {
			result.AppliedRules = append(result.AppliedRules, rule.Name)
		}

		// Update rule metrics
		e.updateRuleMetrics(rule.Name, ruleSuccess, time.Since(startTime))
	}

	result.Output = currentData
	result.Success = allSuccess && len(result.Errors) == 0
	result.ExecutionTime = time.Since(startTime)

	// Update overall metrics
	e.updateMetrics(result.Success, result.ExecutionTime)

	return result, nil
}

// TransformBatch applies transformations to a batch of input data
func (e *Engine) TransformBatch(ctx context.Context, inputs []map[string]interface{}) ([]TransformationResult, error) {
	results := make([]TransformationResult, len(inputs))
	
	for i, input := range inputs {
		result, err := e.Transform(ctx, input)
		if err != nil {
			results[i] = TransformationResult{
				Success:   false,
				Input:     input,
				Output:    input,
				Errors:    []TransformationError{{
					ErrorType:   "batch_transform",
					Message:     err.Error(),
					Timestamp:   time.Now(),
					Recoverable: true,
				}},
				ExecutionTime: 0,
				Timestamp:     time.Now(),
			}
		} else {
			results[i] = *result
		}
	}
	
	return results, nil
}

// ValidateRules validates transformation rules
func (e *Engine) ValidateRules(rules []TransformationRule) error {
	for _, rule := range rules {
		if err := rule.Validate(); err != nil {
			return fmt.Errorf("rule '%s' validation failed: %w", rule.Name, err)
		}
		
		// Validate each action with the rule engine
		for _, action := range rule.Actions {
			if err := e.ruleEngine.ValidateAction(action); err != nil {
				return fmt.Errorf("action validation failed in rule '%s': %w", rule.Name, err)
			}
		}
	}
	return nil
}

// AddRule adds a transformation rule
func (e *Engine) AddRule(rule TransformationRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	// Check if rule already exists
	for _, existingRule := range e.rules {
		if existingRule.Name == rule.Name {
			return ErrRuleAlreadyExists
		}
	}

	e.rules = append(e.rules, rule)
	return nil
}

// RemoveRule removes a transformation rule by name
func (e *Engine) RemoveRule(name string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for i, rule := range e.rules {
		if rule.Name == name {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			
			// Clean up rule metrics
			e.metrics.mutex.Lock()
			delete(e.metrics.RuleMetrics, name)
			e.metrics.mutex.Unlock()
			
			return nil
		}
	}
	return ErrRuleNotFound
}

// GetRules returns all transformation rules
func (e *Engine) GetRules() []TransformationRule {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	
	rules := make([]TransformationRule, len(e.rules))
	copy(rules, e.rules)
	return rules
}

// GetMetrics returns transformation metrics
func (e *Engine) GetMetrics() TransformationMetrics {
	e.metrics.mutex.RLock()
	defer e.metrics.mutex.RUnlock()

	// Calculate success rate
	successRate := float64(0)
	if e.metrics.TotalTransformations > 0 {
		successRate = float64(e.metrics.SuccessfulTransforms) / float64(e.metrics.TotalTransformations)
	}

	// Copy rule metrics
	ruleMetrics := make(map[string]RuleMetrics)
	for name, metrics := range e.metrics.RuleMetrics {
		ruleMetrics[name] = *metrics
	}

	return TransformationMetrics{
		TotalTransformations:   e.metrics.TotalTransformations,
		SuccessfulTransforms:   e.metrics.SuccessfulTransforms,
		FailedTransforms:       e.metrics.FailedTransforms,
		SkippedTransforms:      e.metrics.SkippedTransforms,
		SuccessRate:            successRate,
		AverageExecutionTime:   e.metrics.AverageExecutionTime,
		MaxExecutionTime:       e.metrics.MaxExecutionTime,
		MinExecutionTime:       e.metrics.MinExecutionTime,
		RuleMetrics:            ruleMetrics,
		LastTransformationAt:   e.metrics.LastTransformationAt,
	}
}

// ResetMetrics resets all metrics
func (e *Engine) ResetMetrics() error {
	e.metrics.mutex.Lock()
	defer e.metrics.mutex.Unlock()

	e.metrics.TotalTransformations = 0
	e.metrics.SuccessfulTransforms = 0
	e.metrics.FailedTransforms = 0
	e.metrics.SkippedTransforms = 0
	e.metrics.AverageExecutionTime = 0
	e.metrics.MaxExecutionTime = 0
	e.metrics.MinExecutionTime = 0
	e.metrics.LastTransformationAt = nil
	e.metrics.RuleMetrics = make(map[string]*RuleMetrics)

	return nil
}

// EvaluateConditions evaluates whether conditions are met
func (re *KazaamRuleEngine) EvaluateConditions(ctx context.Context, data map[string]interface{}, conditions []Condition) (bool, error) {
	for _, condition := range conditions {
		met, err := re.evaluateCondition(data, condition)
		if err != nil {
			return false, fmt.Errorf("condition evaluation failed: %w", err)
		}
		if !met {
			return false, nil
		}
	}
	return true, nil
}

// ExecuteAction executes a transformation action
func (re *KazaamRuleEngine) ExecuteAction(ctx context.Context, data map[string]interface{}, action Action) (map[string]interface{}, error) {
	switch strings.ToLower(action.Type) {
	case "kazaam":
		return re.executeKazaamAction(data, action)
	default:
		return nil, fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

// ValidateCondition validates a condition
func (re *KazaamRuleEngine) ValidateCondition(condition Condition) error {
	return condition.Validate()
}

// ValidateAction validates an action
func (re *KazaamRuleEngine) ValidateAction(action Action) error {
	if err := action.Validate(); err != nil {
		return err
	}

	switch strings.ToLower(action.Type) {
	case "kazaam":
		// Validate Kazaam spec by creating a transformer
		_, err := kazaam.NewKazaam(action.Spec)
		return err
	default:
		return fmt.Errorf("unsupported action type: %s", action.Type)
	}
}

// executeKazaamAction executes a Kazaam transformation
func (re *KazaamRuleEngine) executeKazaamAction(data map[string]interface{}, action Action) (map[string]interface{}, error) {
	// Get or create Kazaam transformer
	transformer, err := re.getKazaamTransformer(action.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to get Kazaam transformer: %w", err)
	}

	// Convert data to JSON
	inputJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input data: %w", err)
	}

	// Apply transformation
	outputJSON, err := transformer.Transform(inputJSON)
	if err != nil {
		return nil, fmt.Errorf("Kazaam transformation failed: %w", err)
	}

	// Convert back to map
	var result map[string]interface{}
	if err := json.Unmarshal(outputJSON, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal output data: %w", err)
	}

	return result, nil
}

// getKazaamTransformer gets or creates a Kazaam transformer for the given spec
func (re *KazaamRuleEngine) getKazaamTransformer(spec string) (*kazaam.Kazaam, error) {
	re.mutex.RLock()
	transformer, exists := re.transformers[spec]
	re.mutex.RUnlock()

	if exists {
		return transformer, nil
	}

	// Create new transformer
	newTransformer, err := kazaam.NewKazaam(spec)
	if err != nil {
		return nil, err
	}

	re.mutex.Lock()
	re.transformers[spec] = newTransformer
	re.mutex.Unlock()

	return newTransformer, nil
}

// evaluateCondition evaluates a single condition
func (re *KazaamRuleEngine) evaluateCondition(data map[string]interface{}, condition Condition) (bool, error) {
	// Get field value using dot notation
	value, err := getFieldValue(data, condition.Field)
	if err != nil {
		if condition.Operator == "exists" {
			return false, nil
		}
		return false, err
	}

	switch condition.Operator {
	case "exists":
		return value != nil, nil
	case "eq":
		return compareValues(value, condition.Value, "eq"), nil
	case "ne":
		return compareValues(value, condition.Value, "ne"), nil
	case "gt":
		return compareValues(value, condition.Value, "gt"), nil
	case "lt":
		return compareValues(value, condition.Value, "lt"), nil
	case "contains":
		return containsValue(value, condition.Value), nil
	default:
		return false, fmt.Errorf("unsupported operator: %s", condition.Operator)
	}
}

// Helper functions for condition evaluation and field access
func getFieldValue(data map[string]interface{}, fieldPath string) (interface{}, error) {
	parts := strings.Split(fieldPath, ".")
	current := data

	for i, part := range parts {
		if current == nil {
			return nil, fmt.Errorf("field path '%s' not found", fieldPath)
		}

		if i == len(parts)-1 {
			value, exists := current[part]
			if !exists {
				return nil, fmt.Errorf("field '%s' not found", part)
			}
			return value, nil
		}

		next, exists := current[part]
		if !exists {
			return nil, fmt.Errorf("field path '%s' not found", fieldPath)
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field '%s' is not a map", part)
		}
		current = nextMap
	}

	return current, nil
}

func compareValues(a, b interface{}, operator string) bool {
	// Handle nil values
	if a == nil || b == nil {
		switch operator {
		case "eq":
			return a == b
		case "ne":
			return a != b
		default:
			return false
		}
	}

	// Type-specific comparisons would go here
	// For simplicity, using string representation
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	switch operator {
	case "eq":
		return aStr == bStr
	case "ne":
		return aStr != bStr
	case "gt":
		return aStr > bStr
	case "lt":
		return aStr < bStr
	default:
		return false
	}
}

func containsValue(haystack, needle interface{}) bool {
	haystackStr := fmt.Sprintf("%v", haystack)
	needleStr := fmt.Sprintf("%v", needle)
	return strings.Contains(haystackStr, needleStr)
}

// Helper methods for metrics updates
func (e *Engine) updateMetrics(success bool, duration time.Duration) {
	e.metrics.mutex.Lock()
	defer e.metrics.mutex.Unlock()

	e.metrics.TotalTransformations++
	
	if success {
		e.metrics.SuccessfulTransforms++
	} else {
		e.metrics.FailedTransforms++
	}

	// Update timing metrics
	if e.metrics.TotalTransformations == 1 {
		e.metrics.AverageExecutionTime = duration
		e.metrics.MaxExecutionTime = duration
		e.metrics.MinExecutionTime = duration
	} else {
		// Update average
		total := e.metrics.AverageExecutionTime * time.Duration(e.metrics.TotalTransformations-1)
		e.metrics.AverageExecutionTime = (total + duration) / time.Duration(e.metrics.TotalTransformations)
		
		// Update max/min
		if duration > e.metrics.MaxExecutionTime {
			e.metrics.MaxExecutionTime = duration
		}
		if duration < e.metrics.MinExecutionTime {
			e.metrics.MinExecutionTime = duration
		}
	}

	now := time.Now()
	e.metrics.LastTransformationAt = &now
}

func (e *Engine) updateRuleMetrics(ruleName string, success bool, duration time.Duration) {
	e.metrics.mutex.Lock()
	defer e.metrics.mutex.Unlock()

	metrics, exists := e.metrics.RuleMetrics[ruleName]
	if !exists {
		metrics = &RuleMetrics{Name: ruleName}
		e.metrics.RuleMetrics[ruleName] = metrics
	}

	metrics.Executions++
	if success {
		metrics.Successes++
	} else {
		metrics.Failures++
	}

	// Update average time
	if metrics.Executions == 1 {
		metrics.AverageTime = duration
	} else {
		total := metrics.AverageTime * time.Duration(metrics.Executions-1)
		metrics.AverageTime = (total + duration) / time.Duration(metrics.Executions)
	}

	now := time.Now()
	metrics.LastExecutionAt = &now
}

func (e *Engine) handleError(policy ErrorHandlingPolicy, err TransformationError) error {
	if policy.LogErrors {
		log.Error().
			Str("rule", err.Rule).
			Str("action", err.Action).
			Str("error_type", err.ErrorType).
			Str("message", err.Message).
			Bool("recoverable", err.Recoverable).
			Msg("Transformation error")
	}

	switch policy.Strategy {
	case ErrorStrategyFailFast:
		return fmt.Errorf("transformation failed: %s", err.Message)
	case ErrorStrategySkip:
		return nil // Continue processing
	case ErrorStrategyContinue:
		return nil // Continue processing
	default:
		return nil
	}
}