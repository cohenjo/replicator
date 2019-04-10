package transform

import (
	"github.com/pquerna/ffjson/ffjson"
	"github.com/rs/zerolog/log"

	kazaam "gopkg.in/qntfy/kazaam.v3"
)

type Operation struct {
	Operation string                 `json:"operation"`
	Spec      map[string]interface{} `json:"spec"`
}

type TransformationManager struct {
	operations []Operation
	k          *kazaam.Kazaam
}

func NewTransformer() *TransformationManager {
	return &TransformationManager{
		operations: make([]Operation, 0),
	}
}

// data could be: `{"input":"input value"}`
func (transformer *TransformationManager) Transform(data []byte) []byte {
	// log.Info().Msgf("configure kazam to use: %s", string(data))
	kazaamOut, err := transformer.k.Transform(data)
	if err != nil {
		log.Error().Err(err).Msgf("failed to kazam ")
	}
	// log.Info().Msgf("configure kazam to use: %s", string(kazaamOut))
	return kazaamOut
}

func (transformer *TransformationManager) RegisterOperation(op Operation) {
	transformer.operations = append(transformer.operations, op)
}

func (transformer *TransformationManager) InitializeTransformer() {
	str, err := ffjson.Marshal(transformer.operations)
	// log.Info().Msgf("configure kazam to use: %s", str)
	transformer.k, err = kazaam.New(string(str), kazaam.NewDefaultConfig())
	if err != nil {
		log.Error().Err(err).Msgf("failed to init kazaam ")
	}
}
