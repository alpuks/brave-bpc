package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/AlHeamer/brave-bpc/sqlparams"
	"go.uber.org/zap"
)

const (
	headerContentType = "Content-Type"
	headerContentJson = "application/json"
)

func httpError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Add(headerContentType, headerContentJson)
	http.Error(w, fmt.Sprintf(`{code:%d,msg:"%s"}`, statusCode, message), statusCode)
}

func httpWrite(w http.ResponseWriter, data any) {
	buf, err := json.Marshal(data)
	if err != nil {
		httpError(w, "error marshalling json", http.StatusInternalServerError)
		return
	}

	w.Header().Add(headerContentType, headerContentJson)
	w.Write(buf)
}

type postRequisitionOrderRequest struct {
	Blueprints []requestedBlueprint
}

type requestedBlueprint struct {
	TypeId             int32
	Name               string
	Runs               int16
	MaterialEfficiency int8
	TimeEfficiency     int8
	Any                bool
}

type requisitionOrder struct {
	CharacterId int32
	Blueprints  []requestedBlueprint
	Status      requisitionStatus
	PublicNotes string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type requisitionStatus int8

const (
	requisitionStatus_Open      requisitionStatus = iota
	requisitionStatus_Closed    requisitionStatus = iota
	requisitionStatus_Completed requisitionStatus = iota
	requisitionStatus_Rejected  requisitionStatus = iota
)

var requisitionStauts_name = map[requisitionStatus]string{
	requisitionStatus_Open:      "open",
	requisitionStatus_Closed:    "closed",
	requisitionStatus_Completed: "completed",
	requisitionStatus_Rejected:  "rejected",
}

func (app *app) createApiHandlers(mux *http.ServeMux) {
	mux.Handle("GET /api/blueprints", app.authMiddleware(http.HandlerFunc(app.getBlueprints), authLevel_Authorized))

	mux.Handle("POST /api/requisition/", app.authMiddleware(http.HandlerFunc(app.postRequisitionOrder), authLevel_Authorized))
	mux.Handle("GET /api/requisition/{$}", app.authMiddleware(http.HandlerFunc(app.listRequisitionOrders), authLevel_Authorized))
	mux.Handle("GET /api/requisition/{id}", app.authMiddleware(http.HandlerFunc(app.getRequisitionOrder), authLevel_Authorized))

	mux.Handle("PATCH /api/requisition/{id}/lock", app.authMiddleware(http.HandlerFunc(app.patchRequisitionOrder), authLevel_Worker))
	mux.Handle("PATCH /api/requisition/{id}/unlock", app.authMiddleware(http.HandlerFunc(app.patchRequisitionOrder), authLevel_Worker))
	mux.Handle("PATCH /api/requisition/{id}/complete", app.authMiddleware(http.HandlerFunc(app.patchRequisitionOrder), authLevel_Worker))
	mux.Handle("PATCH /api/requisition/{id}/reject", app.authMiddleware(http.HandlerFunc(app.patchRequisitionOrder), authLevel_Worker))
}

func (app *app) getBlueprints(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		httpError(w, "not logged in", http.StatusUnauthorized)
		return
	}
	httpWrite(w, app.bpcs)
}

func (app *app) patchRequisitionOrder(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		httpError(w, "not logged in", http.StatusUnauthorized)
		return
	}

	characterId := s.Values[sessionCharId].(int32)

	if characterId != app.config.AdminCharacter && s.Values[sessionLevel].(int8) < int8(authLevel_Admin) {
		httpError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	reqId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpError(w, "invalid requisition", http.StatusBadRequest)
		return
	}

	path := strings.Split(r.URL.Path, ":")
	action := path[len(path)-1]
	notes := r.Form.Get("notes")

	logger := app.logger.Named("api").With(zap.Int64("id", reqId), zap.String("action", action))
	logger.Debug("patch requisition", zap.String("notes", notes))
	params := sqlparams.New()

	switch action {
	case "lock":
		if _, ok := app.requisitionLocks[reqId]; ok {
			logger.Debug("attempting to lock pre-locked requisition")
			httpError(w, "resource locked", http.StatusBadRequest)
			return
		}
		app.requisitionLocks[reqId] = characterId
		return

	case "unlock":
		if _, ok := app.requisitionLocks[reqId]; !ok {
			logger.Debug("attempting to unlock requisition that is not locked")
			httpError(w, "resource not locked", http.StatusBadRequest)
			return
		}
		delete(app.requisitionLocks, reqId)
		return

	case "complete":
		lockUser, ok := app.requisitionLocks[reqId]
		if !ok {
			logger.Debug("attempting to complete requisition that is not locked", zap.Int32("locked_by", lockUser), zap.Int32("user", characterId))
			httpError(w, "resource not locked", http.StatusBadRequest)
			return
		}
		if lockUser != characterId {
			logger.Debug("can't complete requisition, locked by other user", zap.Int32("locked_by", lockUser), zap.Int32("user", characterId))
			httpError(w, "can't complete requisition, locked by other user", http.StatusBadRequest)
			return
		}

		app.dao.db.Exec(`UPDATE requisition_order SET status=`+params.AddParam(requisitionStatus_Completed)+` public_notes="`+params.AddParam(notes)+`" updated_at=NOW() WHERE id=`+params.AddParam(reqId), params...)

	case "reject":
		lockUser, ok := app.requisitionLocks[reqId]
		if !ok {
			logger.Debug("attempting to complete requisition that is not locked")
			httpError(w, "resource not locked", http.StatusBadRequest)
			return
		}
		if lockUser != characterId {
			logger.Debug("can't reject requisition, locked by other user", zap.Int32("locked_by", lockUser), zap.Int32("user", characterId))
			httpError(w, "can't reject requisition, locked by other user", http.StatusBadRequest)
			return
		}

		app.dao.db.Exec(`UPDATE requisition_order SET status=`+params.AddParam(requisitionStatus_Rejected)+` public_notes="`+params.AddParam(notes)+`" updated_at=NOW() WHERE id=`+params.AddParam(reqId), params...)
	}
}

func (app *app) getRequisitionOrder(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		httpError(w, "not logged in", http.StatusUnauthorized)
		return
	}

	reqId, err := strconv.ParseInt(r.PathValue("requsition_id"), 10, 64)
	if err != nil {
		httpError(w, "invalid requisition", http.StatusBadRequest)
		return
	}
	logger := app.logger.Named("api").With(zap.Int64("id", reqId))
	// TODO: get req from db
	logger.Debug("get requisition order")
}

func (app *app) listRequisitionOrders(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		httpError(w, "not logged in", http.StatusUnauthorized)
		return
	}

	reqId, err := strconv.ParseInt(r.PathValue("requsition_id"), 10, 64)
	if err != nil {
		httpError(w, "invalid requisition", http.StatusBadRequest)
		return
	}
	logger := app.logger.Named("api").With(zap.Int64("id", reqId))
	// TODO: get req from db
	logger.Debug("get requisition order")
}

// Create a new requisition order
// expects a postRequisitionOrderRequest in the body
func (app *app) postRequisitionOrder(w http.ResponseWriter, r *http.Request) {
	s, _ := app.session.Get(r, cookieName)
	if s.IsNew {
		httpError(w, "not logged in", http.StatusUnauthorized)
		return
	}

	// TODO: check user is in a valid corp

	characterId := s.Values[sessionCharId].(int32)
	logger := app.logger.Named("api").With(zap.Int32("character_id", characterId))

	var buf []byte
	if _, err := r.Body.Read(buf); err != nil {
		logger.Error("error reading body", zap.Error(err))
		httpError(w, "error reading request", http.StatusInternalServerError)
		return
	}

	var bpReq postRequisitionOrderRequest
	if err := json.Unmarshal(buf, &bpReq); err != nil {
		logger.Error("error unmarshalling json")
		httpError(w, "error reading request", http.StatusInternalServerError)
		return
	}

	if len(bpReq.Blueprints) == 0 {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := app.dao.createRequisition(characterId, buf); err != nil {
		httpError(w, "error creating requisition", http.StatusInternalServerError)
		return
	}
}
