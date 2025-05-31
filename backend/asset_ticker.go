package main

import (
	"context"
	"errors"
	"io"
	"maps"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/AlHeamer/brave-bpc/glue"
	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/antihax/goesi/optional"
	"go.uber.org/zap"
)

// runs an infinite loop
func (app *app) createOauthContextLoop(logger *zap.Logger) context.Context {
	var attempts int
	var pair scopeSourcePair
	for len(pair.scope) == 0 {
		pair = app.getAdminToken(logger)

		if len(pair.scope) == 0 {
			attempts++
			logger.Warn("no available tokens for admin character", zap.Int32("character_id", app.config.AdminCharacter), zap.Int("attempts", attempts))
			time.Sleep(time.Minute)
		}
	}

	return context.WithValue(context.Background(), goesi.ContextOAuth2, pair.token)
}

func (app *app) ticker(done <-chan struct{}) {
	var (
		logger         = app.logger.Named("ticker")
		ctx            = app.createOauthContextLoop(logger)
		nextCtxRefresh time.Time
	)

	bpos, bpcs, err := app.updateBlueprintInventory(ctx, logger)
	if err != nil {
		logger.Error("error updating blueprint inventory", zap.Error(err))
	} else {
		app.bpos.Overwrite(bpos)
		app.bpcs.Overwrite(bpcs)
	}

	ticker := time.NewTicker(60 * time.Minute)

	for {
		select {
		case <-done:
			logger.Info("exiting ticker loop")
			return

		case <-app.refreshToken:
			now := time.Now()
			if nextCtxRefresh.Before(now) {
				logger.Debug("refreshing token")
				ctx = app.createOauthContextLoop(logger)
				nextCtxRefresh = now.Add(10 * time.Second)
			}

		case <-ticker.C:
			// 1 minute jitter window
			switch app.runtimeConfig.environment {
			case "prod", "production":
				time.Sleep(time.Duration(rand.IntN(60)) * time.Second)
			}

			bpos, bpcs, err = app.updateBlueprintInventory(ctx, logger)
			if err != nil {
				if errors.Is(err, errErrorsExceeded) {
					time.Sleep(5 * time.Minute)
					ticker.Reset(60 * time.Minute)
				}
			} else {
				app.bpos.Overwrite(bpos)
				app.bpcs.Overwrite(bpcs)
			}
		}
	}
}

func (app *app) updateBlueprintInventory(ctx context.Context, logger *zap.Logger) (
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	error,
) {
	var (
		blueprints   esi.GetCorporationsCorporationIdBlueprints200OkList
		assets       esi.GetCorporationsCorporationIdAssets200OkList
		start        = time.Now()
		wg           sync.WaitGroup
		assetErr     error
		blueprintErr error
		hangars      []string
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		hangars = app.fetchCorpHangarNames(ctx, logger)
		_ = hangars
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		assets, assetErr = app.fetchAssets(ctx, logger)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		blueprints, blueprintErr = app.fetchBlueprints(ctx, logger)
	}()

	wg.Wait()

	if assetErr != nil {
		logger.Error("error fetching assets", zap.NamedError("asset_error", assetErr), zap.NamedError("blueprint_error", blueprintErr))
		err := assetErr
		if err == nil {
			err = blueprintErr
		}
		return nil, nil, err
	}

	var (
		assetMap        = make(map[int64]*esi.GetCorporationsCorporationIdAssets200Ok, len(assets))
		newContainerIds = make(map[int64]struct{})
		structureNames  map[int64]esi.GetUniverseStructuresStructureIdOk
		containerNames  map[int64]string
		typeNameMap     map[int32]string
		newTypeIds      []int32
		newStructureIds []int64
	)

	for _, bp := range assets {
		assetMap[bp.ItemId] = &bp
	}

	for _, bp := range blueprints {
		if _, ok := app.typeNameCache.Get(bp.TypeId); !ok {
			newTypeIds = append(newTypeIds, bp.TypeId)
		}
	}

	for _, asset := range assetMap {
		parent, ok := assetMap[asset.LocationId]
		// locationIds not in the assetMap are top-level locations, and therefore stations, structures, or undocked
		if !ok && asset.LocationType == glue.LocationType_Item || asset.LocationType == glue.LocationType_Station {
			if _, ok = structureNames[asset.ItemId]; !ok {
				newStructureIds = append(newStructureIds, asset.LocationId)
			}
			continue
		}
		if parent.LocationFlag != string(glue.LocationFlag_AssetSafety) && parent.LocationFlag != string(glue.LocationFlag_OfficeFolder) {
			if _, ok = structureNames[asset.LocationId]; !ok {
				newContainerIds[asset.LocationId] = struct{}{}
			}
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		typeNameMap = app.fetchTypeNames(ctx, logger, newTypeIds)
		for k, v := range typeNameMap {
			app.typeNameCache.Set(k, v)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		structureNames = app.fetchStructures(ctx, logger, newStructureIds)
		_ = structureNames
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		containerNames = app.fetchItemNames(ctx, logger, slices.Collect(maps.Keys(newContainerIds)))
		_ = containerNames
	}()

	bpos, bpcs := coalesceBlueprints(blueprints)
	buildItemLocationMap(blueprints)
	app.buildAssetTree(assets)

	wg.Wait()

	logger.Debug("successfully fetched blueprints", zap.Duration("duration", time.Since(start)))
	fetchBlueprintDuration.Observe(time.Since(start).Seconds())
	return bpos, bpcs, nil
}

func (app *app) fetchAssets(ctx context.Context, logger *zap.Logger) ([]esi.GetCorporationsCorporationIdAssets200Ok, error) {
	var (
		pages   int64 = 1
		attempt int   = 1
		assets  []esi.GetCorporationsCorporationIdAssets200Ok
	)
	for page := int32(1); page <= int32(pages); page++ {
		reqCtx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
		defer cancel()

		ap, resp, err := app.esi.ESI.AssetsApi.GetCorporationsCorporationIdAssets(reqCtx, app.config.AdminCorp,
			&esi.GetCorporationsCorporationIdAssetsOpts{
				Page: optional.NewInt32(page),
			})
		if err != nil {
			body := parseEsiError(err)
			logger.Error("error fetching assets", zap.Int32("page", page), zap.String("body", body), zap.Error(err))
			if attempt > 5 {
				return nil, errors.New("retries exceeded")
			}
			page--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		} else if resp.StatusCode < http.StatusOK {
			// some other error happened. print to logs
			var buf []byte
			resp.Body.Read(buf)
			logger.Warn("status not 200", zap.Int32("page", page), zap.String("status_code", resp.Status), zap.String("body", string(buf)))
			if attempt > 5 {
				return nil, errors.New("retries exceeded")
			}
			page--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		assets = append(assets, ap...)
		pages, _ = strconv.ParseInt(resp.Header.Get(headerPages), 10, 64)
	}

	return assets, nil
}

func (app *app) fetchBlueprints(ctx context.Context, logger *zap.Logger) ([]esi.GetCorporationsCorporationIdBlueprints200Ok, error) {
	var (
		pages      int64 = 1
		attempt          = 1
		blueprints []esi.GetCorporationsCorporationIdBlueprints200Ok
	)
	for page := int32(1); page <= int32(pages); page++ {
		reqCtx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
		defer cancel()

		bp, resp, err := app.esi.ESI.CorporationApi.GetCorporationsCorporationIdBlueprints(reqCtx, app.config.AdminCorp,
			&esi.GetCorporationsCorporationIdBlueprintsOpts{
				Page: optional.NewInt32(page),
			})

		if err != nil {
			body := parseEsiError(err)
			logger.Warn("error fetching blueprints", zap.Int("attempt", attempt), zap.Int32("page", page), zap.String("body", body), zap.Error(err))
			if attempt > 5 {
				return nil, errors.New("retries exceeded")
			}
			page--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		} else if resp.StatusCode < http.StatusOK {
			// some other error happened. print to logs
			var buf []byte
			resp.Body.Read(buf)
			logger.Warn("status not 200", zap.Int32("page", page), zap.String("status_code", resp.Status), zap.String("body", string(buf)))
			if attempt > 5 {
				return nil, errors.New("retries exceeded")
			}
			page--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		blueprints = append(blueprints, bp...)
		pages, _ = strconv.ParseInt(resp.Header.Get(headerPages), 10, 64)
	}

	return blueprints, nil
}

func (app *app) fetchTypeNames(ctx context.Context, logger *zap.Logger, typeIds []int32) map[int32]string {
	names := make(map[int32]string, len(typeIds))
	if len(typeIds) == 0 {
		return names
	}
	slices.Sort(typeIds)
	typeIds = slices.Compact(typeIds)

	chunks := slices.Chunk(typeIds, 1000)
	for chunk := range chunks {
		cctx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
		defer cancel()

		namePage, resp, err := app.esi.ESI.UniverseApi.PostUniverseNames(cctx, chunk, nil)
		if err != nil {
			body := parseEsiError(err)
			logger.Error("error fetching type names", zap.String("body", body), zap.Error(err))
			continue
		} else if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			logger.Error("error fetching type names", zap.String("status", resp.Status), zap.Error(err), zap.String("body", string(body)))
			continue
		}
		defer resp.Body.Close()

		for _, v := range namePage {
			if v.Category == string(glue.NameCategory_InventoryType) {
				names[v.Id] = v.Name
			}
		}
	}

	return names
}

func (app *app) fetchStructures(ctx context.Context, logger *zap.Logger, structureIds []int64) map[int64]esi.GetUniverseStructuresStructureIdOk {
	structures := make(map[int64]esi.GetUniverseStructuresStructureIdOk)
	if len(structureIds) == 0 {
		return structures
	}

	slices.Sort(structureIds)
	structureIds = slices.Compact(structureIds)

	attempt := 1
	for i, structureId := range structureIds {
		var (
			structureData esi.GetUniverseStructuresStructureIdOk
			stationData   esi.GetUniverseStationsStationIdOk
			resp          *http.Response
			err           error
		)
		cctx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
		defer cancel()

		structureType := glue.ResolveLoctionType(structureId)
		switch structureType {
		case glue.LocationType_Station:
			stationData, resp, err = app.esi.ESI.UniverseApi.GetUniverseStationsStationId(cctx, int32(structureId), nil)
		case glue.LocationType_Item:
			structureData, resp, err = app.esi.ESI.UniverseApi.GetUniverseStructuresStructureId(cctx, structureId, nil)
		case glue.LocationType_SolarSystem:
			// noop
		default:
			logger.Warn("trying to get station data from solar system or other location", zap.Int64("id", structureId), zap.String("type", string(structureType)))
			continue
		}

		if err != nil {
			body := parseEsiError(err)
			logger.Error("error fetching structure data", zap.Int64("structure_id", structureId), zap.String("body", body), zap.Error(err))
			if attempt > 5 {
				return nil
			}
			i--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		} else if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			logger.Error("error fetching structure data", zap.Int64("structre_id", structureId), zap.String("status", resp.Status), zap.Error(err), zap.String("body", string(body)))
			if attempt > 5 {
				return nil
			}
			i--
			attempt++
			time.Sleep(time.Duration(rand.IntN(1000)) * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		if structureType == glue.LocationType_Station {
			structureData = esi.GetUniverseStructuresStructureIdOk{
				Name:          stationData.Name,
				OwnerId:       stationData.Owner,
				Position:      esi.GetUniverseStructuresStructureIdPosition(stationData.Position),
				SolarSystemId: stationData.SystemId,
				TypeId:        stationData.TypeId,
			}
		}

		structures[structureId] = structureData
	}

	return structures
}

func (app *app) fetchItemNames(ctx context.Context, logger *zap.Logger, itemIds []int64) map[int64]string {
	names := make(map[int64]string, len(itemIds))
	if len(itemIds) == 0 {
		return names
	}

	slices.Sort(itemIds)
	itemIds = slices.Compact(itemIds)

	chunks := slices.Chunk(itemIds, 1000)
	for chunk := range chunks {
		cctx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
		defer cancel()

		namePage, resp, err := app.esi.ESI.AssetsApi.PostCorporationsCorporationIdAssetsNames(cctx, app.config.AdminCorp, chunk, nil)
		if err != nil {
			body := parseEsiError(err)
			logger.Error("error fetching item names", zap.String("body", body), zap.Error(err))
			continue
		} else if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			logger.Error("error fetching item names", zap.String("status", resp.Status), zap.Error(err), zap.String("body", string(body)))
			continue
		}
		defer resp.Body.Close()

		for _, v := range namePage {
			names[v.ItemId] = v.Name
		}
	}

	return names
}

func (app *app) fetchCorpHangarNames(ctx context.Context, logger *zap.Logger) []string {
	reqCtx, cancel := context.WithTimeout(ctx, esiRequestTimeout)
	defer cancel()

	divisions, resp, err := app.esi.ESI.CorporationApi.GetCorporationsCorporationIdDivisions(reqCtx, app.config.AdminCorp, nil)
	if err != nil {
		body := parseEsiError(err)
		logger.Error("error fetching corp divisions", zap.String("body", body), zap.Error(err))
		return nil
	} else if resp.StatusCode != http.StatusOK {
		// some other error happened. print to logs
		var buf []byte
		resp.Body.Read(buf)
		logger.Warn("status not 200", zap.String("status_code", resp.Status), zap.String("body", string(buf)))
		return nil
	}
	defer resp.Body.Close()

	out := make([]string, 7)
	for _, v := range divisions.Hangar {
		out[int(v.Division)-1] = v.Name
	}

	return out
}

func coalesceBlueprints(blueprints esi.GetCorporationsCorporationIdBlueprints200OkList) (
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
	map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList,
) {
	bpos := make(map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList)
	bpcs := make(map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList)

	for _, bp := range blueprints {
		var (
			m   map[int32]esi.GetCorporationsCorporationIdBlueprints200OkList
			qty int32 = 1
		)
		switch bp.Quantity {
		case -2: // BPC
			m = bpcs
		case -1: // researched BPO
			m = bpos
		default: // BPO stack > 0
			m = bpos
			qty = bp.Quantity
		}

		idx := -1
		if _, ok := m[bp.TypeId]; !ok {
			m[bp.TypeId] = esi.GetCorporationsCorporationIdBlueprints200OkList{}
		} else {
			idx = slices.IndexFunc(m[bp.TypeId], func(e esi.GetCorporationsCorporationIdBlueprints200Ok) bool {
				return sameBlueprintQuality(e, bp)
			})
		}

		if idx == -1 { // equivalent blueprint not found
			b := bp
			b.Quantity = qty
			m[bp.TypeId] = append(m[bp.TypeId], b)
		} else {
			m[bp.TypeId][idx].Quantity += qty
		}
	}

	return bpos, bpcs
}

func buildItemLocationMap(blueprints esi.GetCorporationsCorporationIdBlueprints200OkList) map[int64]esi.GetCorporationsCorporationIdBlueprints200OkList {
	locations := make(map[int64]esi.GetCorporationsCorporationIdBlueprints200OkList)
	for _, bp := range blueprints {
		if _, ok := locations[bp.LocationId]; !ok {
			locations[bp.LocationId] = esi.GetCorporationsCorporationIdBlueprints200OkList{bp}
			continue
		}
		locations[bp.LocationId] = append(locations[bp.LocationId], bp)
	}
	return locations
}

type CorpAsset struct {
	Asset    *esi.GetCorporationsCorporationIdAssets200Ok
	Children []*CorpAsset
}

func (app *app) buildAssetTree(assets []esi.GetCorporationsCorporationIdAssets200Ok) map[int64]*CorpAsset {
	m := make(map[int64]*CorpAsset, len(assets))
	for _, asset := range assets {
		_, ok := m[asset.ItemId]
		if !ok {
			m[asset.ItemId] = &CorpAsset{Asset: &asset}
		}

		_, ok = m[asset.LocationId]
		if !ok {
			m[asset.LocationId] = &CorpAsset{
				Children: []*CorpAsset{m[asset.ItemId]},
			}
		} else {

		}
	}

	return m
}
