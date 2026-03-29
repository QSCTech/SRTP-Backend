package repository

import (
	"context"

	"github.com/QSCTech/SRTP-Backend/models"
	"gorm.io/gorm"
)

type RoomFilter struct {
	Keyword      *string
	SportCode    *string
	Organization *string
	TimeSlot     *string
	SkillLevel   *string
	Atmosphere   *string
	Page         int
	PageSize     int
}

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

func (r *RoomRepository) Create(ctx context.Context, room *models.Room) error {
	return r.db.WithContext(ctx).Create(room).Error
}

func (r *RoomRepository) GetByID(ctx context.Context, id uint) (*models.Room, error) {
	var room models.Room
	if err := r.db.WithContext(ctx).Preload("Owner").First(&room, id).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

func (r *RoomRepository) GetByInviteCode(ctx context.Context, code string) (*models.Room, error) {
	var room models.Room
	if err := r.db.WithContext(ctx).Where("invite_code = ?", code).First(&room).Error; err != nil {
		return nil, err
	}
	return &room, nil
}

type RoomListResult struct {
	Items []models.Room
	Total int64
}

func (r *RoomRepository) List(ctx context.Context, f RoomFilter) (*RoomListResult, error) {
	q := r.db.WithContext(ctx).Model(&models.Room{}).Preload("Owner").Where("visibility = 'public'")

	if f.Keyword != nil && *f.Keyword != "" {
		q = q.Where("name ILIKE ? OR location_text ILIKE ?", "%"+*f.Keyword+"%", "%"+*f.Keyword+"%")
	}
	if f.SportCode != nil && *f.SportCode != "" {
		q = q.Where("sport_code = ?", *f.SportCode)
	}
	if f.Organization != nil && *f.Organization != "" {
		q = q.Where("organization_text ILIKE ?", "%"+*f.Organization+"%")
	}
	if f.SkillLevel != nil && *f.SkillLevel != "" {
		q = q.Where("skill_level = ?", *f.SkillLevel)
	}
	if f.Atmosphere != nil && *f.Atmosphere != "" {
		q = q.Where("atmosphere = ?", *f.Atmosphere)
	}
	if f.TimeSlot != nil && *f.TimeSlot != "" {
		switch *f.TimeSlot {
		case "morning":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 6 AND 11")
		case "afternoon":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 12 AND 17")
		case "evening":
			q = q.Where("EXTRACT(HOUR FROM start_time AT TIME ZONE 'Asia/Shanghai') BETWEEN 18 AND 23")
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	var items []models.Room
	if err := q.Order("start_time ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, err
	}

	return &RoomListResult{Items: items, Total: total}, nil
}

func (r *RoomRepository) GetMembersByRoomID(ctx context.Context, roomID uint) ([]models.RoomMember, error) {
	var members []models.RoomMember
	if err := r.db.WithContext(ctx).Preload("User").Where("room_id = ?", roomID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

func (r *RoomRepository) GetMember(ctx context.Context, roomID, userID uint) (*models.RoomMember, error) {
	var member models.RoomMember
	if err := r.db.WithContext(ctx).Where("room_id = ? AND user_id = ?", roomID, userID).First(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *RoomRepository) CreateMember(ctx context.Context, member *models.RoomMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *RoomRepository) CreateJoinRequest(ctx context.Context, req *models.JoinRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *RoomRepository) CountActiveMembers(ctx context.Context, roomID uint) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.RoomMember{}).Where("room_id = ? AND status = 'joined'", roomID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
