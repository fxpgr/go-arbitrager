package usecase

import (
	"context"
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"github.com/fxpgr/go-arbitrager/infrastructure/persistence"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/kokardy/listing"
	"gopkg.in/mgo.v2"
	"math"
	"sync"
	"time"
)

func DisabledCurrencies() []string {
	return []string {"BTM"}
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

func NewArbitragePairs() *ArbitragePairs {
	return &ArbitragePairs{make([]ArbitragePair, 0)}
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

func (s *Scanner) SyncRate(exchanges []string) error {
	logger.Get().Debug("rate sync triggered")
	wg := &sync.WaitGroup{}
	for _, v := range exchanges {
		wg.Add(1)
		go func(exchange string) {
			defer wg.Done()
			logger.Get().Debugf("rate sync %v", exchange)
			RateMap, err := s.PublicResourceRepository.RateMap(exchange)
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v %v",exchange, err)
			}
			s.RateMap.Set(exchange, RateMap)
			VolumeMap, err := s.PublicResourceRepository.VolumeMap(exchange)
			if err != nil {
				logger.Get().Errorf("currency pair rate watcher faced error: %v", err)
			}
			s.VolumeMap.Set(exchange, VolumeMap)
			logger.Get().Debugf("rate sync completed %v", exchange)
		}(v)
	}
	wg.Wait()
	return nil
}

func (s *Scanner) FilterCurrency(currencies []string) error {
	ps:= make([]ArbitragePair,0)
	for _,p := range s.ArbitragePairs.pairs {
		isIncluded := false
		for _,c := range currencies {
			if p.a.pair.Settlement == c || p.a.pair.Trading ==c {
				isIncluded = true
			}
		}
		if isIncluded {
			ps = append(ps, p)
		}
	}
	s.ArbitragePairs.pairs = ps
	logger.Get().Infof("[Scanner] %v currency pairs registered after filtering", len(s.ArbitragePairs.pairs))
	return nil
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
		frozenCurrencies :=s.FrozenCurrency.GetAll()
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
				return
			}
			bBoard, err := s.PublicResourceRepository.Board(arbPair.b.exchange, arbPair.b.pair.Trading, arbPair.b.pair.Settlement)
			if err != nil {
				<-workers
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
			if aBestBidPrice==0||aBestAskPrice==0||bBestBidPrice==0||bBestAskPrice==0 {

			}else if bBestBidPrice/aBestAskPrice > 1+expectedProfitRate {
				tradeAmount := math.Min(aBoard.BestAskAmount(), bBoard.BestBidAmount())
				o := entity.NewOpportunity(entity.NewSide(arbPair.a.exchange, aBestAskPrice, arbPair.a.pair), entity.NewSide(arbPair.b.exchange, bBestBidPrice, arbPair.b.pair), tradeAmount)
				opps.Set(&o)
			} else if aBestBidPrice/bBestAskPrice > 1+expectedProfitRate {
				tradeAmount := math.Min(bBoard.BestAskAmount(), aBoard.BestBidAmount())
				o := entity.NewOpportunity(entity.NewSide(arbPair.b.exchange, bBestAskPrice, arbPair.b.pair), entity.NewSide(arbPair.a.exchange, aBestBidPrice, arbPair.a.pair), tradeAmount)
				opps.Set(&o)
			}
			<-workers
		}(arbPair)
	}
	wg.Wait()
	return opps, nil
}
