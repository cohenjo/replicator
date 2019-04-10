package transform

import (
	"bytes"
	"testing"
)

func TestTransform(t *testing.T) {
	transformer := NewTransformer()
	transformer.RegisterOperation(Operation{
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
