package usecase

import (
	"fmt"
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"github.com/fxpgr/go-exchange-client/models"
	"strconv"
	"time"
)

type Arbitrager struct {
	MessageRepository         repository.MessageRepository         `inject:""`
	PublicResourceRepository  repository.PublicResourceRepository  `inject:""`
	PrivateResourceRepository repository.PrivateResourceRepository `inject:""`
	OngoingOpps               *entity.Opportunities                `inject:""`

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

func (s *Arbitrager) Trace(position models.Position, o entity.Opportunity, expectedProfitRate float64) error {
	if s.OngoingOpps.IsOngoing(&o) {
		return nil
	}
	if err := s.OngoingOpps.Set(&o); err != nil {
		logger.Get().Errorf("[Error] %s", err)
		return err
	}
	o.Print()
	messageText := fmt.Sprintf("[Scanner] margin found\n```%s```", o.MessageText())
	s.MessageRepository.Send(messageText)
	messageText = fmt.Sprintf("[Arbitrager] then I'll trace margin until its convergenced")
	s.MessageRepository.Send(messageText)

	buySideBoard, err := s.PublicResourceRepository.Board(o.BuySide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
	if err != nil {
		logger.Get().Errorf("[Error] %s", err)
		s.OngoingOpps.Remove(&o)
		return err
	}
	initialBuySidePrice := buySideBoard.BestAskPrice()

	sellSideBoard, err := s.PublicResourceRepository.Board(o.SellSide(), o.SellSidePair().Trading, o.SellSidePair().Settlement)
	if err != nil {
		logger.Get().Errorf("[Error] %s", err)
		s.OngoingOpps.Remove(&o)
		return err
	}
	initialSellSidePrice := sellSideBoard.BestBidPrice()

	time.Sleep(3)
	for {
		buySideBoard, err := s.PublicResourceRepository.Board(o.BuySide(), o.BuySidePair().Trading, o.BuySidePair().Settlement)
		if err != nil {
			logger.Get().Errorf("[Error] %s", err)
			time.Sleep(3* time.Second)
			continue
		}
		bestBuyPrice := buySideBoard.BestAskPrice()

		sellSideBoard, err := s.PublicResourceRepository.Board(o.SellSide(), o.SellSidePair().Trading, o.SellSidePair().Settlement)
		if err != nil {
			logger.Get().Errorf("[Error] %s", err)
			time.Sleep(3* time.Second)
			continue
		}
		bestSellPrice := sellSideBoard.BestBidPrice()
		if bestSellPrice/bestBuyPrice < 1+expectedProfitRate/4 {

			log := fmt.Sprintf("--------------------Convergence--------------------\n")
			log += fmt.Sprintf("Buyside    %4s-%4s On %8s\n", o.BuySidePair().Trading, o.BuySidePair().Settlement, o.BuySide())
			log += fmt.Sprintf("Sellside   %4s-%4s On %8s\n", o.SellSidePair().Trading, o.SellSidePair().Settlement, o.SellSide())
			log += fmt.Sprintf("Buyside  : %16s -> %16s\n", strconv.FormatFloat(initialBuySidePrice, 'f', 16, 64),strconv.FormatFloat(bestBuyPrice, 'f', 16, 64) )
			log += fmt.Sprintf("Sellside : %16s -> %16s\n", strconv.FormatFloat(initialSellSidePrice, 'f', 16, 64), strconv.FormatFloat(bestSellPrice, 'f', 16, 64))
			log += fmt.Sprintf("---------------------------------------------------")
			messageText := fmt.Sprintf("[Arbitrager] convergence found\n```%s```", log)
			s.MessageRepository.Send(messageText)

			logger.Get().Infof("--------------------Convergence--------------------")
			logger.Get().Infof("Buyside    %4s-%4s On %8s", o.BuySidePair().Trading, o.BuySidePair().Settlement, o.BuySide())
			logger.Get().Infof("Sellside   %4s-%4s On %8s", o.SellSidePair().Trading, o.SellSidePair().Settlement, o.SellSide())
			logger.Get().Infof("Buyside  : %16s -> %16s", strconv.FormatFloat(initialBuySidePrice, 'f', 16, 64),strconv.FormatFloat(bestBuyPrice, 'f', 16, 64) )
			logger.Get().Infof("Sellside : %16s -> %16s", strconv.FormatFloat(initialSellSidePrice, 'f', 16, 64), strconv.FormatFloat(bestSellPrice, 'f', 16, 64))
			logger.Get().Infof("---------------------------------------------------")
			break
		}
		time.Sleep(15* time.Second)
	}

	if err := s.OngoingOpps.Remove(&o); err != nil {
		logger.Get().Error(err)
		return err
	}
	return nil
}

func (s *Arbitrager) Arbitrage(position models.Position, o entity.Opportunity, expectedProfitRate float64) error {
	if s.OngoingOpps.IsOngoing(&o) {
		logger.Get().Infof("duplicated %v",o)
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
				amount := o.TradingAmount() * (1 - orderFee - 0.0002)
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
