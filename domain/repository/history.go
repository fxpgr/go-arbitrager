package repository

import (
	"github.com/fxpgr/go-arbitrager/domain/entity"
)

//go:generate mockery -name=HistoryRepository
type HistoryRepository interface {
	Create(history *entity.History) error
}
