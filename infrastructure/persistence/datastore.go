package persistence

import (
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"gopkg.in/mgo.v2"
)

type HistoryRepositoryMgo struct {
	DB *mgo.Database `inject:""`
}

func (r *HistoryRepositoryMgo) Create(history *entity.History) error {
	if err := r.DB.C("history").Insert(history); err != nil {
		return err
	}
	return nil
}
