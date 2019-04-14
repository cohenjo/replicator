package transform

import (
	"bytes"
	"testing"

	"github.com/cohenjo/replicator/pkg/config"
)

func TestTransform(t *testing.T) {
	transformer := NewTransformer()
	transformer.RegisterOperation(config.TransformOperation{
		Operation: "shift",
		Spec:      map[string]interface{}{"output": "input"},
	})
	transformer.InitializeTransformer()

	input := []byte(`{"input":"input value"}`)

	output := transformer.Transform(input)

	expected := []byte(`{"output":"input value"}`)

	if !bytes.Equal(output, expected) {
		t.Errorf("transformation failed, got: %s, expected: %s", string(output), string(expected))
	}

}

func TestMongoKeyTransform(t *testing.T) {
	transformer := NewTransformer()
	transformer.RegisterOperation(config.TransformOperation{
		Operation: "shift",
		Spec:      map[string]interface{}{"id": "_id"},
	})
	transformer.InitializeTransformer()

	input := []byte(`{"_id":"14.3"}`)

	output := transformer.Transform(input)

	expected := []byte(`{"id":"14.3"}`)

	if !bytes.Equal(output, expected) {
		t.Errorf("transformation failed, got: %s, expected: %s", string(output), string(expected))
	}

}
