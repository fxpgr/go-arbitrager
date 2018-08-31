package entity

import (
	"github.com/fxpgr/go-exchange-client/models"

	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"github.com/pkg/errors"
	"strconv"
	"sync"
	"fmt"
)

func NewOpportunity(buySide Side, sellSide Side, tradingAmount float64) Opportunity {
	return Opportunity{
		Buyside:       buySide,
		Sellside:      sellSide,
		tradingAmount: tradingAmount,
	}
}
func NewOpportunities() *Opportunities {
	return &Opportunities{
		opps: make([]Opportunity, 0),
	}
}

type Opportunities struct {
	opps []Opportunity
}

func (os *Opportunities) GetAll() []Opportunity {
	return os.opps
}

func (os *Opportunities) IsOngoing(o *Opportunity) bool {
	for _, v := range os.opps {
		if v.BuySide() == o.BuySide() && v.SellSide() == o.SellSide() &&
			v.BuySidePair().Trading == o.BuySidePair().Trading &&
			v.BuySidePair().Settlement == o.BuySidePair().Settlement {
			return true
		}
	}
	return false
}

func (os *Opportunities) Set(o *Opportunity) error {
	os.opps = append(os.opps, *o)
	return nil
}

func (os *Opportunities) Remove(o *Opportunity) error {
	opps := make([]Opportunity, 0)
	for _, v := range os.opps {
		if v.BuySide() == o.BuySide() && v.SellSide() == o.SellSide() &&
			v.BuySidePair().Trading == o.BuySidePair().Trading &&
			v.BuySidePair().Settlement == o.BuySidePair().Settlement {
		} else {
			opps = append(opps, v)
		}
	}
	os.opps = opps
	return nil
}

func (os *Opportunities) BuySideFilter(exchange string) (*Opportunities, error) {
	opps := make([]Opportunity, 0)
	for _, o := range os.opps {
		if o.BuySide() == exchange {
			opps = append(opps, o)
		}
	}
	ret := &Opportunities{opps: opps}
	return ret, nil
}

func (os *Opportunities) HighestDifOpportunity() (Opportunity, error) {
	var ret Opportunity
	dif := 0.0
	for _, o := range os.opps {
		if dif < o.Dif() {
			dif = o.Dif()
			ret = o
		}
	}
	if len(os.opps) == 0 {
		return Opportunity{}, errors.New("there is no os")
	}
	return ret, nil
}

func NewSide(exchange string, rate float64, currencyPair models.CurrencyPair) Side {
	return Side{
		exchange:     exchange,
		rate:         rate,
		currencyPair: currencyPair,
	}
}

type Side struct {
	exchange     string
	rate         float64
	currencyPair models.CurrencyPair
}

type Opportunity struct {
	Buyside       Side
	Sellside      Side
	tradingAmount float64
}

var oppM sync.RWMutex

func (o Opportunity) MessageText() string {
	log := fmt.Sprintf("--------------------Opportunity--------------------\n")
	log += fmt.Sprintf("Buy  %4s-%4s On %8s At %v\n", o.Buyside.currencyPair.Trading, o.Buyside.currencyPair.Settlement, o.BuySide(), strconv.FormatFloat(o.Buyside.rate, 'f', 16, 64))
	log += fmt.Sprintf("Sell %4s-%4s On %8s At %v\n", o.Sellside.currencyPair.Trading, o.Sellside.currencyPair.Settlement, o.SellSide(), strconv.FormatFloat(o.Sellside.rate, 'f', 16, 64))
	log += fmt.Sprintf("Spread                      : %16s\n", strconv.FormatFloat(o.Sellside.rate-o.Buyside.rate, 'f', 16, 64))
	log += fmt.Sprintf("SpreadRate                  : %16s\n", strconv.FormatFloat(o.Sellside.rate/o.Buyside.rate, 'f', 16, 64))
	log += fmt.Sprintf("---------------------------------------------------")
	return log
}

func (o Opportunity) Print() {
	oppM.Lock()
	defer oppM.Unlock()
	logger.Get().Infof("--------------------Opportunity--------------------")
	logger.Get().Infof("Buy  %4s-%4s On %8s At %v", o.Buyside.currencyPair.Trading, o.Buyside.currencyPair.Settlement, o.BuySide(), strconv.FormatFloat(o.Buyside.rate, 'f', 16, 64))
	logger.Get().Infof("Sell %4s-%4s On %8s At %v", o.Sellside.currencyPair.Trading, o.Sellside.currencyPair.Settlement, o.SellSide(), strconv.FormatFloat(o.Sellside.rate, 'f', 16, 64))
	logger.Get().Infof("Spread                      : %16s", strconv.FormatFloat(o.Sellside.rate-o.Buyside.rate, 'f', 16, 64))
	logger.Get().Infof("SpreadRate                  : %16s", strconv.FormatFloat(o.Sellside.rate/o.Buyside.rate, 'f', 16, 64))
	logger.Get().Infof("---------------------------------------------------")
}

func (o *Opportunity) TradingAmount() float64 {
	return o.tradingAmount
}

func (o *Opportunity) Dif() float64 {
	if o.Buyside.rate > o.Sellside.rate {
		return o.Buyside.rate / o.Sellside.rate
	}
	return o.Sellside.rate / o.Buyside.rate
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
