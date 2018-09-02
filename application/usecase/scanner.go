package usecase

import (
	"context"
	"fmt"
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"github.com/fxpgr/go-arbitrager/infrastructure/persistence"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/kokardy/listing"
	"gopkg.in/mgo.v2"
	"sync"
	"time"
	//"github.com/pkg/errors"
	"github.com/pkg/errors"
)

func DisabledCurrencies() []string {
	return []string{"BTM", "DCN"}
}

type TriangleScanner struct {
	MessageRepository         repository.MessageRepository         `inject:""`
	HistoryRepository         repository.HistoryRepository         `inject:""`
	PublicResourceRepository  repository.PublicResourceRepository  `inject:""`
	PrivateResourceRepository repository.PrivateResourceRepository `inject:""`
	FrozenCurrency            *entity.FrozenCurrencySyncMap        `inject:""`
	ExchangeSymbol            *entity.ExchangeSymbolSyncMap        `inject:""`
	RateMap                   *entity.RateSyncMap                  `inject:""`
	VolumeMap                 *entity.VolumeSyncMap                `inject:""`
	ArbitragePairs            *ArbitragePairs                      `inject:""`
	TriangleArbitrageTriples  *TriangleArbitrageTriples            `inject:""`
	cancelFunc                context.CancelFunc
}

type exchangeCurrencyPair struct {
	exchange string
	pair     models.CurrencyPair
}

type ArbitragePair struct {
	a exchangeCurrencyPair
	b exchangeCurrencyPair
}

type ArbitragePairs struct {
	pairs []ArbitragePair
}

func (a *ArbitragePairs) Get() []ArbitragePair {
	return a.pairs
}

type TriangleArbitrageTriple struct {
	a              exchangeCurrencyPair
	b              exchangeCurrencyPair
	intermediaPair exchangeCurrencyPair
}

type TriangleArbitrageTriples struct {
	triples []TriangleArbitrageTriple
}

func (a *TriangleArbitrageTriples) Get() []TriangleArbitrageTriple {
	return a.triples
}

func NewArbitragePairs() *ArbitragePairs {
	return &ArbitragePairs{make([]ArbitragePair, 0)}
}

func NewTriangleArbitrageTriples() *TriangleArbitrageTriples {
	return &TriangleArbitrageTriples{make([]TriangleArbitrageTriple, 0)}
}

type Scanner struct {
	MessageRepository         repository.MessageRepository         `inject:""`
	HistoryRepository         repository.HistoryRepository         `inject:""`
	PublicResourceRepository  repository.PublicResourceRepository  `inject:""`
	PrivateResourceRepository repository.PrivateResourceRepository `inject:""`
	FrozenCurrency            *entity.FrozenCurrencySyncMap        `inject:""`
	ExchangeSymbol            *entity.ExchangeSymbolSyncMap        `inject:""`
	RateMap                   *entity.RateSyncMap                  `inject:""`
	VolumeMap                 *entity.VolumeSyncMap                `inject:""`
	ArbitragePairs            *ArbitragePairs                      `inject:""`
	TriangleArbitrageTriples  *TriangleArbitrageTriples            `inject:""`
	cancelFunc                context.CancelFunc
}

func NewScanner(expectedProfitRate float64) *Scanner {
	session, _ := mgo.Dial("mongo:27017")
	return &Scanner{
		HistoryRepository: &persistence.HistoryRepositoryMgo{
			DB: session.DB("arbitrager"),
		},
		FrozenCurrency: entity.NewFrozenCurrencySyncMap(),
		ExchangeSymbol: entity.NewExchangeSymbolSyncMap(),
		RateMap:        entity.NewRateSyncMap(),
		VolumeMap:      entity.NewVolumeSyncMap(),
		ArbitragePairs: &ArbitragePairs{make([]ArbitragePair, 0)},
	}
}

func (s *Scanner) watch(exchange string, ctx context.Context) {
	logger.Get().Debugf("currency pair rate watcher started: %v", exchange)
	tick := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-tick.C:
			RateMap, err := s.PublicResourceRepository.RateMap(exchange)
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
				continue
			}
			s.RateMap.Set(exchange, RateMap)
			VolumeMap, err := s.PublicResourceRepository.VolumeMap(exchange)
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
				continue
			}
			s.VolumeMap.Set(exchange, VolumeMap)
		case <-ctx.Done():
			return
		}
	}
	return
}

func (s *Scanner) CancelWatchRate() {
	s.cancelFunc()
}

func (s *Scanner) TriggerWatch(exchanges []string) error {
	logger.Get().Debug("rate watcher triggered")
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelFunc = cancel
	for _, v := range exchanges {
		go s.watch(v, ctx)
	}
	return nil
}

func (s *Scanner) SyncRate(exchanges []string) (err error) {
	logger.Get().Debug("rate sync triggered")
	wg := &sync.WaitGroup{}
	for _, v := range exchanges {
		wg.Add(1)
		go func(exchange string) {
			defer wg.Done()
			logger.Get().Debugf("rate sync %v", exchange)
			RateMap, err := s.PublicResourceRepository.RateMap(exchange)
			if err != nil {
				err = errors.Errorf("currency pair rate watcher faced error: %v %v", exchange, err)
				logger.Get().Error(err)
			}
			s.RateMap.Set(exchange, RateMap)
			VolumeMap, err := s.PublicResourceRepository.VolumeMap(exchange)
			if err != nil {
				err = errors.Errorf("currency pair rate watcher faced error: %v", err)
				logger.Get().Error(err)
			}
			s.VolumeMap.Set(exchange, VolumeMap)
			logger.Get().Debugf("rate sync completed %v", exchange)
		}(v)
	}
	wg.Wait()
	return err
}

func (s *Scanner) FilterCurrency(currencies []string) error {
	ps := make([]ArbitragePair, 0)
	for _, p := range s.ArbitragePairs.pairs {
		isIncluded := false
		for _, c := range currencies {
			if p.a.pair.Settlement == c || p.a.pair.Trading == c {
				isIncluded = true
			}
		}
		if isIncluded {
			ps = append(ps, p)
		}
	}
	s.ArbitragePairs.pairs = ps
	s.MessageRepository.Send(fmt.Sprintf("[Scanner] %d currency pairs registered after filtering", len(s.ArbitragePairs.pairs)))
	return nil
}

func (s *Scanner) RegisterTrianglePairs(exchanges []string) error {
	logger.Get().Debug("exchanges are being registered")
	for _, e := range exchanges {
		pairs, err := s.PublicResourceRepository.CurrencyPairs(e)
		if err != nil {
			logger.Get().Errorf("failed to get currency pairs on exchange %v", e)
			return err
		}

		s.ExchangeSymbol.Set(e, pairs)
		FrozenCurrency, err := s.PublicResourceRepository.FrozenCurrency(e)
		if err != nil {
			logger.Get().Errorf("failed to get frozen currencies on exchange %v", e)
			return err
		}

		s.FrozenCurrency.Set(e, FrozenCurrency)
		currencyPairs := s.ExchangeSymbol.Get(e)
		settlementsMap := make(map[string][]string, 0)
		var settlements []string
		for _, c := range currencyPairs {
			isDisable := false
			for _, fc := range s.FrozenCurrency.Get(e) {
				if c.Trading == fc || c.Settlement == fc {
					isDisable = true
				}
			}
			for _, dc := range DisabledCurrencies() {
				if c.Trading == dc || c.Settlement == dc {
					isDisable = true
				}
			}
			if isDisable {
				continue
			}
			settlementsMap[c.Settlement] = append(settlementsMap[c.Settlement], c.Trading)
		}
		for k := range settlementsMap {
			settlements = append(settlements, k)
		}
		pibotCurrencyPairs := make([]models.CurrencyPair, 0)
		for _, s := range settlements {
			for _, c := range currencyPairs {
				if s == c.Trading {
					pibotCurrencyPairs = append(pibotCurrencyPairs,
						models.CurrencyPair{Trading: s, Settlement: c.Settlement})
				}
			}
		}

		for _, c := range pibotCurrencyPairs {
			tradings1, ok := settlementsMap[c.Settlement]
			tradings2, ok := settlementsMap[c.Trading]
			if !ok || !ok {
				continue
			}
			for _, t1 := range tradings1 {
				for _, t2 := range tradings2 {
					if t1 == t2 {
						s.TriangleArbitrageTriples.triples =
							append(s.TriangleArbitrageTriples.triples,
								TriangleArbitrageTriple{
									exchangeCurrencyPair{
										exchange: e,
										pair: models.CurrencyPair{
											Settlement: c.Settlement,
											Trading:    t1,
										},
									},
									exchangeCurrencyPair{
										exchange: e,
										pair: models.CurrencyPair{
											Settlement: c.Trading,
											Trading:    t1,
										},
									},
									exchangeCurrencyPair{
										exchange: e,
										pair: models.CurrencyPair{
											Settlement: c.Settlement,
											Trading:    c.Trading,
										},
									}})
						break
					}
				}
			}
		}
	}
	return nil
}

func (s *Scanner) TriangleOpportunities(expectedProfitRate float64) (opps entity.TriangleOpportunities, err error) {
	wg := &sync.WaitGroup{}
	workers := make(chan int, 5)
	for _, arbTriple := range s.TriangleArbitrageTriples.triples {
		wg.Add(1)
		workers <- 1
		go func(arbTriple TriangleArbitrageTriple) {
			defer wg.Done()
			/*aRate, err := s.PublicResourceRepository.Rate(arbTriple.a.exchange,arbTriple.a.pair.Trading,arbTriple.a.pair.Settlement)
			if err != nil {
				<-workers
				logger.Get().Error(err)
				return
			}
			bRate, err := s.PublicResourceRepository.Rate(arbTriple.b.exchange,arbTriple.b.pair.Trading,arbTriple.b.pair.Settlement)
			if err != nil {
				<-workers
				logger.Get().Error(err)
				return
			}
			intermediateRate, err := s.PublicResourceRepository.Rate(arbTriple.intermediaPair.exchange,arbTriple.intermediaPair.pair.Trading,arbTriple.intermediaPair.pair.Settlement)
			if err != nil {
				<-workers
				logger.Get().Error(err)
				return
			}
			tradeFeeMap,err := s.PrivateResourceRepository.TradeFeeRates(arbTriple.a.exchange)
			if err != nil {
				<-workers
				logger.Get().Error(err)
				return
			}
			_ = 1 - (1-tradeFeeMap[arbTriple.a.pair.Trading][arbTriple.a.pair.Settlement].TakerFee) *
				(1-tradeFeeMap[arbTriple.b.pair.Trading][arbTriple.b.pair.Settlement].TakerFee) *
				(1-tradeFeeMap[arbTriple.intermediaPair.pair.Trading][arbTriple.intermediaPair.pair.Settlement].TakerFee)

			if (intermediateRate*bRate) / aRate  > 1+expectedProfitRate  {
			opp := &entity.TriangleOpportunity{
				Triples:[]entity.Item{
					{"BUY",arbTriple.a.exchange,arbTriple.a.pair.Trading,arbTriple.a.pair.Settlement},
					{"SELL", arbTriple.b.exchange, arbTriple.b.pair.Trading,arbTriple.b.pair.Settlement},
					{"SELL",arbTriple.intermediaPair.exchange,arbTriple.intermediaPair.pair.Trading, arbTriple.intermediaPair.pair.Settlement},
				},
			}
			opps.Set(opp)
			} else if aRate / (intermediateRate*bRate) > 1+expectedProfitRate   {
			opp := &entity.TriangleOpportunity{
				Triples:[]entity.Item{
					{"BUY",arbTriple.intermediaPair.exchange,arbTriple.intermediaPair.pair.Trading, arbTriple.intermediaPair.pair.Settlement},
					{"BUY", arbTriple.b.exchange, arbTriple.b.pair.Trading,arbTriple.b.pair.Settlement},
					{"SELL",arbTriple.a.exchange,arbTriple.a.pair.Trading,arbTriple.a.pair.Settlement},
				},
			}
			opps.Set(opp)
			}*/
			aBoard, err := s.PublicResourceRepository.Board(arbTriple.a.exchange, arbTriple.a.pair.Trading, arbTriple.a.pair.Settlement)
			if err != nil {
				<-workers
				//logger.Get().Error(errors.Wrap(err,arbTriple.a.exchange+":"+arbTriple.a.pair.Trading+arbTriple.a.pair.Settlement))
				return
			}
			bBoard, err := s.PublicResourceRepository.Board(arbTriple.b.exchange, arbTriple.b.pair.Trading, arbTriple.b.pair.Settlement)
			if err != nil {
				<-workers
				//logger.Get().Error(errors.Wrap(err,arbTriple.b.exchange+":"+arbTriple.b.pair.Trading+arbTriple.b.pair.Settlement))
				return
			}
			intermediateBoard, err := s.PublicResourceRepository.Board(arbTriple.intermediaPair.exchange, arbTriple.intermediaPair.pair.Trading, arbTriple.intermediaPair.pair.Settlement)
			if err != nil {
				<-workers
				//logger.Get().Error(errors.Wrap(err,arbTriple.intermediaPair.exchange+":"+arbTriple.intermediaPair.pair.Trading+arbTriple.intermediaPair.pair.Settlement))
				return
			}
			tradeFeeMap, err := s.PrivateResourceRepository.TradeFeeRates(arbTriple.a.exchange)
			if err != nil {
				<-workers
				//logger.Get().Error(err)
				return
			}
			_ = 1 - (1-tradeFeeMap[arbTriple.a.pair.Trading][arbTriple.a.pair.Settlement].TakerFee)*
				(1-tradeFeeMap[arbTriple.b.pair.Trading][arbTriple.b.pair.Settlement].TakerFee)*
				(1-tradeFeeMap[arbTriple.intermediaPair.pair.Trading][arbTriple.intermediaPair.pair.Settlement].TakerFee)
			// aの買い手の最良希望価格
			aBestBidPrice := aBoard.BestBidPrice()
			// aの売り手の最良希望価格
			aBestAskPrice := aBoard.BestAskPrice()
			// bの買い手の最良希望価格
			bBestBidPrice := bBoard.BestBidPrice()
			// bの売り手の最良希望価格
			bBestAskPrice := bBoard.BestAskPrice()
			//
			intermediateBidPrice := intermediateBoard.BestBidPrice()
			// intermediateの売り手の最良希望価格
			intermediateAskPrice := intermediateBoard.BestAskPrice()
			if aBestBidPrice == 0 || aBestAskPrice == 0 || bBestBidPrice == 0 || bBestAskPrice == 0 || intermediateBidPrice == 0 || intermediateAskPrice == 0 {
				<-workers
				return
			}
			if (intermediateBidPrice*bBestBidPrice)/aBestAskPrice > 1+expectedProfitRate {
				opp := &entity.TriangleOpportunity{
					Triples: []entity.Item{
						{"BUY", arbTriple.a.exchange, arbTriple.a.pair.Trading, arbTriple.a.pair.Settlement},
						{"SELL", arbTriple.b.exchange, arbTriple.b.pair.Trading, arbTriple.b.pair.Settlement},
						{"SELL", arbTriple.intermediaPair.exchange, arbTriple.intermediaPair.pair.Trading, arbTriple.intermediaPair.pair.Settlement},
					},
				}
				opps.Set(opp)
			}
			if aBestBidPrice/(intermediateAskPrice*bBestAskPrice) > 1+expectedProfitRate {
				opp := &entity.TriangleOpportunity{
					Triples: []entity.Item{
						{"BUY", arbTriple.intermediaPair.exchange, arbTriple.intermediaPair.pair.Trading, arbTriple.intermediaPair.pair.Settlement},
						{"BUY", arbTriple.b.exchange, arbTriple.b.pair.Trading, arbTriple.b.pair.Settlement},
						{"SELL", arbTriple.a.exchange, arbTriple.a.pair.Trading, arbTriple.a.pair.Settlement},
					},
				}
				opps.Set(opp)
			}

			<-workers
		}(arbTriple)
	}
	wg.Wait()
	return opps, nil
}

func (s *Scanner) RegisterExchanges(exchanges []string) error {
	logger.Get().Debug("exchanges are being registered")
	for _, v := range exchanges {
		pairs, err := s.PublicResourceRepository.CurrencyPairs(v)
		if err != nil {
			logger.Get().Errorf("failed to get currency pairs on exchange %v", v)
			return err
		}

		s.ExchangeSymbol.Set(v, pairs)
		FrozenCurrency, err := s.PublicResourceRepository.FrozenCurrency(v)
		if err != nil {
			logger.Get().Errorf("failed to get frozen currencies on exchange %v", v)
			return err
		}
		s.FrozenCurrency.Set(v, FrozenCurrency)

	}

	exchangeList := listing.StringReplacer(exchanges)
	for exchangeComb := range listing.Combinations(exchangeList, 2, false, 5) {
		exchangeComb := exchangeComb.(listing.StringReplacer)
		currencyPair1 := s.ExchangeSymbol.Get(exchangeComb[0])
		currencyPair2 := s.ExchangeSymbol.Get(exchangeComb[1])
		exchange1 := exchangeComb[0]
		exchange2 := exchangeComb[1]
		disabledCurrencies := DisabledCurrencies()
		frozenCurrencies := s.FrozenCurrency.GetAll()
		for _, c1 := range currencyPair1 {
			for _, c2 := range currencyPair2 {
				if c1.Settlement == c2.Settlement && c1.Trading == c2.Trading {
					isFrozen := false
					for _, fc := range frozenCurrencies {
						for _, f := range fc {
							if c1.Trading == f || c2.Trading == f || c1.Settlement == f || c2.Settlement == f {
								isFrozen = true
							}
						}
					}
					for _, dc := range disabledCurrencies {
						if c1.Trading == dc || c1.Settlement == dc {
							isFrozen = true
						}
					}
					if !isFrozen {
						s.ArbitragePairs.pairs = append(s.ArbitragePairs.pairs, ArbitragePair{
							a: exchangeCurrencyPair{exchange: exchange1, pair: c1},
							b: exchangeCurrencyPair{exchange: exchange2, pair: c2},
						})
					}
				}
			}
		}
	}
	logger.Get().Infof("[Scanner] %v currency pairs registered", len(s.ArbitragePairs.pairs))
	return nil
}

func (s *Scanner) Opportunities(expectedProfitRate float64) (opps entity.Opportunities, err error) {
	wg := &sync.WaitGroup{}
	workers := make(chan int, 10)
	for _, arbPair := range s.ArbitragePairs.pairs {
		wg.Add(1)
		workers <- 1
		go func(arbPair ArbitragePair) {
			defer wg.Done()
			aBoard, err := s.PublicResourceRepository.Board(arbPair.a.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement)
			if err != nil {
				<-workers
				//logger.Get().Error(errors.Wrap(err,arbPair.a.exchange+":"+arbPair.a.pair.Trading+arbPair.a.pair.Settlement))
				return
			}
			bBoard, err := s.PublicResourceRepository.Board(arbPair.b.exchange, arbPair.b.pair.Trading, arbPair.b.pair.Settlement)
			if err != nil {
				<-workers
				//logger.Get().Error(errors.Wrap(err,arbPair.b.exchange+":"+arbPair.b.pair.Trading+arbPair.b.pair.Settlement))
				return
			}

			// aの買い手の最良希望価格
			aBestBidPrice := aBoard.BestBidPrice()
			// aの売り手の最良希望価格
			aBestAskPrice := aBoard.BestAskPrice()
			// bの買い手の最良希望価格
			bBestBidPrice := bBoard.BestBidPrice()
			// bの売り手の最良希望価格
			bBestAskPrice := bBoard.BestAskPrice()
			if aBestBidPrice == 0 || aBestAskPrice == 0 || bBestBidPrice == 0 || bBestAskPrice == 0 {

			} else if bBestBidPrice/aBestAskPrice > 1+expectedProfitRate {
				opp := entity.Opportunity{
					Pairs: []entity.Item{
						{"BUY", arbPair.a.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement},
						{"SELL", arbPair.b.exchange, arbPair.b.pair.Trading, arbPair.b.pair.Settlement},
					}}
				opps.Set(&opp)
			} else if aBestBidPrice/bBestAskPrice > 1+expectedProfitRate {
				opp := entity.Opportunity{
					Pairs: []entity.Item{
						{"BUY", arbPair.b.exchange, arbPair.b.pair.Trading, arbPair.b.pair.Settlement},
						{"SELL", arbPair.a.exchange, arbPair.a.pair.Trading, arbPair.a.pair.Settlement},
					}}
				opps.Set(&opp)
			}
			<-workers
		}(arbPair)
	}
	wg.Wait()
	return opps, nil
}
