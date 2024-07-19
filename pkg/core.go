package micro

import "go.uber.org/zap"

func New(logger *zap.Logger) *CoreEntity {
	return &CoreEntity{
		logger: logger,
	}
}
