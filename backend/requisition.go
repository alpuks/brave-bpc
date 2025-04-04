package main

import "time"

type requisitionOrder struct {
	Id          int64
	CharacterId int32
	Status      requisitionStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Blueprints  []requestedBlueprint
	UpdatedBy   string
	PublicNotes string
}

type requisitionStatus int8

const (
	requisitionStatus_Open      requisitionStatus = iota
	requisitionStatus_Canceled  requisitionStatus = iota
	requisitionStatus_Completed requisitionStatus = iota
	requisitionStatus_Rejected  requisitionStatus = iota
)

var requisitionStauts_name = map[requisitionStatus]string{
	requisitionStatus_Open:      "open",
	requisitionStatus_Canceled:  "closed",
	requisitionStatus_Completed: "completed",
	requisitionStatus_Rejected:  "rejected",
}
