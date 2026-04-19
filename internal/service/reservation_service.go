package service

import (
	"context"
	"fmt"

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
)

type ReservationVenueItem struct {
	SportType  string
	CampusName string
	VenueName  string
}

type ReservationSlotItem struct {
	SlotKey   string
	StartTime string
	EndTime   string
	Available bool
	SpaceName *string
}

type ReservationPreviewInput struct {
	RoomID          uint
	SportType       string
	CampusName      string
	VenueName       string
	ReservationDate string
	StartTime       string
	EndTime         string
	BuddyCode       *string
	VenueID         *uint
	VenueSiteID     *uint
	SpaceID         *uint
	SpaceName       *string
}

type ReservationPreviewOutput struct {
	RoomID            uint
	Provider          string
	ReservationStatus string
	SportType         string
	CampusName        string
	VenueName         string
	ReservationDate   string
	StartTime         string
	EndTime           string
	BuddyCode         *string
	VenueID           *uint
	VenueSiteID       *uint
	SpaceID           *uint
	SpaceName         *string
}

type ReservationService struct {
	roomRepo        *repository.RoomRepository
	reservationRepo *repository.ReservationRepository
}

func NewReservationService(roomRepo *repository.RoomRepository, reservationRepo *repository.ReservationRepository) *ReservationService {
	return &ReservationService{roomRepo: roomRepo, reservationRepo: reservationRepo}
}

func (s *ReservationService) ListVenues(ctx context.Context, sportType, campus *string) []ReservationVenueItem {
	return nil
}

func (s *ReservationService) ListSlots(ctx context.Context, sportType, campusName, venueName, reservationDate string) ([]ReservationSlotItem, error) {
	return nil, fmt.Errorf("reservation service ListSlots not implemented")
}

func (s *ReservationService) Preview(ctx context.Context, input ReservationPreviewInput) (*ReservationPreviewOutput, error) {
	return nil, fmt.Errorf("reservation service Preview not implemented")
}

func (s *ReservationService) Submit(ctx context.Context, input ReservationPreviewInput) (*models.RoomReservation, error) {
	return nil, fmt.Errorf("reservation service Submit not implemented")
}
