package models

import (
	"github.com/fxpgr/go-exchange-client/models"

	"github.com/pkg/errors"
)

func NewOpportunity(exchangeA string, exchangeARate float64, exchangeACurrencyPair models.CurrencyPair,
	exchangeB string, exchangeBRate float64, exchangeBCurrencyPair models.CurrencyPair) Opportunity {
		return Opportunity{
			a:             exchangeA,
			aRate:         exchangeARate,
			aCurrencyPair: exchangeACurrencyPair,
			b:             exchangeB,
			bRate:         exchangeBRate,
			bCurrencyPair: exchangeBCurrencyPair,
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

type Opportunity struct {
	a             string
	aRate         float64
	aCurrencyPair models.CurrencyPair
	b             string
	bRate         float64
	bCurrencyPair models.CurrencyPair
	tradingAmount float64
}

func (o *Opportunity) TradingAmount() float64 {
	return o.tradingAmount
}

func (o *Opportunity) Dif() float64 {
	if o.aRate > o.bRate {
		return o.aRate / o.bRate
	}
	return o.bRate/ o.aRate
}

func (o *Opportunity) BuySideRate() (rate float64) {
	if o.BuySide()==o.a {
		return o.aRate
	}
	return o.bRate
}

func (o *Opportunity) SellSideRate() (rate float64) {
	if o.SellSide()==o.a {
		return o.aRate
	}
	return o.bRate
}

func (o *Opportunity) BuySide() (exchange string) {
	if o.aRate > o.bRate {
		return o.b
	}
	return o.a
}

func (o *Opportunity) SellSide() (exchange string) {
	if o.a == o.BuySide() {
		return o.b
	}
	return o.a
}

func (o *Opportunity) BuySidePair() models.CurrencyPair {
	if o.aRate > o.bRate {
		return o.aCurrencyPair
	}
	return o.bCurrencyPair
}

func (o *Opportunity) SellSidePair() models.CurrencyPair {
	if o.a == o.BuySide() {
		return o.bCurrencyPair
	}
	return o.aCurrencyPair
}

func (o *Opportunity) IsDuplicated(opp *Opportunity) bool {
	if o.a==opp.a&&o.aCurrencyPair == opp.aCurrencyPair &&o.b==opp.b&&o.bCurrencyPair==opp.bCurrencyPair {
		return true
	}
	if o.a==opp.b&&o.aCurrencyPair == opp.bCurrencyPair &&o.b==opp.a&&o.bCurrencyPair==opp.aCurrencyPair {
		return true
	}
	return false
}
