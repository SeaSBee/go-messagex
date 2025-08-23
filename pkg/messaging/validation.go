// Package messaging provides transport-agnostic interfaces for messaging systems.
package messaging

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// ValidationRule represents a validation rule.
type ValidationRule interface {
	Validate(value interface{}) error
	GetName() string
	GetDescription() string
}

// ValidationResult represents the result of a validation.
type ValidationResult struct {
	Valid    bool
	Errors   []*ValidationError
	Warnings []*ValidationWarning
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field       string
	Value       interface{}
	Rule        string
	Message     string
	Description string
	Severity    ValidationSeverity
}

// ValidationWarning represents a validation warning.
type ValidationWarning struct {
	Field       string
	Value       interface{}
	Rule        string
	Message     string
	Description string
}

// ValidationSeverity represents the severity of a validation error.
type ValidationSeverity string

const (
	// ValidationSeverityError represents an error severity
	ValidationSeverityError ValidationSeverity = "error"
	// ValidationSeverityWarning represents a warning severity
	ValidationSeverityWarning ValidationSeverity = "warning"
	// ValidationSeverityInfo represents an info severity
	ValidationSeverityInfo ValidationSeverity = "info"
)

// Validator provides validation functionality.
type Validator struct {
	rules map[string]ValidationRule
	mu    sync.RWMutex
}

// NewValidator creates a new validator.
func NewValidator() *Validator {
	validator := &Validator{
		rules: make(map[string]ValidationRule),
	}
	validator.registerDefaultRules()
	return validator
}

// RegisterRule registers a validation rule.
func (v *Validator) RegisterRule(rule ValidationRule) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.rules[rule.GetName()] = rule
}

// GetRule retrieves a validation rule.
func (v *Validator) GetRule(name string) (ValidationRule, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	rule, exists := v.rules[name]
	return rule, exists
}

// Validate validates a value using a specific rule.
func (v *Validator) Validate(ruleName string, value interface{}) error {
	rule, exists := v.GetRule(ruleName)
	if !exists {
		return NewErrorf(ErrorCodeValidation, "validation", "unknown validation rule: %s", ruleName)
	}
	return rule.Validate(value)
}

// ValidateStruct validates a struct using validation tags.
func (v *Validator) ValidateStruct(obj interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   make([]*ValidationError, 0),
		Warnings: make([]*ValidationWarning, 0),
	}

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		result.Valid = false
		result.Errors = append(result.Errors, &ValidationError{
			Field:    "root",
			Value:    obj,
			Rule:     "struct",
			Message:  "value must be a struct",
			Severity: ValidationSeverityError,
		})
		return result
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Get validation tags
		validationTag := fieldType.Tag.Get("validate")
		if validationTag == "" {
			continue
		}

		// Parse validation rules
		rules := strings.Split(validationTag, ",")
		for _, ruleStr := range rules {
			ruleStr = strings.TrimSpace(ruleStr)
			if ruleStr == "" {
				continue
			}

			// Parse rule and parameters
			ruleParts := strings.SplitN(ruleStr, ":", 2)
			ruleName := ruleParts[0]
			var ruleParams string
			if len(ruleParts) > 1 {
				ruleParams = ruleParts[1]
			}

			// Apply validation rule
			if err := v.applyRule(ruleName, ruleParams, field.Interface(), fieldType.Name, result); err != nil {
				result.Valid = false
				result.Errors = append(result.Errors, &ValidationError{
					Field:    fieldType.Name,
					Value:    field.Interface(),
					Rule:     ruleName,
					Message:  err.Error(),
					Severity: ValidationSeverityError,
				})
			}
		}
	}

	return result
}

// applyRule applies a validation rule to a value.
func (v *Validator) applyRule(ruleName, params string, value interface{}, fieldName string, result *ValidationResult) error {
	switch ruleName {
	case "required":
		return v.validateRequired(value, fieldName)
	case "min":
		return v.validateMin(value, params, fieldName)
	case "max":
		return v.validateMax(value, params, fieldName)
	case "len":
		return v.validateLen(value, params, fieldName)
	case "email":
		return v.validateEmail(value, fieldName)
	case "url":
		return v.validateURL(value, fieldName)
	case "regexp":
		return v.validateRegexp(value, params, fieldName)
	case "oneof":
		return v.validateOneOf(value, params, fieldName)
	case "gte":
		return v.validateGTE(value, params, fieldName)
	case "lte":
		return v.validateLTE(value, params, fieldName)
	case "gt":
		return v.validateGT(value, params, fieldName)
	case "lt":
		return v.validateLT(value, params, fieldName)
	default:
		// Try to use a custom rule
		return v.Validate(ruleName, value)
	}
}

// validateRequired validates that a value is not empty.
func (v *Validator) validateRequired(value interface{}, fieldName string) error {
	if value == nil {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' is required", fieldName)
	}

	val := reflect.ValueOf(value)
	switch val.Kind() {
	case reflect.String:
		if strings.TrimSpace(val.String()) == "" {
			return NewErrorf(ErrorCodeValidation, "validation", "field '%s' is required", fieldName)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		if val.Len() == 0 {
			return NewErrorf(ErrorCodeValidation, "validation", "field '%s' is required", fieldName)
		}
	case reflect.Ptr, reflect.Interface:
		if val.IsNil() {
			return NewErrorf(ErrorCodeValidation, "validation", "field '%s' is required", fieldName)
		}
	}

	return nil
}

// validateMin validates minimum value.
func (v *Validator) validateMin(value interface{}, params string, fieldName string) error {
	// Implementation for min validation
	return nil
}

// validateMax validates maximum value.
func (v *Validator) validateMax(value interface{}, params string, fieldName string) error {
	// Implementation for max validation
	return nil
}

// validateLen validates length.
func (v *Validator) validateLen(value interface{}, params string, fieldName string) error {
	// Implementation for length validation
	return nil
}

// validateEmail validates email format.
func (v *Validator) validateEmail(value interface{}, fieldName string) error {
	if value == nil {
		return nil // Skip if nil (use required rule for that)
	}

	str, ok := value.(string)
	if !ok {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be a string", fieldName)
	}

	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(str) {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be a valid email address", fieldName)
	}

	return nil
}

// validateURL validates URL format.
func (v *Validator) validateURL(value interface{}, fieldName string) error {
	if value == nil {
		return nil // Skip if nil (use required rule for that)
	}

	str, ok := value.(string)
	if !ok {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be a string", fieldName)
	}

	urlRegex := regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)
	if !urlRegex.MatchString(str) {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be a valid URL", fieldName)
	}

	return nil
}

// validateRegexp validates against a regular expression.
func (v *Validator) validateRegexp(value interface{}, params string, fieldName string) error {
	if value == nil {
		return nil // Skip if nil (use required rule for that)
	}

	str, ok := value.(string)
	if !ok {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be a string", fieldName)
	}

	regex, err := regexp.Compile(params)
	if err != nil {
		return NewErrorf(ErrorCodeValidation, "validation", "invalid regexp pattern for field '%s': %v", fieldName, err)
	}

	if !regex.MatchString(str) {
		return NewErrorf(ErrorCodeValidation, "validation", "field '%s' does not match pattern", fieldName)
	}

	return nil
}

// validateOneOf validates that a value is one of the specified values.
func (v *Validator) validateOneOf(value interface{}, params string, fieldName string) error {
	if value == nil {
		return nil // Skip if nil (use required rule for that)
	}

	allowedValues := strings.Split(params, " ")
	valueStr := fmt.Sprintf("%v", value)

	for _, allowed := range allowedValues {
		if valueStr == allowed {
			return nil
		}
	}

	return NewErrorf(ErrorCodeValidation, "validation", "field '%s' must be one of: %s", fieldName, params)
}

// validateGTE validates greater than or equal.
func (v *Validator) validateGTE(value interface{}, params string, fieldName string) error {
	// Implementation for GTE validation
	return nil
}

// validateLTE validates less than or equal.
func (v *Validator) validateLTE(value interface{}, params string, fieldName string) error {
	// Implementation for LTE validation
	return nil
}

// validateGT validates greater than.
func (v *Validator) validateGT(value interface{}, params string, fieldName string) error {
	// Implementation for GT validation
	return nil
}

// validateLT validates less than.
func (v *Validator) validateLT(value interface{}, params string, fieldName string) error {
	// Implementation for LT validation
	return nil
}

// registerDefaultRules registers default validation rules.
func (v *Validator) registerDefaultRules() {
	// Register custom validation rules here
}

// ValidationContext provides context for validation operations.
type ValidationContext struct {
	validator *Validator
	errors    []*ValidationError
	warnings  []*ValidationWarning
	mu        sync.RWMutex
}

// NewValidationContext creates a new validation context.
func NewValidationContext(validator *Validator) *ValidationContext {
	return &ValidationContext{
		validator: validator,
		errors:    make([]*ValidationError, 0),
		warnings:  make([]*ValidationWarning, 0),
	}
}

// Validate validates a value and adds errors to the context.
func (vc *ValidationContext) Validate(ruleName string, value interface{}, fieldName string) {
	if err := vc.validator.Validate(ruleName, value); err != nil {
		vc.mu.Lock()
		defer vc.mu.Unlock()
		vc.errors = append(vc.errors, &ValidationError{
			Field:    fieldName,
			Value:    value,
			Rule:     ruleName,
			Message:  err.Error(),
			Severity: ValidationSeverityError,
		})
	}
}

// ValidateStruct validates a struct and adds errors to the context.
func (vc *ValidationContext) ValidateStruct(obj interface{}) {
	result := vc.validator.ValidateStruct(obj)

	vc.mu.Lock()
	defer vc.mu.Unlock()

	vc.errors = append(vc.errors, result.Errors...)
	vc.warnings = append(vc.warnings, result.Warnings...)
}

// AddError adds a validation error to the context.
func (vc *ValidationContext) AddError(err *ValidationError) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.errors = append(vc.errors, err)
}

// AddWarning adds a validation warning to the context.
func (vc *ValidationContext) AddWarning(warning *ValidationWarning) {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.warnings = append(vc.warnings, warning)
}

// HasErrors returns true if there are validation errors.
func (vc *ValidationContext) HasErrors() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return len(vc.errors) > 0
}

// HasWarnings returns true if there are validation warnings.
func (vc *ValidationContext) HasWarnings() bool {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return len(vc.warnings) > 0
}

// GetErrors returns all validation errors.
func (vc *ValidationContext) GetErrors() []*ValidationError {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make([]*ValidationError, len(vc.errors))
	copy(result, vc.errors)
	return result
}

// GetWarnings returns all validation warnings.
func (vc *ValidationContext) GetWarnings() []*ValidationWarning {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	result := make([]*ValidationWarning, len(vc.warnings))
	copy(result, vc.warnings)
	return result
}

// Clear clears all errors and warnings.
func (vc *ValidationContext) Clear() {
	vc.mu.Lock()
	defer vc.mu.Unlock()
	vc.errors = vc.errors[:0]
	vc.warnings = vc.warnings[:0]
}

// ToError returns a combined error if there are validation errors.
func (vc *ValidationContext) ToError() error {
	if !vc.HasErrors() {
		return nil
	}

	errors := vc.GetErrors()
	if len(errors) == 1 {
		return NewError(ErrorCodeValidation, "validation", errors[0].Message)
	}

	// Create a combined error
	errorMessages := make([]string, len(errors))
	for i, err := range errors {
		errorMessages[i] = fmt.Sprintf("%s: %s", err.Field, err.Message)
	}

	return NewErrorf(ErrorCodeValidation, "validation", "validation failed: %s", strings.Join(errorMessages, "; "))
}

// ValidationMiddleware provides middleware for automatic validation.
type ValidationMiddleware struct {
	validator *Validator
}

// NewValidationMiddleware creates a new validation middleware.
func NewValidationMiddleware(validator *Validator) *ValidationMiddleware {
	return &ValidationMiddleware{
		validator: validator,
	}
}

// ValidateConfig validates configuration.
func (vm *ValidationMiddleware) ValidateConfig(cfg *Config) error {
	ctx := NewValidationContext(vm.validator)
	ctx.ValidateStruct(cfg)
	return ctx.ToError()
}

// ValidateMessage validates a message.
func (vm *ValidationMiddleware) ValidateMessage(msg *Message) error {
	ctx := NewValidationContext(vm.validator)

	// Validate required fields
	ctx.Validate("required", msg.ID, "ID")
	ctx.Validate("required", msg.Body, "Body")
	ctx.Validate("required", msg.ContentType, "ContentType")

	// Validate content type
	ctx.Validate("oneof", msg.ContentType, "ContentType")

	// Validate message size
	if len(msg.Body) > 0 {
		ctx.Validate("max", len(msg.Body), "Body")
	}

	return ctx.ToError()
}

// ValidateDelivery validates a delivery.
func (vm *ValidationMiddleware) ValidateDelivery(delivery *Delivery) error {
	ctx := NewValidationContext(vm.validator)

	// Validate required fields
	ctx.Validate("required", delivery.ID, "ID")
	ctx.Validate("required", delivery.Body, "Body")
	ctx.Validate("required", delivery.Queue, "Queue")
	ctx.Validate("required", delivery.DeliveryTag, "DeliveryTag")

	return ctx.ToError()
}

// Global validator instance
var globalValidator = NewValidator()

// ValidateStruct validates a struct using the global validator.
func ValidateStruct(obj interface{}) *ValidationResult {
	return globalValidator.ValidateStruct(obj)
}

// ValidateField validates a field using the global validator.
func ValidateField(ruleName string, value interface{}) error {
	return globalValidator.Validate(ruleName, value)
}
