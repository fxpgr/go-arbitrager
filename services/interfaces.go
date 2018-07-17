package services

import "github.com/fxpgr/go-arbitrager/models"

//go:generate mockery -name=Exchange
type HistoryRepository interface {
	Create(history *models.History) (error)
}