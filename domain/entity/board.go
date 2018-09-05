package entity

import (
	"fmt"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"strconv"
)

type ComputableBoardTriangleArray struct {
	Arr []ComputableBoard
}

func (b *ComputableBoardTriangleArray) Set(c ComputableBoard) {
	b.Arr = append(b.Arr, c)
}

func (b *ComputableBoardTriangleArray) SpreadPrices() (buyPrice float64, sellPrice float64, err error) {
	buyComputableBoardArray, sellComputableBoardArray, err := b.DivideArrayBySide()
	if err != nil {
		return 0, 0, errors.WithStack(err)
	}
	buyPrice = 1
	sellPrice = 1
	for _, bc := range buyComputableBoardArray {
		buyPrice *= bc.BestAskPrice()
	}
	for _, bc := range sellComputableBoardArray {
		sellPrice *= bc.BestBidPrice()
	}
	return buyPrice, sellPrice, nil
}

func (b *ComputableBoardTriangleArray) GetTradeAmount() (float64, string, error) {
	amountArray := make([]float64, 0)
	buyComputableBoardArray, sellComputableBoardArray, err := b.DivideArrayBySide()
	if err != nil {
		return 0,"", err
	}
	duplicateSide, err := b.DuplicateSide()
	if err != nil {
		return 0,"", err
	}
	pivotCurrency, err := b.PivotCurrency()
	if err != nil {
		return 0,"", err
	}

	for _, cb := range b.Arr {
		if cb.Item.Op == "BUY" {
			if cb.Item.Op == duplicateSide && cb.Item.Settlement != pivotCurrency {
				pivotBasedBoard, err := buyComputableBoardArray[0].Multiply(&buyComputableBoardArray[1], pivotCurrency)
				if err != nil {
					return 0,"", errors.WithStack(err)
				}
				amount := pivotBasedBoard.BestAskAmount() * cb.BestAskPrice()
				amountArray = append(amountArray, amount)
			} else {
				amount := cb.BestAskPrice() * cb.BestAskAmount()
				amountArray = append(amountArray, amount)
			}
		}
		if cb.Item.Op == "SELL" {
			if cb.Item.Op == duplicateSide && cb.Item.Settlement != pivotCurrency {
				pivotBasedBoard, err := sellComputableBoardArray[0].Multiply(&sellComputableBoardArray[1], pivotCurrency)
				if err != nil {
					return 0,"", errors.WithStack(err)
				}
				amount := pivotBasedBoard.BestBidAmount() * cb.BestBidPrice()
				amountArray = append(amountArray, amount)
			} else {
				amount := cb.BestBidPrice() * cb.BestBidAmount()
				amountArray = append(amountArray, amount)
			}
		}
	}
	return min(amountArray),pivotCurrency,nil
}

func (b *ComputableBoardTriangleArray) GenerateText() (messageText []string, err error) {
	buyComputableBoardArray, sellComputableBoardArray, nil := b.DivideArrayBySide()
	if err != nil {
		return []string{}, err
	}
	duplicateSide, err := b.DuplicateSide()
	if err != nil {
		return []string{}, err
	}
	pivotCurrency, err := b.PivotCurrency()
	if err != nil {
		return []string{}, err
	}
	amountArray := make([]float64, 0)
	for _, cb := range b.Arr {
		if cb.Item.Op == "BUY" {
			messageText = append(messageText, fmt.Sprintf("%-4s %-5s-%-5s On %-8s At %v", cb.Item.Op, cb.Item.Trading, cb.Item.Settlement, cb.Item.Exchange, strconv.FormatFloat(cb.BestAskPrice(), 'f', 16, 64)))
			if cb.Item.Op == duplicateSide && cb.Item.Settlement != pivotCurrency {
				pivotBasedBoard, err := buyComputableBoardArray[0].Multiply(&buyComputableBoardArray[1], pivotCurrency)
				if err != nil {
					return []string{}, errors.WithStack(err)
				}
				amount := pivotBasedBoard.BestAskAmount() * cb.BestAskPrice()
				amountArray = append(amountArray, amount)
				messageText = append(messageText, fmt.Sprintf("BuyAmount                     : %8s%5s", strconv.FormatFloat(amount, 'f', 16, 64), pivotCurrency))
			} else {
				amount := cb.BestAskPrice() * cb.BestAskAmount()
				amountArray = append(amountArray, amount)
				messageText = append(messageText, fmt.Sprintf("BuyAmount                     : %8s%5s", strconv.FormatFloat(amount, 'f', 16, 64), cb.Item.Settlement))
			}
		}
		if cb.Item.Op == "SELL" {
			messageText = append(messageText, fmt.Sprintf("%-4s %-5s-%-5s On %-8s At %v", cb.Item.Op, cb.Item.Trading, cb.Item.Settlement, cb.Item.Exchange, strconv.FormatFloat(cb.BestBidPrice(), 'f', 16, 64)))
			if cb.Item.Op == duplicateSide && cb.Item.Settlement != pivotCurrency {
				pivotBasedBoard, err := sellComputableBoardArray[0].Multiply(&sellComputableBoardArray[1], pivotCurrency)
				if err != nil {
					return []string{}, errors.WithStack(err)
				}
				amount := pivotBasedBoard.BestBidAmount() * cb.BestBidPrice()
				amountArray = append(amountArray, amount)
				messageText = append(messageText, fmt.Sprintf("SellAmount                    : %8s%5s", strconv.FormatFloat(amount, 'f', 16, 64), pivotCurrency))
			} else {
				amount := cb.BestBidPrice() * cb.BestBidAmount()
				amountArray = append(amountArray, amount)
				messageText = append(messageText, fmt.Sprintf("SellAmount                    : %8s%5s", strconv.FormatFloat(amount, 'f', 16, 64), cb.Item.Settlement))
			}
		}
	}

	messageText = append(messageText, fmt.Sprintf("TradeAmount                   : %8s%5s", strconv.FormatFloat(min(amountArray), 'f', 16, 64), pivotCurrency))
	return messageText, nil
}

func min(a []float64) float64 {
	min := a[0]
	for _, i := range a {
		if i < min {
			min = i
		}
	}
	return min
}

func (b *ComputableBoardTriangleArray) PivotCurrency() (string, error) {
	counter := make(map[string]int)
	for _, cb := range b.Arr {
		counter[cb.Item.Settlement] += 1
	}
	for k, c := range counter {
		if c == 2 {
			return k, nil
		}
	}
	return "", errors.Errorf("ComputableBoardTriangleArray cannot find pivot currency %s %v", len(b.Arr), counter)
}

func (b *ComputableBoardTriangleArray) DivideArrayBySide() ([]ComputableBoard, []ComputableBoard, error) {
	if len(b.Arr) != 3 {
		return nil, nil, errors.New("computableBoardArray length is not 3")
	}
	buyAry := make([]ComputableBoard, 0)
	sellAry := make([]ComputableBoard, 0)
	for _, cb := range b.Arr {
		if cb.Item.Op == "BUY" {
			buyAry = append(buyAry, cb)
		} else {
			sellAry = append(sellAry, cb)
		}
	}
	return buyAry, sellAry, nil
}

func (b *ComputableBoardTriangleArray) DuplicateSide() (string, error) {
	if len(b.Arr) != 3 {
		return "", errors.New("computableBoardArray length is not 3")
	}
	counter := 0
	for _, cb := range b.Arr {
		if cb.Item.Op == "BUY" {
			counter += 1
		}
	}
	if counter > 1 {
		return "BUY", nil
	}
	return "SELL", nil
}

type ComputableBoard struct {
	*models.Board
	Item *Item
}

func NewComputableBoard(board models.Board, item Item) ComputableBoard {
	return ComputableBoard{&board, &item}
}

func (b *ComputableBoard) Multiply(wrapper *ComputableBoard, targetCurrency string) (*models.Board, error) {
	//  BUY  TKY  -USDT
	//	SELL TKY  -KCS   On kucoin   At 0.0032000000000000
	//	SellAmount                    : 0.1038051200000000  KCS
	//	SELL KCS  -USDT  On kucoin   At 1.4574570000000000
	//	SellAmount                    : 34.5529533189000020 USDT
	if targetCurrency == b.Item.Settlement && b.Item.Trading == wrapper.Item.Settlement {
		//currency USDT
		// b       KCS-USDT
		// wrapper TKY-KCS
		asks := make([]models.BoardOrder, 0)
		bids := make([]models.BoardOrder, 0)
		settlementRate := b.BestBidPrice()
		if b.Item.Op == "SELL" {
			settlementRate = b.BestAskPrice()
		}
		for _, boardOrder := range wrapper.Asks {
			newBoardOrder := models.BoardOrder{
				Type:   boardOrder.Type,
				Price:  boardOrder.Price,
				Amount: boardOrder.Amount / settlementRate,
			}
			asks = append(asks, newBoardOrder)
		}
		for _, boardOrder := range wrapper.Bids {
			newBoardOrder := models.BoardOrder{
				Type:   boardOrder.Type,
				Price:  boardOrder.Price,
				Amount: boardOrder.Amount / settlementRate,
			}
			bids = append(bids, newBoardOrder)
		}
		return &models.Board{
			Asks: asks, Bids: bids,
		}, nil

	} else if targetCurrency == wrapper.Item.Settlement && b.Item.Settlement == wrapper.Item.Trading {
		//currency USDT
		// wrapper KCS-USDT
		// b       TKY-KCS
		asks := make([]models.BoardOrder, 0)
		bids := make([]models.BoardOrder, 0)
		settlementRate := wrapper.BestBidPrice()
		if b.Item.Op == "SELL" {
			settlementRate = wrapper.BestAskPrice()
		}
		for _, boardOrder := range b.Asks {
			newBoardOrder := models.BoardOrder{
				Type:   boardOrder.Type,
				Price:  boardOrder.Price,
				Amount: boardOrder.Amount / settlementRate,
			}
			asks = append(asks, newBoardOrder)
		}
		for _, boardOrder := range b.Bids {
			newBoardOrder := models.BoardOrder{
				Type:   boardOrder.Type,
				Price:  boardOrder.Price,
				Amount: boardOrder.Amount / settlementRate,
			}
			bids = append(bids, newBoardOrder)
		}
		return &models.Board{
			Asks: asks, Bids: bids,
		}, nil
	}
	return nil, errors.New("mismatch currency pair")
}
