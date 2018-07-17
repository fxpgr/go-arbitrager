package repositories

import (
	mgo "gopkg.in/mgo.v2"
	"github.com/fxpgr/go-arbitrager/models"
)

type HistoryRepositoryMgo struct {
	DB *mgo.Database `inject:""`
}

func (r *HistoryRepositoryMgo) Create(history *models.History) error {
	if err := r.DB.C("history").Insert(history); err != nil {
		return err
	}
	return nil
}
