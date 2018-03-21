# Go-Arbitrager

[![GoDoc](https://img.shields.io/badge/api-Godoc-blue.svg?style=flat-square)](https://godoc.org/github.com/fxpgr/go-arbitrager)
[![Go Report Card](https://goreportcard.com/badge/github.com/fxpgr/go-arbitrager)](https://goreportcard.com/report/github.com/fxpgr/go-arbitrager)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/fxpgr/go-arbitrager/graphs/commit-activity)

- this is Auto Trading Server implemented with golang.
- how to use document is being created now.
- You can arbitrage between Poloniex Hitbtc Huobi.


# How to Install :

```bash
go get github.com/fxpgr/go-arbitrager
```

# How to Use :

1. Save config.yaml in project dir.

```yaml
hitbtc:
  api_key: "hitbtc-api-key"
  secret_key: "hitbtc-secret-key"
poloniex:
  api_key: "poloniex-api-key"
  secret_key: "poloniex-secret-key"
huobi:
  api_key: "huobi-api-key"
  secret_key: "huobi-secret-key"
```

2. Build a docker image.

```bash
make build
```

3. Run go-arbitrager

```bash
make run
```
