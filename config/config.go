package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
)

type KeySetting struct {
	APIKey    string `yaml:"api_key"`
	SecretKey string `yaml:"secret_key"`
}

type Config struct {
	Hitbtc   KeySetting `yaml:"hitbtc"`
	Poloniex KeySetting `yaml:"poloniex"`
	Bitflyer KeySetting `yaml:"bitflyer"`
	Huobi KeySetting    `yaml:"huobi"`
}

func (c *Config) Get(exchange string) KeySetting {
	switch exchange {
	case "hitbtc":
		return c.Hitbtc
	case "poloniex":
		return c.Poloniex
	case "bitflyer":
		return c.Bitflyer
	case "huobi":
		return c.Huobi
	}
	return KeySetting{}
}

func ReadConfig(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		panic(fmt.Sprintf("failed to open config: %s", err))
	}
	defer f.Close()

	return ReadConfigReader(f)
}

func ReadConfigReader(reader io.Reader) *Config {
	bs, err := ioutil.ReadAll(reader)
	if err != nil {
		panic(fmt.Sprintf("failed to read config: %s", err))
	}

	var config Config
	err = yaml.Unmarshal(bs, &config)
	if err != nil {
		panic(fmt.Sprintf("failed to parse config: %s", err))
	}

	return &config
}
