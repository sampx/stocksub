package subscriber

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStructuredDataError(t *testing.T) {
	err := NewStructuredDataError(
		ErrInvalidFieldType,
		"price",
		"invalid value type",
	)

	assert.Contains(t, err.Error(), "INVALID_FIELD_TYPE")
	assert.Contains(t, err.Error(), "price")
	assert.Contains(t, err.Error(), "invalid value type")
	assert.Equal(t, ErrInvalidFieldType, err.Code)
	assert.Equal(t, "price", err.Field)
	assert.Equal(t, "invalid value type", err.Message)
}
