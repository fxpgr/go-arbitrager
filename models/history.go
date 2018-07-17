package models

import (
	"gopkg.in/mgo.v2/bson"
	"time"
)

type History struct {
	ID   bson.ObjectId `bson:"_id"`

	DateTime  time.Time
	SellSide string    `bson:"sell_side"`
	SellRate float64   `bson:"sell_rate"`
	BuySide string    `bson:"buy_side"`
	BuyRate float64   `bson:"buy_rate"`
	CurrencyPair string   `bson:"currency_pair"`
	Amount float64 `bson:"amount"`
}
