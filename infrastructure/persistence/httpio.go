package persistence

import (
	"github.com/fxpgr/go-arbitrager/domain/repository"
	"github.com/fxpgr/go-arbitrager/infrastructure/config"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"sync"
)

func NewPublicResourceRepository(exchanges []string) repository.PublicResourceRepository {
	rep := &httpPublicClient{
		clientMap: NewPublicClientSyncMap(),
	}
	for _, v := range exchanges {
		c, err := public.NewClient(v)
		if err != nil {
			continue
		}
		rep.clientMap.Set(v, c)
	}
	return rep
}

func NewPrivateResourceRepository(mode private.ClientMode, conf *config.Config, exchanges []string) repository.PrivateResourceRepository {
	rep := &httpPrivateClient{
		clientMap: NewPrivateClientSyncMap(),
	}
	for _, v := range exchanges {
		setting := conf.Get(v)
		pc, err := private.NewClient(mode, v, setting.APIKey, setting.SecretKey)
		if err != nil {
			continue
		}
		rep.clientMap.Set(v, pc)
	}
	return rep
}

type httpPrivateClient struct {
	clientMap PrivateClientSyncMap
}

func (h *httpPrivateClient) TransferFee(exchange string) (map[string]float64, error) {
	m := h.clientMap.Get(exchange)
	return m.TransferFee()
}

func (h *httpPrivateClient) TradeFeeRates(exchange string) (map[string]map[string]private.TradeFee, error) {
	m := h.clientMap.Get(exchange)
	return m.TradeFeeRates()
}
func (h *httpPrivateClient) TradeFeeRate(exchange string, trading string, settlement string) (private.TradeFee, error) {
	m := h.clientMap.Get(exchange)
	return m.TradeFeeRate(trading, settlement)
}
func (h *httpPrivateClient) Balances(exchange string) (map[string]float64, error) {
	m := h.clientMap.Get(exchange)
	return m.Balances()
}
func (h *httpPrivateClient) CompleteBalances(exchange string) (map[string]*models.Balance, error) {
	m := h.clientMap.Get(exchange)
	return m.CompleteBalances()
}
func (h *httpPrivateClient) ActiveOrders(exchange string) ([]*models.Order, error) {
	m := h.clientMap.Get(exchange)
	return m.ActiveOrders()
}
func (h *httpPrivateClient) IsOrderFilled(exchange string, trading string, settlement string) (bool, error) {
	m := h.clientMap.Get(exchange)
	return m.IsOrderFilled(trading, settlement)
}
func (h *httpPrivateClient) Order(exchange string, trading string, settlement string,
	orderType models.OrderType, price float64, amount float64) (string, error) {
	m := h.clientMap.Get(exchange)
	return m.Order(trading, settlement, orderType, price, amount)
}
func (h *httpPrivateClient) CancelOrder(exchange string, trading string, settlement string, orderType models.OrderType, orderNumber string) error {
	m := h.clientMap.Get(exchange)
	return m.CancelOrder(trading, settlement,orderType,orderNumber)
}
func (h *httpPrivateClient) Transfer(exchange string, typ string, addr string,
	amount float64, additionalFee float64) error {
	m := h.clientMap.Get(exchange)
	return m.Transfer(typ, addr, amount, additionalFee)
}
func (h *httpPrivateClient) Address(exchange string, c string) (string, error) {
	m := h.clientMap.Get(exchange)
	return m.Address(c)
}

func NewPrivateClientSyncMap() PrivateClientSyncMap {
	return PrivateClientSyncMap{make(map[string]private.PrivateClient), new(sync.Mutex)}
}

type privateClientMap map[string]private.PrivateClient

type PrivateClientSyncMap struct {
	privateClientMap
	m *sync.Mutex
}

func (sm *PrivateClientSyncMap) Get(exchange string) private.PrivateClient {
	sm.m.Lock()
	defer sm.m.Unlock()
	cli, _ := sm.privateClientMap[exchange]
	return cli
}

func (sm *PrivateClientSyncMap) Set(exchange string, cli private.PrivateClient) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.privateClientMap[exchange] = cli
}

type httpPublicClient struct {
	clientMap PublicClientSyncMap
}

func (h *httpPublicClient) Volume(exchange string, trading string, settlement string) (float64, error) {
	m := h.clientMap.Get(exchange)
	return m.Volume(trading, settlement)
}

func (h *httpPublicClient) CurrencyPairs(exchange string) ([]models.CurrencyPair, error) {
	m := h.clientMap.Get(exchange)
	return m.CurrencyPairs()
}

func (h *httpPublicClient) Rate(exchange string, trading string, settlement string) (float64, error) {
	m := h.clientMap.Get(exchange)
	return m.Rate(trading, settlement)
}
func (h *httpPublicClient) RateMap(exchange string) (map[string]map[string]float64, error) {
	m := h.clientMap.Get(exchange)
	return m.RateMap()
}

func (h *httpPublicClient) VolumeMap(exchange string) (map[string]map[string]float64, error) {
	m := h.clientMap.Get(exchange)
	return m.VolumeMap()
}

func (h *httpPublicClient) FrozenCurrency(exchange string) ([]string, error) {
	m := h.clientMap.Get(exchange)
	return m.FrozenCurrency()
}
func (h *httpPublicClient) Board(exchange string, trading string, settlement string) (*models.Board, error) {
	m := h.clientMap.Get(exchange)
	return m.Board(trading, settlement)
}

type publicClientMap map[string]public.PublicClient

func NewPublicClientSyncMap() PublicClientSyncMap {
	return PublicClientSyncMap{make(map[string]public.PublicClient), new(sync.Mutex)}
}

type PublicClientSyncMap struct {
	publicClientMap
	m *sync.Mutex
}

func (sm *PublicClientSyncMap) Get(exchange string) public.PublicClient {
	sm.m.Lock()
	defer sm.m.Unlock()
	cli, _ := sm.publicClientMap[exchange]
	return cli
}

func (sm *PublicClientSyncMap) GetAll() map[string]public.PublicClient {
	return sm.publicClientMap
}

func (sm *PublicClientSyncMap) Set(exchange string, cli public.PublicClient) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.publicClientMap[exchange] = cli
}
