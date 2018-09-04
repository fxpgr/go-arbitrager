package entity

import (
	"github.com/fxpgr/go-exchange-client/models"
	"reflect"
	"time"
)

func NewOpportunity(items []Item) Opportunity {
	return Opportunity{
		Pairs: items,
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
	for _, w := range os.opps {
		if reflect.DeepEqual(w.Pairs, o.Pairs) {
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
	for _, w := range os.opps {
		if reflect.DeepEqual(w.Pairs, o.Pairs) {
		} else {
			opps = append(opps, w)
		}
	}
	os.opps = opps
	return nil
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

type Item struct {
	Op         string
	Exchange   string
	Trading    string
	Settlement string
}

type TriangleOpportunity struct {
	StartedTime      time.Time
	InitialBuyPrice  float64
	InitialSellPrice float64
	ClosedTime       time.Time
	Triples          []Item
}

type TriangleOpportunities struct {
	opps []*TriangleOpportunity
}

func NewTriangleOpportunity(triples []Item) *TriangleOpportunity {
	return &TriangleOpportunity{
		StartedTime: time.Time{},
		Triples:     triples,
	}
}

func (t *TriangleOpportunity) InterMediaMethod() string {
	buyCounter := 0
	for _, item := range t.Triples {
		if item.Op == "BUY" {
			buyCounter += 1
		}
	}
	if buyCounter == 2 {
		return "BUY"
	}
	return "SELL"
}

func NewTriangleOpportunities() *TriangleOpportunities {
	return &TriangleOpportunities{
		opps: make([]*TriangleOpportunity, 0),
	}
}
func (os *TriangleOpportunities) GetAll() []*TriangleOpportunity {
	return os.opps
}

func (os *TriangleOpportunities) IsOngoing(o *TriangleOpportunity) bool {
	for _, w := range os.opps {
		for _, x := range w.Triples {
			for _, p := range o.Triples {
				if p.Trading == x.Trading && p.Settlement == x.Settlement &&
					p.Exchange == x.Exchange && p.Op == x.Op {
					return true
				}
			}
		}
	}
	return false
}

func (os *TriangleOpportunities) Set(o *TriangleOpportunity) error {
	os.opps = append(os.opps, o)
	return nil
}

func (os *TriangleOpportunities) Remove(o *TriangleOpportunity) error {
	opps := make([]*TriangleOpportunity, 0)
	for _, w := range os.opps {
		for _, x := range w.Triples {
			for _, p := range o.Triples {
				if p.Trading == x.Trading && p.Settlement == x.Settlement &&
					p.Exchange == x.Exchange && p.Op == x.Op {
				} else {
					opps = append(opps, w)
				}
			}
		}
	}
	os.opps = opps
	return nil
}

type Opportunity struct {
	Pairs []Item
}

func (o *Opportunity) IsDuplicated(opp *Opportunity) bool {
	if o == opp {
		return true
	}
	return false
}
