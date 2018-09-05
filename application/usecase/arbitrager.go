package usecase

import (
	"fmt"
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"strconv"
	"sync"
	"time"
	"github.com/pkg/errors"
	"github.com/fxpgr/go-exchange-client/models"
)

var (
	languages map[string]float64
)

func getTradeAmount(coin string) float64 {
	switch coin {
	case "BTC":
		return 0.002
	case "ETH":
		return 0.05
	}
	return 0
}

type Arbitrager struct {
	MessageRepository         repository.MessageRepository         `inject:""`
	PublicResourceRepository  repository.PublicResourceRepository  `inject:""`
	PrivateResourceRepository repository.PrivateResourceRepository `inject:""`
	OngoingOpps               *entity.Opportunities                `inject:""`
	OngoingTriangleOpps       *entity.TriangleOpportunities        `inject:""`
}

func NewArbitrager() *Arbitrager {
	return &Arbitrager{
		OngoingOpps: entity.NewOpportunities(),
	}
}

const (
	PHASE_BUY = iota
	PHASE_TRANSFER
	PHASE_SELL
	PHASE_RETRANSFER
	RESTART
	COMPLETED
	INTERRUPTED
	END
)
/*
func (s *Arbitrager) Trace(position models.Position, o entity.Opportunity, expectedProfitRate float64) error {
	if s.OngoingOpps.IsOngoing(&o) {
		return nil
	}
	if err := s.OngoingOpps.Set(&o); err != nil {
		logger.Get().Errorf("[Error] %s", err)
		return err
	}
	var initialBuyPrice float64
	var initialBuyAmount float64
	var initialSellPrice float64
	var initialSellAmount float64

	messageText := make([]string, 0)
	messageText = append(messageText, fmt.Sprintf("--------------------Opportunity--------------------"))
	for _, item := range o.Pairs {
		board, err := s.PublicResourceRepository.Board(item.Exchange, item.Trading, item.Settlement)
		if err != nil {
			logger.Get().Errorf("[Error] %s", err)
			s.OngoingOpps.Remove(&o)
			return err
		}
		if item.Op == "BUY" {
			fmt.Println(board.AverageAskRate(getTradeAmount(item.Settlement)))
			initialBuyPrice = board.BestAskPrice()
			initialBuyAmount = board.BestAskAmount()
			messageText = append(messageText, fmt.Sprintf("%-4s %-5s-%-5s On %-8s At %v", item.Op, item.Trading, item.Settlement, item.Exchange, strconv.FormatFloat(initialBuyPrice, 'f', 16, 64)))
			messageText = append(messageText, fmt.Sprintf("BuyAmount                     : %8s%5s", strconv.FormatFloat(initialBuyPrice*initialBuyAmount, 'f', 16, 64), item.Settlement))
		} else {
			fmt.Println(board.AverageBidRate(getTradeAmount(item.Settlement)))
			initialSellPrice = board.BestBidPrice()
			initialSellAmount = board.BestBidAmount()
			messageText = append(messageText, fmt.Sprintf("%-4s %-5s-%-5s On %-8s At %v", item.Op, item.Trading, item.Settlement, item.Exchange, strconv.FormatFloat(initialSellPrice, 'f', 16, 64)))
			messageText = append(messageText, fmt.Sprintf("SellAmount                    : %8s%5s", strconv.FormatFloat(initialSellPrice*initialSellAmount, 'f', 16, 64), item.Settlement))
		}
	}
	if initialSellPrice/initialBuyPrice < 1+expectedProfitRate {
		s.OngoingOpps.Remove(&o)
		return nil
	}

	messageText = append(messageText, fmt.Sprintf("Spread                      : %16s", strconv.FormatFloat(initialSellPrice-initialBuyPrice, 'f', 16, 64)))
	messageText = append(messageText, fmt.Sprintf("SpreadRate                  : %16s", strconv.FormatFloat(initialSellPrice/initialBuyPrice, 'f', 16, 64)))
	messageText = append(messageText, fmt.Sprintf("---------------------------------------------------"))
	s.MessageRepository.BulkSend(messageText)
	s.MessageRepository.Send(fmt.Sprintf("[Arbitrager] then I'll trace margin until its convergenced"))

	time.Sleep(3 * time.Second)
	var bestBuyPrice float64
	var bestSellPrice float64
	for {
		for _, item := range o.Pairs {
			board, err := s.PublicResourceRepository.Board(item.Exchange, item.Trading, item.Settlement)
			if err != nil {
				logger.Get().Errorf("[Error] %s", err)
				time.Sleep(3 * time.Second)
				continue
			}
			if item.Op == "BUY" {
				bestBuyPrice = board.BestAskPrice()
			} else {
				bestSellPrice = board.BestBidPrice()
			}
		}
		if bestSellPrice/bestBuyPrice < 1+expectedProfitRate/4 {
			s.MessageRepository.Send(fmt.Sprintf("[Arbitrager] convergence found"))
			messageText := make([]string, 0)
			messageText = append(messageText, "--------------------Convergence--------------------")
			for _, item := range o.Pairs {
				messageText = append(messageText, fmt.Sprintf("%4s %4s-%4s On %8s", item.Op, item.Trading, item.Settlement, item.Exchange))
			}
			messageText = append(messageText, fmt.Sprintf("BUY  : %16s -> %16s", strconv.FormatFloat(initialBuyPrice, 'f', 16, 64), strconv.FormatFloat(bestBuyPrice, 'f', 16, 64)))
			messageText = append(messageText, fmt.Sprintf("SELL : %16s -> %16s", strconv.FormatFloat(initialSellPrice, 'f', 16, 64), strconv.FormatFloat(bestSellPrice, 'f', 16, 64)))
			messageText = append(messageText, fmt.Sprintf("---------------------------------------------------"))
			s.MessageRepository.BulkSend(messageText)
			break
		}
		time.Sleep(15 * time.Second)
	}

	if err := s.OngoingOpps.Remove(&o); err != nil {
		logger.Get().Error(err)
		return err
	}
	return nil
}*/

func (s *Arbitrager) TradeTriangle(o *entity.TriangleOpportunity, expectedProfitRate float64) error {
	// does not implemented
	computableBoardArray := &entity.ComputableBoardTriangleArray{
		Arr: make([]entity.ComputableBoard, 0),
	}
	for _, item := range o.Triples {
		board, err := s.PublicResourceRepository.Board(item.Exchange, item.Trading, item.Settlement)
		if err != nil {
			return err
		}
		computableBoardArray.Set(entity.NewComputableBoard(*board, item))
	}

	buyPrice, sellPrice, err := computableBoardArray.SpreadPrices()
	if err != nil {
		return err
	}

	if sellPrice/buyPrice < 1 + expectedProfitRate {
		return errors.Errorf("there is not margin")
	}
	tradeAmount,currency,err :=computableBoardArray.GetTradeAmount()
	if err != nil {
		return err
	}
	tradeFlag := false
	if currency == "BTC" && tradeAmount >= 0.0001 {
		tradeFlag = true
	} else if currency ==  "ETH" && tradeAmount >= 0.001 {
		tradeFlag = true
	}
	if tradeFlag {
		balances, err:= s.PrivateResourceRepository.Balances(o.Triples[0].Exchange)
		if err !=nil {
			return err
		}
		if balances[currency] <= tradeAmount {
			return errors.Errorf("there is no amount of %s (expected %s but %s)",currency, tradeAmount, balances[currency])
		}
		for _,cb := range computableBoardArray.Arr {
			var (
				orderType models.OrderType
				price float64
				amount float64
			)
			if cb.Item.Op == "BUY" {
				orderType = models.Bid
				price = cb.BestAskPrice()
				amount = cb.BestAskAmount()
			} else if cb.Item.Op == "SELL" {
				orderType = models.Ask
				price = cb.BestBidPrice()
				amount = cb.BestBidAmount()
			}
			orderId,err:=s.PrivateResourceRepository.Order(cb.Item.Exchange, cb.Item.Trading, cb.Item.Settlement, orderType, price, amount)
			if err != nil {
				return err
			}
			isFilled, err := s.PrivateResourceRepository.IsOrderFilled(orderId, cb.Item.Trading, cb.Item.Settlement)
			if err != nil {
				return err
			}
			time.Sleep(time.Second * 5)
			if !isFilled {
				return errors.Errorf("the trade is not contracted!!! within 5 sec!")
			}
		}
	}
	return nil
}

func (s *Arbitrager) TraceTriangle(opps []*entity.TriangleOpportunity, expectedProfitRate float64) error {
	for _, o := range opps {
		if s.OngoingTriangleOpps.IsOngoing(o) {
			continue
		}
		if err := s.OngoingTriangleOpps.Set(o); err != nil {
			logger.Get().Errorf("[Error] %s", err)
			continue
		}
	}

	workers := make(chan int, 4)
	wg := &sync.WaitGroup{}

	for _, ongoingOpp := range s.OngoingTriangleOpps.GetAll() {
		wg.Add(1)
		workers <- 1
		go func(ongoingOpp *entity.TriangleOpportunity) {
			defer wg.Done()
			messageText := make([]string, 0)
			messageText = append(messageText, fmt.Sprintf("--------------------Opportunity--------------------"))


			computableBoardArray := &entity.ComputableBoardTriangleArray{
				Arr: make([]entity.ComputableBoard, 0),
			}
			for _, item := range ongoingOpp.Triples {
				board, err := s.PublicResourceRepository.Board(item.Exchange, item.Trading, item.Settlement)
				if err != nil {
					//logger.Get().Errorf("[Error] %s", err)
					s.OngoingTriangleOpps.Remove(ongoingOpp)
					<-workers
					return
				}
				computableBoardArray.Set(entity.NewComputableBoard(*board, item))
			}
			buyPrice, sellPrice, err := computableBoardArray.SpreadPrices()


			if sellPrice/buyPrice > 1+expectedProfitRate {
				if ongoingOpp.StartedTime.IsZero() == true {
					// initialize
					ongoingOpp.StartedTime = time.Now()
					ongoingOpp.InitialBuyPrice = buyPrice
					ongoingOpp.InitialSellPrice = sellPrice
					generatedMessage, err := computableBoardArray.GenerateText()
					if err != nil {
						logger.Get().Errorf("[Error] %s", err)
						s.OngoingTriangleOpps.Remove(ongoingOpp)
						<-workers
						return
					}
					messageText = append(messageText, generatedMessage...)
					messageText = append(messageText, fmt.Sprintf("Spread                        : %16s", strconv.FormatFloat(sellPrice-buyPrice, 'f', 16, 64)))
					messageText = append(messageText, fmt.Sprintf("SpreadRate                    : %16s", strconv.FormatFloat(sellPrice/buyPrice, 'f', 16, 64)))
					messageText = append(messageText, fmt.Sprintf("---------------------------------------------------"))
					s.MessageRepository.BulkSend(messageText)
					<-workers
					return
				}
			}

			if sellPrice/buyPrice < 1+expectedProfitRate/4 {
				if ongoingOpp.StartedTime.IsZero() == true {
					s.OngoingTriangleOpps.Remove(ongoingOpp)
					<-workers
					return
				}
				// close処理
				computableBoardArray := &entity.ComputableBoardTriangleArray{
					Arr: make([]entity.ComputableBoard, 0),
				}
				for _, item := range ongoingOpp.Triples {
					board, err := s.PublicResourceRepository.Board(item.Exchange, item.Trading, item.Settlement)
					if err != nil {
						//logger.Get().Errorf("[Error] %s : %s", item.Exchange, err)
						continue
					}
					computableBoardArray.Set(entity.NewComputableBoard(*board, item))
				}
				buyPrice, sellPrice, err = computableBoardArray.SpreadPrices()
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					<-workers
					return
				}
				if sellPrice/buyPrice < 1+expectedProfitRate/4 {
					messageText := make([]string, 0)
					messageText = append(messageText, "--------------------Convergence--------------------")
					for _, item := range ongoingOpp.Triples {
						messageText = append(messageText, fmt.Sprintf("%4s %4s-%4s On %8s", item.Op, item.Trading, item.Settlement, item.Exchange))
					}
					tradeAmount,pivotCurrency, err := computableBoardArray.GetTradeAmount()
					if err != nil {
						logger.Get().Errorf("[Error] %s", err)
						<-workers
						return
					}
					messageText = append(messageText, fmt.Sprintf("BUY  : %16s -> %16s", strconv.FormatFloat(ongoingOpp.InitialBuyPrice, 'f', 16, 64), strconv.FormatFloat(buyPrice, 'f', 16, 64)))
					messageText = append(messageText, fmt.Sprintf("SELL : %16s -> %16s", strconv.FormatFloat(ongoingOpp.InitialSellPrice, 'f', 16, 64), strconv.FormatFloat(sellPrice, 'f', 16, 64)))
					messageText = append(messageText, fmt.Sprintf("TradeAmount                : %8s%5s", strconv.FormatFloat(tradeAmount, 'f', 16, 64), pivotCurrency))
					messageText = append(messageText, fmt.Sprintf("---------------------------------------------------"))
					s.MessageRepository.BulkSend(messageText)
					err = s.OngoingTriangleOpps.Remove(ongoingOpp)
					if err != nil {
						logger.Get().Errorf("[Error] %s", err)
						<-workers
						return
					}
				}
			}
			<-workers
		}(ongoingOpp)

	}
	wg.Wait()
	return nil
}

/*func (s *Arbitrager) Arbitrage(position models.Position, o entity.Opportunity, expectedProfitRate float64) error {
	if s.OngoingOpps.IsOngoing(&o) {
		logger.Get().Infof("duplicated %v", o)
		return nil
	}
	if err := s.OngoingOpps.Set(&o); err != nil {
		logger.Get().Errorf("[Error] %s", err)
		return err
	}

	buySideBoard, err := s.PublicResourceRepository.Board(o.BuySide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
	if err != nil {
		logger.Get().Errorf("[Error] %s", err)
		return err
	}
	sellSideBoard, err := s.PublicResourceRepository.Board(o.SellSide(), o.SellSidePair().Trading, o.SellSidePair().Settlement)
	if err != nil {
		logger.Get().Errorf("[Error] %s", err)
		return err
	}
	bestBuyPrice := buySideBoard.BestAskPrice()
	bestSellPrice := sellSideBoard.BestBidPrice()
	if bestSellPrice/bestBuyPrice < 1+expectedProfitRate {
		return nil
	}
	logLabel := fmt.Sprintf("%v-%v %v-%v %s", o.BuySide(), o.SellSide(), o.BuySidePair().Trading, o.BuySidePair().Settlement, strconv.FormatFloat(bestSellPrice/bestBuyPrice, 'f', 16, 64))
	var orderedAmount float64
	if position == models.Long {
		phase := PHASE_BUY
	LongLoop:
		for {
			switch phase {
			case PHASE_BUY:
				logger.Get().Infof("[Arbitrage] %s: PHASE_BUY", logLabel)
				buyBoard, err := s.PublicResourceRepository.Board(o.BuySide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					phase = INTERRUPTED
					continue
				}
				tradeFeeRate, err := s.PrivateResourceRepository.TradeFeeRate(o.BuySide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					phase = INTERRUPTED
					continue
				}
				orderFee := tradeFeeRate.TakerFee
				bestBuyPrice := buyBoard.BestAskPrice()
				amount := 1 * (1 - orderFee - 0.0002)
				orderNumber, err := s.PrivateResourceRepository.Order(
					o.BuySide(),
					o.BuySidePair().Trading, o.BuySidePair().Settlement,
					models.Ask, bestBuyPrice, amount)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					phase = INTERRUPTED
					continue
				}
				orderTimeLimit := time.Now().Add(time.Minute * 10)
				fillCheckTicker := time.NewTicker(time.Second * 3)
			PhaseBuyLoop:
				for {
					select {
					case <-fillCheckTicker.C:
						isFilled, err := s.PrivateResourceRepository.IsOrderFilled(o.BuySide(), orderNumber, "")
						if err != nil {
							logger.Get().Errorf("[Error] %s", err)
							continue
						}
						if isFilled {
							orderedAmount = amount
							phase = PHASE_TRANSFER
							break PhaseBuyLoop
						}
						if !orderTimeLimit.After(time.Now()) {
							err = s.PrivateResourceRepository.CancelOrder(o.BuySide(), orderNumber, o.BuySidePair().Trading+"_"+o.BuySidePair().Settlement)
							if err != nil {
								logger.Get().Errorf("[Error] %s", err)
								continue
							}
							phase = INTERRUPTED
							break PhaseBuyLoop
						}
					}
				}

			case PHASE_TRANSFER:
				logger.Get().Infof("[Arbitrage] %s: PHASE_TRANSFER", logLabel)
				address, err := s.PrivateResourceRepository.Address(o.SellSide(), o.SellSidePair().Trading)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					continue
				}
				err = s.PrivateResourceRepository.Transfer(o.BuySide(), o.BuySidePair().Trading, address, orderedAmount, 0)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					phase = INTERRUPTED
					continue
				}
				phase = PHASE_SELL

			case PHASE_SELL:
				logger.Get().Infof("[Arbitrage] %s: PHASE_SELL", logLabel)
				targetPrice := o.BuySideRate() * (1 + expectedProfitRate)
				arbitrageTimeLimit := time.Now().Add(time.Minute * 240)
				sellSideTradeFeeRate, err := s.PrivateResourceRepository.TradeFeeRate(o.SellSide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
				if err != nil {
					logger.Get().Errorf("[Error] %s", err)
					continue
				}
				orderFee := sellSideTradeFeeRate.TakerFee
				amount := orderedAmount * (1 - orderFee - 0.0002)
			PhaseSellLoop:
				for {
					sellBoard, err := s.PublicResourceRepository.Board(o.SellSide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
					if err != nil {
						logger.Get().Errorf("[Error] %s", err)
						continue
					}
					bestSellPrice := sellBoard.BestBidPrice()
					if targetPrice <= bestSellPrice || !arbitrageTimeLimit.After(time.Now()) {
						orderNumber, err := s.PrivateResourceRepository.Order(
							o.SellSide(),
							o.BuySidePair().Trading, o.BuySidePair().Settlement,
							models.Bid, bestSellPrice, amount)
						if err != nil {
							logger.Get().Errorf("[Error] %s", err)
							continue
						}
						orderTimeLimit := time.Now().Add(time.Minute * 10)
						fillCheckTicker := time.NewTicker(time.Second * 3)
						for {
							select {
							case <-fillCheckTicker.C:
								isFilled, err := s.PrivateResourceRepository.IsOrderFilled(o.SellSide(), orderNumber, "")
								if err != nil {
									logger.Get().Errorf("[Error] %s", err)
									continue
								}
								if isFilled {
									sellBoard, err := s.PublicResourceRepository.Board(o.SellSide(), o.SellSidePair().Trading, o.SellSidePair().Settlement)
									if err != nil {
										logger.Get().Errorf("[Error] %s", err)
										continue
									}
									_ = sellBoard.BestBidPrice()
									orderedAmount = amount
									phase = COMPLETED
									break PhaseSellLoop
								}
								if !orderTimeLimit.After(time.Now()) {
									err = s.PrivateResourceRepository.CancelOrder(o.SellSide(), orderNumber, o.SellSidePair().Trading+"_"+o.SellSidePair().Settlement)
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
				logger.Get().Infof("[Arbitrage] %s arbitrage completed.", logLabel)
				break LongLoop
			}
		}
	}

	if err := s.OngoingOpps.Remove(&o); err != nil {
		logger.Get().Error(err)
		return err
	}
	return nil

}
*/
