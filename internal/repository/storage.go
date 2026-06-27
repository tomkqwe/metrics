package repository

import models "github.com/tomkqwe/metrics/internal/model"

type Storage interface {
	UpdateGauge(name string, value models.Gauge)
	UpdateCounter(name string, value models.Counter)
}
