package main

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/antihax/goesi/esi"
	"go.uber.org/zap"
)

func (app *app) buildMarketTree(logger *zap.Logger) {
	startTime := time.Now()
	logger = logger.Named("market_tree")
	groups, resp, err := app.esi.ESI.MarketApi.GetMarketsGroups(context.Background(), nil)
	if err != nil {
		logger.Error("error getting market groups", zap.Error(err))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var buf []byte
		resp.Body.Read(buf)
		logger.Error("error getting market groups", zap.String("status", resp.Status), zap.String("body", string(buf)))
	}

	ch := make(chan int32)
	wch := make(chan esi.GetMarketsGroupsMarketGroupIdOk)
	wg := sync.WaitGroup{}
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				groupId, ok := <-ch
				if !ok {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				gData, resp, err := app.esi.ESI.MarketApi.GetMarketsGroupsMarketGroupId(ctx, groupId, nil)
				if err != nil {
					logger.Warn("error fetching group data", zap.Error(err))
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					var buf []byte
					resp.Body.Read(buf)
					logger.Error("error getting market group data", zap.String("status", resp.Status), zap.String("body", string(buf)))
				}

				wch <- gData
			}
		}()
	}

	doneCh := make(chan struct{})
	groupMap := make(map[int32]esi.GetMarketsGroupsMarketGroupIdOk, len(groups))
	groupTree := make(map[int32][]int32)
	go func() {
		for {
			select {
			case <-doneCh:
				return
			case gData := <-wch:
				groupMap[gData.MarketGroupId] = gData
				if _, ok := groupTree[gData.ParentGroupId]; ok {
					groupTree[gData.ParentGroupId] = []int32{}
				} else {
					groupTree[gData.ParentGroupId] = append(groupTree[gData.ParentGroupId], gData.MarketGroupId)
				}
			}
		}
	}()

	for _, group := range groups {
		ch <- group
	}
	close(ch)
	wg.Wait()
	doneCh <- struct{}{}
	//app.groupMap = groupMap
	//app.groupTree = groupTree
	logger.Debug("created groups", zap.Duration("duration", time.Since(startTime)))
}
