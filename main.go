package main

import (
	"fmt"
	"os"
	"time"
	"github.com/fxpgr/go-arbitrager/logger"
	"github.com/urfave/cli"
	"github.com/fxpgr/go-arbitrager/arbitrager"
	"github.com/fxpgr/go-exchange-client/api/private"
	"github.com/fxpgr/go-exchange-client/models"
)

func help_and_exit() {
	fmt.Fprintf(os.Stderr, "%s config.yml\n", os.Args[0])
	os.Exit(1)
}

func main() {
	app := cli.NewApp()
	app.Name = "go-arbitrager"
	app.Usage = "arbitrage bot for dead of gold"
	app.Flags = []cli.Flag {
		cli.StringFlag{
			Name:        "mode, m",
			Value:       "cli",
			Usage:       "display mode",
		},
		cli.StringFlag{
			Name:        "config, c",
			Value:       "config.yml",
			Usage:       "config path",
		},
	}
	app.Action = func(c *cli.Context) error {
		configPath := c.String("config")
		if c.String("mode")=="cui" {
			fmt.Println("not implemented")

		}else{
			simpleArbitrager := arbitrager.NewSimpleArbitrager(configPath,0.01)
			err := simpleArbitrager.RegisterExchanges(private.TEST, []string{"poloniex", "hitbtc", "huobi"})
			if err != nil {
				panic(err)
			}
			simpleArbitrager.SyncRate()
			//simpleArbitrager.WatchRate()
			time.Sleep(time.Second*20)
			tick := time.NewTicker(15*time.Second)
			for {
				select{
				case <- tick.C:
					opps, err := simpleArbitrager.Opportunities()
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					o, err := opps.HighestDifOpportunity()
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					err = simpleArbitrager.SingleLegArbitrage(models.Long, &o)
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					/*
					filterdO,err := opportunities.BuySideFilter("hitbtc")
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					opp,err := filterdO.HighestDifOpportunity()
					if err != nil {
						logger.Get().Error(err)
						continue
					}
					err = simpleArbitrager.Arbitrage(opp)
					if err != nil {
						logger.Get().Error(err)
						continue
					}*/
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
