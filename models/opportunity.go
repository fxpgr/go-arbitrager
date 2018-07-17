package models

import (
	"github.com/fxpgr/go-exchange-client/models"

	"github.com/pkg/errors"
)

func NewOpportunity(buySide Side,sellSide Side, tradingAmount float64) Opportunity {
	return Opportunity{
		Buyside:buySide,
		Sellside:sellSide,
		tradingAmount: tradingAmount,
	}
}

type Opportunities []Opportunity


func (os *Opportunities) BuySideFilter(exchange string) (*Opportunities,error) {
	ret := make(Opportunities,0)
	for _, o := range *os {
		if o.BuySide() == exchange {
			ret = append(ret, o)
		}
	}
	return &ret,nil
}

func (os *Opportunities) HighestDifOpportunity() (Opportunity,error) {
	var ret Opportunity
	dif := 0.0
	for _, o := range *os {
		if dif < o.Dif(){
			dif = o.Dif()
			ret = o
		}
	}
	if len(*os)== 0 {
		return Opportunity{},errors.New("there is no os")
	}
	return ret,nil
}

func NewSide(exchange string, rate float64, currencyPair models.CurrencyPair) Side {
	return Side{
		exchange:exchange,
		rate:rate,
		currencyPair:currencyPair,
	}
}

type Side struct {
	exchange string
	rate float64
	currencyPair models.CurrencyPair
}


type Opportunity struct {
	Buyside  Side
	Sellside Side
	tradingAmount float64
}

func (o *Opportunity) TradingAmount() float64 {
	return o.tradingAmount
}

func (o *Opportunity) Dif() float64 {
	if o.Buyside.rate > o.Sellside.rate {
		return o.Buyside.rate / o.Sellside.rate
	}
	return o.Sellside.rate/ o.Buyside.rate
}

func (o *Opportunity) BuySideRate() (rate float64) {
	return o.Buyside.rate
}

func (o *Opportunity) SellSideRate() (rate float64) {
	return o.Sellside.rate
}

func (o *Opportunity) BuySide() (exchange string) {
	return o.Buyside.exchange
}

func (o *Opportunity) SellSide() (exchange string) {
	return o.Sellside.exchange
}

func (o *Opportunity) BuySidePair() models.CurrencyPair {
	return o.Buyside.currencyPair
}

func (o *Opportunity) SellSidePair() models.CurrencyPair {
	return o.Sellside.currencyPair
}


func (o *Opportunity) IsDuplicated(opp *Opportunity) bool {
	if o == opp {
		return true
	}
	return false
}
