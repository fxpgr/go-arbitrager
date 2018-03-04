package main

import (
	"fmt"
	"github.com/fxpgr/go-ccex-api-client/api/public"
	"github.com/fxpgr/go-ccex-api-client/models"
	"time"
	"os"
	"github.com/fxpgr/go-arbitrager/config"
	"github.com/fxpgr/go-ccex-api-client/api/private"
)

type exchangeCurrencyPair struct {
	exchange string
	pair     models.CurrencyPair
}

type arbitragePair struct {
	a exchangeCurrencyPair
	b exchangeCurrencyPair
}

var (
	rates = make(map[string]map[string]map[string]float64)
)

func help_and_exit() {
	fmt.Fprintf(os.Stderr, "%s config.yml\n", os.Args[0])
	os.Exit(1)
}

func watcher(exchange string, client public.PublicClient) {
	tick := time.NewTicker(time.Second * 3)
	for {
		select {
		case <-tick.C:
			rateMap, err := client.RateMap()
			if err != nil {
				fmt.Println(err)
				continue
			}
			rates[exchange] = rateMap
		}
	}
	return
}

func main() {
	if len(os.Args) != 2 {
		help_and_exit()
	}
	confPath := os.Args[1]
	conf := config.ReadConfig(confPath)

	frozenCurrenciesMap := make(map[string][]string)
	currencyPairs := make(map[string][]models.CurrencyPair)
	clients := make(map[string]public.PublicClient)
	privateClients := make(map[string]private.PrivateClient)
	exchanges := []string{"hitbtc", "poloniex"}
	for _, v := range exchanges {
		c, err := public.NewClient(v)
		if err != nil {
			continue
		}
		clients[v] = c
		setting := conf.Get(v)
		pc,err := private.NewClient(v,setting.APIKey,setting.SecretKey)
		if err != nil {
			continue
		}
		privateClients[v] = pc
		pairs, err := c.CurrencyPairs()
		if err != nil {
			continue
		}
		currencyPairs[v] = pairs
		frozenCurrency, err := c.FrozenCurrency()
		frozenCurrenciesMap[v] = frozenCurrency
		go watcher(v, c)
	}
	arbitragePairs := make([]arbitragePair, 0)
	for exchange1, currencyPair1 := range currencyPairs {
		for exchange2, currencyPair2 := range currencyPairs {
			if exchange1 == exchange2 {
				continue
			}
			for _, c1 := range currencyPair1 {
				for _, c2 := range currencyPair2 {
					if c1.Settlement == c2.Settlement && c1.Trading == c2.Trading {
						isFrozen := false
						for _, fc := range frozenCurrenciesMap {
							for _, f := range fc {
								if c1.Trading == f || c2.Trading == f || c1.Settlement == f || c2.Settlement == f {
									isFrozen = true
								}
							}
						}
						if !isFrozen {
							arbitragePairs = append(arbitragePairs, arbitragePair{
								a: exchangeCurrencyPair{exchange: exchange1, pair: c1},
								b: exchangeCurrencyPair{exchange: exchange2, pair: c2},
							})
						}
					}
				}
			}
		}
	}
	var x, y float64
	for {
		for _, arbPair := range arbitragePairs {
			x = rates[arbPair.a.exchange][arbPair.a.pair.Trading][arbPair.a.pair.Settlement]
			y = rates[arbPair.b.exchange][arbPair.b.pair.Trading][arbPair.b.pair.Settlement]
			if (x/y) > 1.02 || 0.98 > (x/y) {
				fmt.Printf("%v-%v %v/%v\n", arbPair.a.exchange, arbPair.b.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
				fmt.Printf("A:%v B:%v Rate:%v\n", x, y, x/y)
				transferFee,_:=privateClients[arbPair.a.exchange].TransferFee()
				purchaseFee,_:=privateClients[arbPair.a.exchange].PurchaseFeeRate()
				fmt.Printf("%v %v",transferFee,purchaseFee)
			}
		}
		time.Sleep(time.Second * 3)
	}
}
