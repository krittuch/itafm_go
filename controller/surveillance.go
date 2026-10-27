package controller

import (
	"context"
	"database/sql"

	"aerothai/itafm/model"
	"aerothai/itafm/repository"
)

type SurveillanceInterface interface {
	InsertOrUpdateSurveillance(*model.AODSSurveillance) bool
	InsertOrUpdateSurveillanceBatch([]*model.AODSSurveillance) bool
	GetSurveillancePosition(context.Context, []string) (*model.SurveillancePosition, error)
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

func (s *SurveillanceController) InsertOrUpdateSurveillanceBatch(surveillance []*model.AODSSurveillance) bool {
	repo := repository.NewSurveillanceRepository(s.DB)
	return repo.InsertOrUpdateSurveillanceBatch(surveillance)
}

func (s *SurveillanceController) GetSurveillancePosition(ctx context.Context, callsigns []string) (*model.SurveillancePosition, error) {
	repo := repository.NewSurveillanceRepository(s.DB)
	return repo.GetSurveillancePosition(ctx, callsigns)
}
