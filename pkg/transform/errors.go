package transform

import "errors"

// Transformation error definitions
var (
	ErrInvalidRuleName          = errors.New("invalid rule name")
	ErrNoActions                = errors.New("no actions specified")
	ErrInvalidConditionField    = errors.New("invalid condition field")
	ErrInvalidConditionOperator = errors.New("invalid condition operator")
	ErrInvalidActionType        = errors.New("invalid action type")
	ErrInvalidActionSpec        = errors.New("invalid action specification")
	ErrInvalidErrorStrategy     = errors.New("invalid error handling strategy")
	ErrInvalidMaxRetries        = errors.New("invalid max retries value")
	ErrInvalidDeadLetterTopic   = errors.New("invalid dead letter topic")
	ErrTransformationFailed     = errors.New("transformation failed")
	ErrRuleNotFound             = errors.New("transformation rule not found")
	ErrRuleAlreadyExists        = errors.New("transformation rule already exists")
	ErrEngineNotSupported       = errors.New("transformation engine not supported")
	ErrInvalidInput             = errors.New("invalid input data")
	ErrInvalidOutput            = errors.New("invalid output data")
	ErrValidationFailed         = errors.New("validation failed")
	ErrConditionEvaluationFailed = errors.New("condition evaluation failed")
	ErrActionExecutionFailed    = errors.New("action execution failed")
)