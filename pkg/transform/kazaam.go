package transform

import (
	"github.com/cohenjo/replicator/pkg/config"
	"github.com/pquerna/ffjson/ffjson"
	"github.com/rs/zerolog/log"

	kazaam "gopkg.in/qntfy/kazaam.v3"
)

type TransformationManager struct {
	operations []config.TransformOperation
	k          *kazaam.Kazaam
}

func NewTransformer() *TransformationManager {
	return &TransformationManager{
		operations: make([]config.TransformOperation, 0),
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

func (transformer *TransformationManager) RegisterOperation(op config.TransformOperation) {
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
