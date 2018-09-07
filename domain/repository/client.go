package repository

import (
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/models"
)

type PublicResourceRepository interface {
	Volume(exchange string, trading string, settlement string) (float64, error)
	CurrencyPairs(exchange string) ([]models.CurrencyPair, error)
	Rate(exchange string, trading string, settlement string) (float64, error)
	RateMap(exchange string) (map[string]map[string]float64, error)
	VolumeMap(exchange string) (map[string]map[string]float64, error)
	FrozenCurrency(exchange string) ([]string, error)
	Board(exchange string, trading string, settlement string) (*models.Board, error)
}

type PrivateResourceRepository interface {
	TransferFee(exchange string) (map[string]float64, error)
	TradeFeeRates(exchange string) (map[string]map[string]private.TradeFee, error)
	TradeFeeRate(exchange string, trading string, settlement string) (private.TradeFee, error)
	Balances(exchange string) (map[string]float64, error)
	CompleteBalances(exchange string) (map[string]*models.Balance, error)
	ActiveOrders(exchange string) ([]*models.Order, error)
	IsOrderFilled(exchange string, trading string, settlement string) (bool, error)
	Order(exchange string, trading string, settlement string,
		ordertype models.OrderType, price float64, amount float64) (string, error)
	CancelOrder(exchange string, trading string, settlement string, orderType models.OrderType, orderNumber string) error
	//FilledOrderInfo(orderNumber string) (models.FilledOrderInfo,error)
	Transfer(exchange string, typ string, addr string,
		amount float64, additionalFee float64) error
	Address(exchange string, c string) (string, error)
}
