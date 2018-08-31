package config

import "testing"

func TestConfig(t *testing.T) {
	c := &Config{}
	exchanges := []string{"poloniex", "hitbtc", "bitflyer", "huobi", "lbank"}
	for _, e := range exchanges {
		_ = c.Get(e)
	}
}
