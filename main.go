package main

import (
	"fmt"
	"github.com/facebookgo/inject"
	"github.com/fxpgr/go-arbitrager/application/usecase"
	"github.com/fxpgr/go-arbitrager/domain/entity"
	"github.com/fxpgr/go-arbitrager/infrastructure/config"
	"github.com/fxpgr/go-arbitrager/infrastructure/logger"
	"github.com/fxpgr/go-arbitrager/infrastructure/persistence"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/models"
	"github.com/urfave/cli"
	"gopkg.in/mgo.v2"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"
)

func help_and_exit() {
	fmt.Fprintf(os.Stderr, "%s config.yml\n", os.Args[0])
	os.Exit(1)
}

func main() {
	go func() {
		logger.Get().Debug(http.ListenAndServe("localhost:6060", nil))
	}()
	app := cli.NewApp()
	app.Name = "go-arbitrager"
	app.Usage = "arbitrage bot for dead of gold"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "mode, m",
			Value: "cli",
			Usage: "display mode",
		},
		cli.StringFlag{
			Name:  "config, c",
			Value: "config.yml",
			Usage: "config path",
		},
	}
	session, _ := mgo.Dial("mongo:27017")
	historyRepository := &persistence.HistoryRepositoryMgo{
		DB: session.DB("arbitrager"),
	}
	app.Action = func(c *cli.Context) error {
		if c.String("mode") == "lab" {
		} else if c.String("mode") == "test" {
			configPath := c.String("config")
			exchanges := []string{"lbank", "kucoin"}
			conf := config.ReadConfig(configPath)

			var scanner usecase.Scanner
			var arbitrager usecase.Arbitrager

			var g inject.Graph
			err := g.Provide(
				&inject.Object{Value: persistence.NewPublicResourceRepository(exchanges)},
				&inject.Object{Value: persistence.NewPrivateResourceRepository(private.PROJECT, conf, exchanges)},
				&inject.Object{Value: historyRepository},
				&inject.Object{Value: persistence.NewSlackClient(conf.Slack.APIToken, conf.Slack.Channel, logger.Get())},
				&inject.Object{Value: entity.NewFrozenCurrencySyncMap()},
				&inject.Object{Value: entity.NewExchangeSymbolSyncMap()},
				&inject.Object{Value: entity.NewRateSyncMap()},
				&inject.Object{Value: entity.NewVolumeSyncMap()},
				&inject.Object{Value: entity.NewOpportunities()},
				&inject.Object{Value: usecase.NewArbitragePairs()},
				&inject.Object{Value: &arbitrager},
				&inject.Object{Value: &scanner},
			)
			if err != nil {
				panic(err)
			}
			if err := g.Populate(); err != nil {
				panic(err)
			}
			err = scanner.RegisterTrianglePairs(exchanges)
			if err != nil {
				panic(err)
			}
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] triangle-arbitrage scan mode"))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] expected profit rate:%f", 0.003))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] I'll watch exchanges %v", exchanges))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] %d pairs registered", len(scanner.TriangleArbitrageTriples.Get())))
			scanner.MessageRepository.Send("[Scanner] scan started")

			scanner.SyncRate(exchanges)
			tick := time.NewTicker(15 * time.Second)
			func (){

			}()
			for {
				select {
				case <-tick.C:
					opps, err := scanner.TriangleOpportunities(0.003)
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					err = arbitrager.TraceTriangle(opps.GetAll(), 0.003)
					if err != nil {
						logger.Get().Error(err)
						continue
					}
				}
			}
		} else {
			configPath := c.String("config")
			exchanges := []string{"poloniex", "hitbtc", "huobi", "lbank", "kucoin"}
			conf := config.ReadConfig(configPath)

			var scanner usecase.Scanner
			var arbitrager usecase.Arbitrager

			var g inject.Graph
			err := g.Provide(
				&inject.Object{Value: persistence.NewPublicResourceRepository(exchanges)},
				&inject.Object{Value: persistence.NewPrivateResourceRepository(private.PROJECT, conf, exchanges)},
				&inject.Object{Value: historyRepository},
				&inject.Object{Value: persistence.NewSlackClient(conf.Slack.APIToken, conf.Slack.Channel, logger.Get())},
				&inject.Object{Value: entity.NewFrozenCurrencySyncMap()},
				&inject.Object{Value: entity.NewExchangeSymbolSyncMap()},
				&inject.Object{Value: entity.NewRateSyncMap()},
				&inject.Object{Value: entity.NewVolumeSyncMap()},
				&inject.Object{Value: entity.NewOpportunities()},
				&inject.Object{Value: usecase.NewArbitragePairs()},
				&inject.Object{Value: &arbitrager},
				&inject.Object{Value: &scanner},
			)
			if err != nil {
				panic(err)
			}
			if err := g.Populate(); err != nil {
				panic(err)
			}
			err = scanner.RegisterExchanges(exchanges)
			if err != nil {
				panic(err)
			}
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] swing-arbitrage scan mode"))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] expected profit rate:%f", 0.003))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] I'll watch exchanges %v", exchanges))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] %d pairs registered", len(scanner.TriangleArbitrageTriples.Get())))
			scanner.MessageRepository.Send(fmt.Sprintf("[Scanner] scan started"))
			scanner.SyncRate(exchanges)
			err = scanner.FilterCurrency([]string{"ETH"})
			if err != nil {
				panic(err)
			}
			scanner.MessageRepository.Send("[Scanner] filtered with ETH")
			tick := time.NewTicker(15 * time.Second)
			for {
				select {
				case <-tick.C:
					opps, err := scanner.Opportunities(0.01)
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					for _, o := range opps.GetAll() {
						go arbitrager.Trace(models.Long, o, 0.01)
					}
				}
			}
		}

		return nil
	}
	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
	return
}
