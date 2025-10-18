package validation

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// 1.- FieldError represents a single validation failure for a struct field.
type FieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

// 1.- Errors aggregates field errors keyed by the JSON field name.
type Errors struct {
	Fields map[string][]FieldError `json:"fields"`
}

// 1.- Empty reports whether any validation failures were captured.
func (e Errors) Empty() bool {
	return len(e.Fields) == 0
}

// 1.- ToMap converts the error collection into the structure expected by respond.Error.
func (e Errors) ToMap() map[string]interface{} {
	if e.Empty() {
		return map[string]interface{}{}
	}
	return map[string]interface{}{"fields": e.Fields}
}

// 1.- Validator wraps go-playground/validator with JSON tag awareness and friendly messages.
type Validator struct {
	engine *validator.Validate
}

// 1.- New constructs a Validator configured to honour JSON struct tags for field names.
func New() (*Validator, error) {
	engine := validator.New(validator.WithRequiredStructEnabled())

	// 2.- Ensure validation errors surface JSON tag names rather than struct field names.
	engine.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})

	return &Validator{engine: engine}, nil
}

// 1.- ValidateStruct inspects the provided payload and returns structured field errors.
func (v *Validator) ValidateStruct(value interface{}) (Errors, error) {
	// 2.- Guard against programmer errors where the validator has not been constructed.
	if v == nil || v.engine == nil {
		return Errors{}, errors.New("validator is not initialized")
	}

	result := Errors{Fields: map[string][]FieldError{}}

	// 3.- Delegate to go-playground/validator for rule evaluation.
	err := v.engine.Struct(value)
	if err == nil {
		return result, nil
	}

        var invalidErr *validator.InvalidValidationError
        if errors.As(err, &invalidErr) {
                return result, invalidErr
        }

	// 4.- Translate each validation error into a JSON-friendly representation.
	for _, fieldErr := range err.(validator.ValidationErrors) {
		field := fieldErr.Field()
		translated := FieldError{
			Field:   field,
			Rule:    fieldErr.Tag(),
			Param:   fieldErr.Param(),
			Message: describeFailure(field, fieldErr),
		}
		result.Fields[field] = append(result.Fields[field], translated)
	}

	return result, nil
}

// 1.- describeFailure crafts a human-readable error message for the given rule violation.
func describeFailure(field string, err validator.FieldError) string {
	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, err.Param())
	default:
		return fmt.Sprintf("%s failed the %s validation", field, err.Tag())
	}
}
