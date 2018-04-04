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
	"strconv"
)

type Arbitrager interface {
	RegisterExchanges(private.ClientMode,[]string) error
	Opportunities() (opps models2.Opportunities, err error)
	WatchRate() error
	SyncRate() error
	CancelWatchRate()
	Inspect(models2.Opportunity) (error)
	SwingArbitrage(position models.Position, o *models2.Opportunity)(error)
}

type exchangeCurrencyPair struct {
	exchange string
	pair     models.CurrencyPair
}

type arbitragePair struct {
	a exchangeCurrencyPair
	b exchangeCurrencyPair
}

type runningArbitragePairs struct {
	pairs []arbitragePair
	m *sync.Mutex
}

func(ra *runningArbitragePairs) Set(pair arbitragePair) {
	ra.m.Lock()
	defer ra.m.Unlock()
	ra.pairs = append(ra.pairs,pair)
}

func(ra *runningArbitragePairs) Delete(pair arbitragePair) error {
	ra.m.Lock()
	defer ra.m.Unlock()
	pairs := make([]arbitragePair,0)
	for _,v := range ra.pairs {
		if v != pair {
			pairs = append(pairs,v)
		}
	}
	ra.pairs = pairs
	return nil
}

func(ra *runningArbitragePairs) Exists(pair arbitragePair) bool {
	ra.m.Lock()
	defer ra.m.Unlock()
	for _,v := range ra.pairs {
		if v == pair {
			return true
		}
	}
	return false
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
	runningArbitragePairs runningArbitragePairs

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

func (a *simpleArbitrager) SwingArbitrage(position models.Position,o *models2.Opportunity)(error) {
	buySidePublicClient := a.publicClient.Get(o.BuySide())
	buySidePrivateClient := a.privateClient.Get(o.BuySide())
	sellSidePublicClient := a.publicClient.Get(o.SellSide())
	sellSidePrivateClient := a.privateClient.Get(o.SellSide())
	buySideBoard, err := buySidePublicClient.Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
	if err != nil {
		logger.Get().Error(err)
		return err
	}
	sellSideBoard, err := sellSidePublicClient.Board(o.SellSidePair().Trading, o.SellSidePair().Settlement)
	if err != nil {
		logger.Get().Error(err)
		return err
	}
	bestBuyPrice:= buySideBoard.BestAskPrice()
	bestSellPrice:= sellSideBoard.BestBidPrice()
	logSign :=fmt.Sprintf("%v-%v %v-%v %s",o.BuySide(),o.SellSide(),o.BuySidePair().Trading,o.BuySidePair().Settlement,strconv.FormatFloat(bestSellPrice/bestBuyPrice, 'f', 16, 64))
	logger.Get().Infof("[Arbitrage] %v started",logSign)
	logger.Get().Infof("[Arbitrage] detected arbitrage buy  price %s on %10s",strconv.FormatFloat(bestBuyPrice, 'f', 16, 64), o.BuySide())
	logger.Get().Infof("[Arbitrage] detected arbitrage sell price %s on %10s",strconv.FormatFloat(bestSellPrice, 'f', 16, 64), o.SellSide())

	var orderedAmount float64
	if position == models.Long {
		phase := PHASE_BUY
	LongLoop:
		for {
			switch phase {
			case PHASE_BUY:
				logger.Get().Infof("[Arbitrage] %v phase_buy", logSign)
				buyBoard, err := a.publicClient.Get(o.BuySide()).Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
				if err != nil {
					logger.Get().Error(err)
					phase = INTERRUPTED
					continue
				}
				tradeFeeRate,err:= buySidePrivateClient.TradeFeeRate(o.BuySidePair().Trading,o.BuySidePair().Settlement)
				if err !=nil {
					logger.Get().Error(err)
					phase = INTERRUPTED
					continue
				}
				orderFee := tradeFeeRate.TakerFee
				bestBuyPrice := buyBoard.BestAskPrice()
				amount := o.TradingAmount() * (1 - orderFee - 0.0002)
				logger.Get().Infof("[Arbitrage] buy price is %v", strconv.FormatFloat(bestBuyPrice, 'f', 16, 64))
				orderNumber, err := buySidePrivateClient.Order(
					o.BuySidePair().Trading, o.BuySidePair().Settlement,
					models.Ask, bestBuyPrice, amount)
				if err != nil {
					logger.Get().Error(err)
					logger.Get().Error(amount)
					logger.Get().Infof("order fee rate: %v", orderFee)
					logger.Get().Infof("best buy price: %v", bestBuyPrice)
					phase = INTERRUPTED
					continue
				}
				logger.Get().Infof("[Arbitrage] order_number is %v", orderNumber)
				orderTimeLimit := time.Now().Add(time.Minute * 10)
				fillCheckTicker := time.NewTicker(time.Second * 3)
			PhaseBuyLoop:
				for {
					select {
					case <-fillCheckTicker.C:
						isFilled, err := buySidePrivateClient.IsOrderFilled(orderNumber, "")
						if err != nil {
							logger.Get().Error(err)
							continue
						}
						if isFilled {
							logger.Get().Infof("[Arbitrage] %v is filled", orderNumber)
							orderedAmount = amount
							phase = PHASE_SELL
							break PhaseBuyLoop
						}
						if !orderTimeLimit.After(time.Now()) {
							err = buySidePrivateClient.CancelOrder(orderNumber, o.BuySidePair().Trading+"_"+o.BuySidePair().Settlement)
							if err != nil {
								logger.Get().Error(err)
								continue
							}
							phase = INTERRUPTED
							break PhaseBuyLoop
						}
					}
				}
			case PHASE_SELL:
				// if buyside price has reached at targetPrice
				//
				targetPrice := o.BuySideRate() * (1 + a.expectedProfitRate)
				arbitrageTimeLimit := time.Now().Add(time.Minute * 240)
				sellSideTradeFeeRate, err := buySidePrivateClient.TradeFeeRate(o.BuySidePair().Trading, o.BuySidePair().Settlement)
				if err != nil {
					logger.Get().Error(err)
					continue
				}
				orderFee := sellSideTradeFeeRate.TakerFee
				amount := orderedAmount * (1 - orderFee - 0.0002)
				logger.Get().Infof("[Arbitrage] %v phase_sell", logSign)
			PhaseSellLoop:
				for {

					sellBoard, err := a.publicClient.Get(o.BuySide()).Board(o.BuySidePair().Trading, o.BuySidePair().Settlement)
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					bestSellPrice := sellBoard.BestBidPrice()
					if targetPrice <= bestSellPrice || !arbitrageTimeLimit.After(time.Now()) {
						logger.Get().Infof("[Arbitrage] sell price is %v", strconv.FormatFloat(bestSellPrice, 'f', 16, 64))
						orderNumber, err := buySidePrivateClient.Order(
							o.BuySidePair().Trading, o.BuySidePair().Settlement,
							models.Bid, bestSellPrice, amount)
						if err != nil {
							logger.Get().Error(err)
							continue
						}
						orderTimeLimit := time.Now().Add(time.Minute * 10)
						fillCheckTicker := time.NewTicker(time.Second * 3)
						for {
							select {
							case <-fillCheckTicker.C:
								isFilled, err := buySidePrivateClient.IsOrderFilled(orderNumber, "")
								if err != nil {
									logger.Get().Error(err)
									continue
								}
								if isFilled {
									orderedAmount = amount
									phase = COMPLETED
									break PhaseSellLoop
								}
								if !orderTimeLimit.After(time.Now()) {
									err = sellSidePrivateClient.CancelOrder(orderNumber, o.SellSidePair().Trading+"_"+o.SellSidePair().Settlement)
									if err != nil {
										continue
									}
									break PhaseSellLoop
								}
							}
						}
					}
				}
			case INTERRUPTED:
				logger.Get().Info("[Fail] failed to arbitrage ")
				time.Sleep(time.Second * 3)
				logger.Get().Info("[Restart] arbitrager")
				phase = PHASE_BUY
			case COMPLETED:
				logger.Get().Infof("[Success] arbitrage completed.", )
				break LongLoop
			}
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
		buyRate:= buyBoard.BestBidPrice()
		sellRate:= sellBoard.BestAskPrice()
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
			defer wg.Done()
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
		}(k,v)
	}
	wg.Wait()
	return nil
}

func (a *simpleArbitrager) RegisterExchanges(mode private.ClientMode,exchanges []string) error {
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
		pc, err := private.NewClient(mode, v, setting.APIKey, setting.SecretKey)
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
	wg := &sync.WaitGroup{}
	workers := make(chan int, 10)
	for _, arbPair := range a.arbitragePairs {
		wg.Add(1)
		workers <- 1
		go func(arbPair arbitragePair) {
			defer wg.Done()
			aBoard,err :=  a.publicClient.Get(arbPair.a.exchange).Board(arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
			if err != nil {
				//logger.Get().Errorf("failed to get rate on exchange %v", arbPair.a.exchange)
				//logger.Get().Error(err)
				return
			}
			bBoard,err :=  a.publicClient.Get(arbPair.b.exchange).Board(arbPair.b.pair.Trading, arbPair.b.pair.Settlement)
			if err != nil {
				//logger.Get().Errorf("failed to get rate on exchange %v", arbPair.b.exchange)
				//logger.Get().Error(err)
				return
			}
			aBestBidPrice := aBoard.BestBidPrice()
			aBestAskPrice:= aBoard.BestAskPrice()
			bBestBidPrice := bBoard.BestBidPrice() // Bid = you can buy
			bBestAskPrice := bBoard.BestAskPrice() // Ask = you can sell
			if bBestBidPrice/aBestAskPrice > 1+a.expectedProfitRate {
				spread := bBestBidPrice - aBestAskPrice
				tradeAmount := math.Min(aBoard.BestBidAmount(), bBoard.BestAskAmount())
				logger.Get().Infof("--------------------Opportunity--------------------")
				logger.Get().Infof("%v-%v %v-%v", arbPair.a.exchange, arbPair.b.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
				logger.Get().Infof("Best Ask       : %16s %v", arbPair.a.exchange, strconv.FormatFloat(aBestAskPrice, 'f', 16, 64))
				logger.Get().Infof("Best Bid       : %16s %v", arbPair.b.exchange, strconv.FormatFloat(bBestBidPrice, 'f', 16, 64))
				logger.Get().Infof("Spread         : %16s %v", "",strconv.FormatFloat(spread, 'f', 16, 64))
				logger.Get().Infof("SpreadRate     : %16f", bBestBidPrice/aBestAskPrice )
				logger.Get().Infof("ExpectedProfit : %16f %v", spread*tradeAmount, arbPair.a.pair.Trading)
				logger.Get().Infof("---------------------------------------------------")
				o := models2.NewOpportunity(models2.NewSide(arbPair.a.exchange, aBestBidPrice, arbPair.a.pair),models2.NewSide(arbPair.b.exchange, bBestAskPrice, arbPair.b.pair), tradeAmount)
				opps = append(opps, o)

			} else if aBestBidPrice / bBestAskPrice > 1+a.expectedProfitRate  {
				spread := aBestBidPrice - bBestAskPrice
				tradeAmount := math.Min(bBoard.BestBidAmount(),aBoard.BestAskAmount())
				logger.Get().Infof("--------------------Opportunity--------------------")
				logger.Get().Infof("%v-%v %v-%v",arbPair.a.exchange,arbPair.b.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
				logger.Get().Infof("Best Bid       : %16s %v",arbPair.a.exchange, strconv.FormatFloat(aBestBidPrice, 'f', 16, 64))
				logger.Get().Infof("Best Ask       : %16s %v",arbPair.b.exchange, strconv.FormatFloat(bBestAskPrice, 'f', 16, 64))
				logger.Get().Infof("Spread         : %16s %v","", strconv.FormatFloat(spread, 'f', 16, 64))
				logger.Get().Infof("SpreadRate     : %16f", aBestBidPrice / bBestAskPrice )
				logger.Get().Infof("ExpectedProfit : %16f %v",spread*tradeAmount,arbPair.a.pair.Trading)
				logger.Get().Infof("---------------------------------------------------")
				o := models2.NewOpportunity(models2.NewSide(arbPair.b.exchange, bBestAskPrice, arbPair.b.pair),models2.NewSide(arbPair.a.exchange, aBestBidPrice, arbPair.a.pair), tradeAmount)
				opps = append(opps, o)
			}
			<-workers
		}(arbPair)
	}
	wg.Wait()
	return opps, nil
}
