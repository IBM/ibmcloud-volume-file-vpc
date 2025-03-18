package payload

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewJSONBodyProvider(t *testing.T) {
	payload := map[string]interface{}{}
	provider := NewJSONBodyProvider(payload)
	assert.Equal(t, "application/json", provider.ContentType())
	assert.NotNil(t, provider)
}

func TestJSONBodyProvider_Body(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	provider := NewJSONBodyProvider(payload)
	_, err := provider.Body()
	assert.NoError(t, err)
}

func TestNewMultipartFileBody(t *testing.T) {
	contents := bytes.NewReader([]byte("test contents"))
	body := NewMultipartFileBody("test_file", contents)
	assert.NotEqual(t, "", body.ContentType())
	assert.NotNil(t, body)
}

func TestMultipartFileBody_Body(t *testing.T) {
	contents := bytes.NewReader([]byte("test contents"))
	body := NewMultipartFileBody("test_file", contents)
	_, err := body.Body()
	assert.NoError(t, err)
}
