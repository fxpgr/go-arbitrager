package entity

import (
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/api/public"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/pkg/errors"
	"sync"
)

func NewFrozenCurrencySyncMap() *FrozenCurrencySyncMap {
	return &FrozenCurrencySyncMap{make(map[string][]string), new(sync.Mutex)}
}

type frozenCurrencyMap map[string][]string

type FrozenCurrencySyncMap struct {
	frozenCurrencyMap
	m *sync.Mutex
}

func (sm *FrozenCurrencySyncMap) Set(exchange string, currencies []string) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.frozenCurrencyMap[exchange] = currencies
}

func (sm *FrozenCurrencySyncMap) Get(exchange string) []string {
	sm.m.Lock()
	defer sm.m.Unlock()
	currencies, _ := sm.frozenCurrencyMap[exchange]
	return currencies
}

func (sm *FrozenCurrencySyncMap) GetAll() map[string][]string {
	return sm.frozenCurrencyMap
}

func NewExchangeSymbolSyncMap() *ExchangeSymbolSyncMap {
	return &ExchangeSymbolSyncMap{make(map[string][]models.CurrencyPair), new(sync.Mutex)}
}

type exchangeSymbolMap map[string][]models.CurrencyPair

type ExchangeSymbolSyncMap struct {
	exchangeSymbolMap
	m *sync.Mutex
}

func (sm *ExchangeSymbolSyncMap) Set(exchange string, symbols []models.CurrencyPair) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.exchangeSymbolMap[exchange] = symbols
}

func (sm *ExchangeSymbolSyncMap) Get(exchange string) []models.CurrencyPair {
	sm.m.Lock()
	defer sm.m.Unlock()
	symbols, _ := sm.exchangeSymbolMap[exchange]
	return symbols
}

func (sm *ExchangeSymbolSyncMap) GetAll() map[string][]models.CurrencyPair {
	return sm.exchangeSymbolMap
}

func NewPublicClientSyncMap() *PublicClientSyncMap {
	return &PublicClientSyncMap{make(map[string]public.PublicClient), new(sync.Mutex)}
}

type publicClientMap map[string]public.PublicClient

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

func NewPrivateClientSyncMap() *PrivateClientSyncMap {
	return &PrivateClientSyncMap{make(map[string]private.PrivateClient), new(sync.Mutex)}
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

func NewRateSyncMap() *RateSyncMap {
	return &RateSyncMap{make(map[string]map[string]map[string]float64), new(sync.Mutex)}
}

type rateMap map[string]map[string]map[string]float64

type RateSyncMap struct {
	rateMap
	m *sync.Mutex
}

func (sm *RateSyncMap) Set(exchange string, rates map[string]map[string]float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.rateMap[exchange] = rates
}

func (sm *RateSyncMap) GetRate(exchange string, trading string, settlement string) (float64, error) {
	sm.m.Lock()
	defer sm.m.Unlock()
	rate, ok := sm.rateMap[exchange][trading][settlement]
	if !ok {
		return 0, errors.New("no rate")
	}
	return rate, nil
}

func NewVolumeSyncMap() *VolumeSyncMap {
	return &VolumeSyncMap{make(map[string]map[string]map[string]float64), new(sync.Mutex)}
}

type volumeMap map[string]map[string]map[string]float64

type VolumeSyncMap struct {
	volumeMap
	m *sync.Mutex
}

func (sm *VolumeSyncMap) Set(exchange string, voluemes map[string]map[string]float64) {
	sm.m.Lock()
	defer sm.m.Unlock()
	sm.volumeMap[exchange] = voluemes
}

func (sm *VolumeSyncMap) GetRate(exchange string, trading string, settlement string) (float64, error) {
	sm.m.Lock()
	defer sm.m.Unlock()
	volume, ok := sm.volumeMap[exchange][trading][settlement]
	if !ok {
		return 0, errors.New("no rate")
	}
	return volume, nil
}
