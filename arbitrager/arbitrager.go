package arbitrager

import (
	"math"
	"sync"
	"fmt"
	"github.com/kokardy/listing"
	models2 "github.com/fxpgr/go-arbitrager/models"
	"time"
	"context"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-arbitrager/logger"
	"github.com/pkg/errors"
	"github.com/fxpgr/go-arbitrager/config"
)

type Arbitrager interface {
	RegisterExchanges([]string) error
	Opportunities() (opps models2.Opportunities, err error)
	WatchRate() error
	SyncRate() error
	CancelWatchRate()
	Inspect(models2.Opportunity) (error)
	Arbitrage(o *models2.Opportunity)(error)
}

type exchangeCurrencyPair struct {
	exchange string
	pair     models.CurrencyPair
}

type arbitragePair struct {
	a exchangeCurrencyPair
	b exchangeCurrencyPair
}

type simpleArbitrager struct {
	configPath string

	frozenCurrency models2.FrozenCurrencySyncMap
	exchangeSymbol models2.ExchangeSymbolSyncMap
	publicClient   models2.PublicClientSyncMap
	privateClient  models2.PrivateClientSyncMap
	rateMap        models2.RateSyncMap
	volumeMap      models2.VolumeSyncMap
	arbitragePairs []arbitragePair

	expectedProfitRate     float64
	watchArbitrageDuration time.Duration
	cancelFunc             context.CancelFunc
	verifyTracerSlice models2.VerifyTracerSyncSlice

}

func NewSimpleArbitrager(confPath string,expectedProfitRate float64) Arbitrager {
	return &simpleArbitrager{
		configPath:confPath,
		frozenCurrency:         models2.NewFrozenCurrencySyncMap(),
		exchangeSymbol:         models2.NewExchangeSymbolSyncMap(),
		publicClient:           models2.NewPublicClientSyncMap(),
		privateClient:          models2.NewPrivateClientSyncMap(),
		rateMap:                models2.NewRateSyncMap(),
		volumeMap:              models2.NewVolumeSyncMap(),
		arbitragePairs:         make([]arbitragePair, 0),
		expectedProfitRate:     expectedProfitRate,
		watchArbitrageDuration: time.Second * 15,
		verifyTracerSlice: models2.NewVerifyTracerSyncSlice(),
	}
}

func (a *simpleArbitrager) watch(exchange string, client public.PublicClient, ctx context.Context) {
	logger.Get().Debugf("currency pair rate watcher started: %v", exchange)
	tick := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-tick.C:
			rateMap, err := client.RateMap()
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
				continue
			}
			a.rateMap.Set(exchange, rateMap)
			volumeMap, err := client.VolumeMap()
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
				continue
			}
			a.volumeMap.Set(exchange, volumeMap)
		case <-ctx.Done():
			return
		}
	}
	return
}
const(
	PHASE_BUY = iota
	PHASE_TRANSFER
	PHASE_SELL
	PHASE_RETRANSFER
	RESTART
	COMPLETED
	INTERRUPTED
	END
)

func (a *simpleArbitrager) Arbitrage(o *models2.Opportunity)(error) {
	buySideBoard, err := a.publicClient.Get(o.BuySide()).Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
	if err != nil {
		logger.Get().Error(err)
		return err
	}
	sellSideBoard, err := a.publicClient.Get(o.SellSide()).Board(o.SellSidePair().Trading, o.SellSidePair().Settlement)
	if err != nil {
		logger.Get().Error(err)
		return err
	}
	bestSellPrice:= buySideBoard.BestSellPrice()
	bestBuyPrice:= sellSideBoard.BestBuyPrice()
	logger.Get().Infof("[Arbitrage] target buy price %10f ",bestSellPrice)
	logger.Get().Infof("[Arbitrage] target sell price %10f ",bestBuyPrice)
	logSign :=fmt.Sprintf("%v-%v %v-%v %10f",o.BuySide(),o.SellSide(),o.BuySidePair().Trading,o.BuySidePair().Settlement,bestBuyPrice/bestSellPrice)
	logger.Get().Infof("[Arbitrage] %v started",logSign)
	if a.verifyTracerSlice.IsDuplicated(o) {
		//logger.Get().Infof("duplicate opportunity detected %v-%v %v-%v",o.BuySide(),o.SellSide(),o.BuySidePair().Trading,o.BuySidePair().Settlement)
		return errors.New("duplicate opportunity detected")
	}
	phase := PHASE_BUY
	buySidePrivateClient := a.privateClient.Get(o.BuySide())
	sellSidePrivateClient := a.privateClient.Get(o.SellSide())
	buySideBalance,err := buySidePrivateClient.Balances()
	if err != nil {
		return errors.New("failed to get balance")
	}
	tradeFeeRate,err:= buySidePrivateClient.TradeFeeRate()
	if err !=nil {
		logger.Get().Error(err)
		return errors.New("failed to get trade fee rate")
	}
	orderFee := tradeFeeRate[o.BuySidePair().Trading][o.BuySidePair().Settlement].TakerFee
	var amount float64
	var orderedAmount float64
	var transferedAmount float64
ArbitrageLoopOuter:
	for{
	ArbitrageLoop:
		switch phase {
		case PHASE_BUY:
			logger.Get().Infof("[Arbitrage] %v phase_buy",logSign)
			buyBoard, err := a.publicClient.Get(o.BuySide()).Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
			if err != nil {
				logger.Get().Error(err)
				continue
			}
			bestBuyPrice:= buyBoard.BestBuyPrice()
			amount := (buySideBalance[o.BuySidePair().Settlement]/bestSellPrice)*(1-orderFee-0.0002)
			orderNumber,err := buySidePrivateClient.Order(
				o.BuySidePair().Trading,o.BuySidePair().Settlement,
				models.Ask,bestSellPrice,amount)
			if err != nil {
				logger.Get().Error(err)
				logger.Get().Error(amount)
				logger.Get().Infof("order fee rate: %v", orderFee)
				logger.Get().Infof("best buy price: %v", bestBuyPrice)
				phase=INTERRUPTED
				continue
			}
			logger.Get().Infof("[Arbitrage] order_number is %v",orderNumber)
			orderTimeLimit := time.Now().Add(time.Minute*10)
			fillCheckTicker := time.NewTicker(time.Second*3)
			for{
				select {
				case <- fillCheckTicker.C:
					isFilled,err:= buySidePrivateClient.IsOrderFilled(orderNumber,"")
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					if isFilled {
						logger.Get().Infof("[Arbitrage] %v is filled",orderNumber)
						orderedAmount = amount
						phase = PHASE_TRANSFER
						break  ArbitrageLoop
					}
					if !orderTimeLimit.After(time.Now())  {
						err = buySidePrivateClient.CancelOrder(orderNumber,o.BuySidePair().Trading+"_"+o.BuySidePair().Settlement)
						if err != nil {
							logger.Get().Error(err)
							continue
						}
						phase = INTERRUPTED
						break ArbitrageLoop
					}
				}
			}
		case PHASE_TRANSFER:
			logger.Get().Infof("[Arbitrage] %v phase_transfer",logSign)
			sellSideAddress,err :=sellSidePrivateClient.Address(o.SellSidePair().Trading)
			if err != nil {
				logger.Get().Error(err)
				time.Sleep(time.Second*3)
				continue
			}
			transferFee,err := buySidePrivateClient.TransferFee()
			if err != nil {
				logger.Get().Error(err)
				time.Sleep(time.Second*3)
				continue
			}
			err = buySidePrivateClient.Transfer(o.BuySidePair().Trading,sellSideAddress,orderedAmount-transferFee[o.BuySidePair().Trading],0)
			if err != nil {
				logger.Get().Error(err)
				logger.Get().Infof("[Arbitrage] transfer fee %v", transferFee)
				time.Sleep(time.Second*3)
				continue
			}

			transferCheckTicker := time.NewTicker(time.Second*30)
			for {
				select{
				case <-transferCheckTicker.C:

					transferedAmount := orderedAmount - transferFee[o.BuySidePair().Trading]
					balanceMap,err := sellSidePrivateClient.Balances()
					if err != nil {
						time.Sleep(time.Second*3)
						continue
					}
					sellSideBalance := balanceMap[o.SellSidePair().Trading]
					if sellSideBalance >= transferedAmount {
						phase = PHASE_SELL
						break ArbitrageLoop
					}
				}
			}
		case PHASE_SELL:
			logger.Get().Infof("[Arbitrage] %v phase_sell",logSign)
			sellBoard, err := a.publicClient.Get(o.SellSide()).Board(o.SellSidePair().Trading, o.SellSidePair().Settlement)
			if err != nil {
				logger.Get().Error(err)
				continue
			}
			bestSellPrice:= sellBoard.BestSellPrice()

			sellSideTradeFeeRate,err:= sellSidePrivateClient.TradeFeeRate()
			if err !=nil {
				logger.Get().Error(err)
				continue
			}
			orderFee := sellSideTradeFeeRate[o.SellSidePair().Trading][o.SellSidePair().Settlement].TakerFee
			amount := transferedAmount*(1-orderFee-0.0002)
			orderNumber,err := sellSidePrivateClient.Order(
				o.SellSidePair().Trading,o.SellSidePair().Settlement,
				models.Bid,bestSellPrice,amount)
			if err != nil {
				logger.Get().Error(err)
				continue
			}
			orderTimeLimit := time.Now().Add(time.Minute*10)
			fillCheckTicker := time.NewTicker(time.Second*3)
			for{
				select {
				case <- fillCheckTicker.C:
					isFilled,err:= sellSidePrivateClient.IsOrderFilled(orderNumber,"")
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					if isFilled {
						orderedAmount = amount
						phase = PHASE_RETRANSFER
						break ArbitrageLoop
					}
					if !orderTimeLimit.After(time.Now())  {
						err = sellSidePrivateClient.CancelOrder(orderNumber,o.SellSidePair().Trading+"_"+o.SellSidePair().Settlement)
						if err != nil {
							continue
						}
						break ArbitrageLoop
					}
				}
			}
		case PHASE_RETRANSFER:
			logger.Get().Infof("[Arbitrage] %v phase_retransfer",logSign)
			buySideAddress,err :=buySidePrivateClient.Address(o.BuySidePair().Settlement)
			if err != nil {
				time.Sleep(time.Second*3)
				continue
			}
			err = sellSidePrivateClient.Transfer(o.SellSidePair().Settlement,buySideAddress,orderedAmount,0)
			if err != nil {
				time.Sleep(time.Second*3)
				continue
			}

			transferCheckTicker := time.NewTicker(time.Second*30)
			for {
				select{
				case <-transferCheckTicker.C:
					transferFee,err := sellSidePrivateClient.TransferFee()
					if err != nil {
						time.Sleep(time.Second*3)
					}
					transferedAmount := orderedAmount - transferFee[o.SellSidePair().Settlement]
					balanceMap,err := buySidePrivateClient.Balances()
					if err != nil {
						time.Sleep(time.Second*3)
						continue
					}
					buySideBalance := balanceMap[o.BuySidePair().Settlement]
					if buySideBalance >= transferedAmount {
						phase = COMPLETED
						break ArbitrageLoop
					}
				}
			}
		case RESTART:
			logger.Get().Infof("[Arbitrage] %v phase_restart",logSign)
			time.Sleep(time.Second*3)
			phase = PHASE_BUY
		case INTERRUPTED:
			logger.Get().Info("[Fail] failed to arbitrage ")
			break ArbitrageLoopOuter
		case COMPLETED:
			logger.Get().Infof("[Success] arbitrage completed. you got %10f",transferedAmount-amount)
			break ArbitrageLoopOuter
		}
	}
	return nil
}

func (a *simpleArbitrager) Inspect(o models2.Opportunity) (error) {
	if a.verifyTracerSlice.IsDuplicated(&o) {
		return errors.New("duplicate opportunity detected")
	}
	logger.Get().Infof("[Inspect] %v-%v %v-%v",o.BuySide(),o.SellSide(),o.BuySidePair().Trading,o.BuySidePair().Settlement)
	vt := &models2.VerifyTracer{
		DateTime:time.Now(),
		Opportunity:o,
		TargetTime:time.Now().Add(time.Minute*30),
	}
	a.verifyTracerSlice.Set(vt)
	go func(a *simpleArbitrager,tracer *models2.VerifyTracer){
		defer a.verifyTracerSlice.Remove(tracer)
		sellBoard, err := a.publicClient.Get(o.SellSide()).Board(o.SellSidePair().Trading, o.SellSidePair().Settlement)
		if err != nil {
			logger.Get().Error(err)
			return
		}
		// buy loop
		// transfer loop
		// sell loop
		// transfer loop
		timer := time.NewTimer(tracer.TargetTime.Sub(time.Now()))
		<- timer.C
		buyBoard, err := a.publicClient.Get(o.BuySide()).Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
		if err != nil {
			logger.Get().Error(err)
			return
		}
		if err != nil {
			logger.Get().Error(err)
			return
		}
		buyRate:= buyBoard.BestBuyPrice()
		sellRate:= sellBoard.BestSellPrice()
		if buyRate==0 ||sellRate==0 {
			logger.Get().Error("[Result] got 0 board rate in opportunity verify tracing")
			return
		}
		sign := fmt.Sprintf("%v-%v %v-%v",tracer.Opportunity.BuySide(),tracer.Opportunity.SellSide(),tracer.Opportunity.BuySidePair().Trading,tracer.Opportunity.BuySidePair().Settlement)
		_ = fmt.Sprintf("%v-%v",tracer.Opportunity.BuySide(),tracer.Opportunity.BuySidePair().Trading)
		if err != nil {
			logger.Get().Errorf("[Result] failed to get sellside address %v",err)
			return
		}
		logger.Get().Infof("[Result] %v inspect triggerd on %v",sign, tracer.DateTime.String())
		logger.Get().Infof("[Result] %v opportunity was :%v",sign,tracer.Opportunity.Dif())
		logger.Get().Infof("[Result] %v opportunity got :%v",sign,sellRate/buyRate)
		logger.Get().Infof("[Result] %v buy rate(while ago):%10f buy rate(now):%10f",sign, tracer.Opportunity.BuySideRate(),buyRate)
		logger.Get().Infof("[Result] %v sell rate(while ago):%10f sell rate(now):%10f",sign, tracer.Opportunity.SellSideRate(),sellRate)
	}(a,vt)
	return  nil
}

func (a *simpleArbitrager) CancelWatchRate() {
	a.cancelFunc()
}

func (a *simpleArbitrager) WatchRate() error {
	logger.Get().Debug("rate watcher triggered")
	ctx, cancel := context.WithCancel(context.Background())
	a.cancelFunc = cancel
	for k, v := range a.publicClient.GetAll() {
		go a.watch(k, v, ctx)
	}
	return nil
}

func (a *simpleArbitrager) SyncRate() error {
	logger.Get().Debug("rate sync triggered")
	wg := &sync.WaitGroup{}
	for k, v := range a.publicClient.GetAll() {
		wg.Add(1)
		go func(exchange string, client public.PublicClient){
			logger.Get().Debugf("rate sync %v",exchange)
			rateMap, err := client.RateMap()
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
			}
			a.rateMap.Set(exchange, rateMap)
			volumeMap, err := client.VolumeMap()
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
			}
			a.volumeMap.Set(exchange, volumeMap)
			logger.Get().Debugf("rate sync completed %v",exchange)
			wg.Done()
		}(k,v)
	}
	wg.Wait()
	return nil
}

func (a *simpleArbitrager) RegisterExchanges(exchanges []string) error {
	logger.Get().Debug("exchanges are being registered")
	for _, v := range exchanges {
		c, err := public.NewClient(v)
		if err != nil {
			logger.Get().Errorf("exchange %v public client cannot be initialized", v)
			return err
		}
		a.publicClient.Set(v, c)
		conf := config.ReadConfig(a.configPath)
		setting := conf.Get(v)
		pc, err := private.NewClient(v, setting.APIKey, setting.SecretKey)
		if err != nil {
			logger.Get().Errorf("exchange %v private client cannot be initialized", v)
			return err
		}
		a.privateClient.Set(v, pc)
		pairs, err := c.CurrencyPairs()
		if err != nil {
			logger.Get().Errorf("failed to get currency pairs on exchange %v", v)
			return err
		}

		a.exchangeSymbol.Set(v, pairs)
		frozenCurrency, err := c.FrozenCurrency()
		if err != nil {
			logger.Get().Errorf("failed to get frozen currencies on exchange %v", v)
			return err
		}
		a.frozenCurrency.Set(v, frozenCurrency)
	}

	exchangeList := listing.StringReplacer(exchanges)
	for exchangeComb := range listing.Combinations(exchangeList, 2, false, 5) {
		exchangeComb := exchangeComb.(listing.StringReplacer)
		currencyPair1 := a.exchangeSymbol.Get(exchangeComb[0])
		currencyPair2 := a.exchangeSymbol.Get(exchangeComb[1])
		exchange1 := exchangeComb[0]
		exchange2 := exchangeComb[1]
		for _, c1 := range currencyPair1 {
			for _, c2 := range currencyPair2 {
				if c1.Settlement == c2.Settlement && c1.Trading == c2.Trading {
					isFrozen := false
					for _, fc := range a.frozenCurrency.GetAll() {
						for _, f := range fc {
							if c1.Trading == f || c2.Trading == f || c1.Settlement == f || c2.Settlement == f {
								isFrozen = true
							}
						}
					}
					if !isFrozen {
						a.arbitragePairs = append(a.arbitragePairs, arbitragePair{
							a: exchangeCurrencyPair{exchange: exchange1, pair: c1},
							b: exchangeCurrencyPair{exchange: exchange2, pair: c2},
						})
					}
				}
			}
		}
	}
	return nil
}

func (a *simpleArbitrager) Opportunities() (opps models2.Opportunities, err error) {
	for _, arbPair := range a.arbitragePairs {
		aBoard,err :=  a.publicClient.Get(arbPair.a.exchange).Board(arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
		if err != nil {
			logger.Get().Errorf("failed to get rate on exchange %v", arbPair.a.exchange)
			logger.Get().Error(err)
			continue
		}
		bBoard,err :=  a.publicClient.Get(arbPair.b.exchange).Board(arbPair.b.pair.Trading, arbPair.b.pair.Settlement)
		if err != nil {
			logger.Get().Errorf("failed to get rate on exchange %v", arbPair.b.exchange)
			logger.Get().Error(err)
			continue
		}
		aBestBuyPrice := aBoard.BestBuyPrice()
		aBestSellPrice:= aBoard.BestSellPrice()
		bBestBuyPrice := bBoard.BestBuyPrice() // Bid = you can buy
		bBestSellPrice := bBoard.BestSellPrice() // Ask = you can sell
		if bBestSellPrice- aBestBuyPrice/ aBestBuyPrice > 1+a.expectedProfitRate {
			spread := bBestSellPrice - aBestBuyPrice
			tradeAmount := math.Min(aBoard.BestBuyAmount(), bBoard.BestSellAmount())
			logger.Get().Infof("--------------------Opportunity--------------------")
			logger.Get().Infof("%v-%v %v-%v", arbPair.a.exchange, arbPair.b.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
			logger.Get().Infof("Best Bid       : %16s %10f", arbPair.b.exchange, bBestSellPrice)
			logger.Get().Infof("Best Ask       : %16s %10f", arbPair.a.exchange, aBestBuyPrice)
			logger.Get().Infof("Spread         : %16f", spread)
			logger.Get().Infof("ExpectedProfit : %16f %v", spread*tradeAmount, arbPair.a.pair.Trading)
			logger.Get().Infof("---------------------------------------------------")
		} else if aBestSellPrice- bBestBuyPrice/ bBestBuyPrice > 1+a.expectedProfitRate  {
			spread := aBestSellPrice - bBestBuyPrice
			tradeAmount := math.Min(bBoard.BestBuyAmount(),aBoard.BestSellAmount())

			logger.Get().Infof("--------------------Opportunity--------------------")
			logger.Get().Infof("%v-%v %v-%v",arbPair.a.exchange,arbPair.b.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
			logger.Get().Infof("Best Bid       : %16s %10f",arbPair.a.exchange, aBestSellPrice)
			logger.Get().Infof("Best Ask       : %16s %10f",arbPair.b.exchange, bBestBuyPrice)
			logger.Get().Infof("Spread         : %16f",spread)
			logger.Get().Infof("ExpectedProfit : %16f %v",spread*tradeAmount,arbPair.a.pair.Trading)
			logger.Get().Infof("---------------------------------------------------")
		}
	}
	return opps, nil
}
