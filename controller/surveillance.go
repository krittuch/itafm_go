package controller

import (
	"database/sql"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type SurveillanceInterface interface {
	InsertOrUpdateSurveillance(*model.AODSSurveillance) bool
}

type SurveillanceController struct {
	DB *sql.DB
}

func NewSurveillanceController(db *sql.DB) *SurveillanceController {
	return &SurveillanceController{
		DB: db,
	}
}

func (s *SurveillanceController) InsertOrUpdateSurveillance(surveillance *model.AODSSurveillance) bool {
	repo := repository.NewSurveillanceRepository(s.DB)
	return repo.InsertOrUpdateSurveillance(surveillance)
}
