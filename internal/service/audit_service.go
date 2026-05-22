package service

import (
	"context" 
	"errors"  
	"fmt"   
	"regexp"  
	"strings" 
	"sync"    
	"time"    

	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/QSCTech/SRTP-Backend/pkg/utils"
	"go.uber.org/zap" 
)

var ErrRiskWordDetected = errors.New("risk word detected")


type AuditService struct {
	auditRepo    *repository.AuditRepository   
	userRepo     *repository.UserRepository    
	riskWordRepo *repository.RiskWordRepository 
	detector     *RiskWordDetector           
	log          *zap.Logger                    
}

func NewAuditService(
	auditRepo *repository.AuditRepository,
	userRepo *repository.UserRepository,
	riskWordRepo *repository.RiskWordRepository,
	log *zap.Logger,
) *AuditService {
	return &AuditService{
		auditRepo:    auditRepo,
		userRepo:     userRepo,
		riskWordRepo: riskWordRepo,
		detector:     NewRiskWordDetector(), 
		log:          log,
	}
}

type SubmitProfileInput struct {
	Nickname string
	Bio      string
}

func (s *AuditService) ValidateProfile(nickname, bio string) error {

	nickname = utils.NormalizeWhitespace(nickname)
	bio = utils.NormalizeWhitespace(bio)


	if nickname == "" {
		return fmt.Errorf("nickname is required")
	}

	hasRisk, field, pattern := s.detector.Detect(nickname, bio)
	if hasRisk {

		return fmt.Errorf("%w: %s contains risk word matched by rule %q", ErrRiskWordDetected, field, pattern)
	}

	return nil 
}

func (s *AuditService) SubmitProfile(ctx context.Context, userID uint, input SubmitProfileInput) (*models.UserProfileAudit, error) {

	if err := s.ValidateProfile(input.Nickname, input.Bio); err != nil {
		return nil, err
	}

	audit := &models.UserProfileAudit{
		UserID:            userID,
		SubmittedNickname: utils.NormalizeWhitespace(input.Nickname),
		SubmittedBio:      utils.NormalizeWhitespace(input.Bio),
		Status:            "approved", 
		ReviewedBy:        nil,    
		ReviewedAt:        nil,    
	}


	if err := s.auditRepo.Create(ctx, audit); err != nil {
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}


	if err := s.auditRepo.UpdateUserProfile(ctx, userID, audit.SubmittedNickname, audit.SubmittedBio); err != nil {
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}

	return audit, nil
}


func (s *AuditService) GetLatestAudit(ctx context.Context, userID uint) (*models.UserProfileAudit, error) {
	return s.auditRepo.GetLatestByUserID(ctx, userID)
}


func (s *AuditService) InitRiskWords(ctx context.Context) error {
	return s.ReloadRiskWords(ctx)
}


func (s *AuditService) ReloadRiskWords(ctx context.Context) error {

	words, err := s.riskWordRepo.GetAllEnabled(ctx)
	if err != nil {
		return fmt.Errorf("fetch risk words from db: %w", err)
	}


	patterns := make([]string, 0, len(words))
	for _, w := range words {
		patterns = append(patterns, w.Pattern)
	}


	if err := s.detector.LoadPatterns(patterns); err != nil {
		return fmt.Errorf("compile risk word patterns: %w", err)
	}

	s.log.Info("risk words reloaded", zap.Int("count", len(patterns)))
	return nil
}


func (s *AuditService) StartPeriodicReload(ctx context.Context, interval time.Duration) {
	go func() { 
		ticker := time.NewTicker(interval)
		defer ticker.Stop()             

		for { 
			select { 
			case <-ticker.C: 
				if err := s.ReloadRiskWords(ctx); err != nil {
					s.log.Error("periodic reload risk words failed", zap.Error(err))
				}
			case <-ctx.Done(): 
				s.log.Info("risk word reload goroutine stopped")
				return 
			}
		}
	}()
}


type RiskWordDetector struct {
	patterns []*regexp.Regexp 
	mu       sync.RWMutex   
}


func NewRiskWordDetector() *RiskWordDetector {
	return &RiskWordDetector{
		patterns: make([]*regexp.Regexp, 0),
	}
}


func (d *RiskWordDetector) LoadPatterns(patterns []string) error {
	compiled := make([]*regexp.Regexp, 0, len(patterns)) 

	for _, p := range patterns {
		if strings.TrimSpace(p) == "" {
			continue 
		}

		re, err := regexp.Compile(p)
		if err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}


	d.mu.Lock()
	d.patterns = compiled
	d.mu.Unlock()

	return nil
}

func (d *RiskWordDetector) Detect(nickname, bio string) (hasRisk bool, field string, pattern string) {

	if nickname == "" && bio == "" {
		return false, "", ""
	}


	d.mu.RLock()
	defer d.mu.RUnlock() 


	for _, re := range d.patterns {
		if re.MatchString(nickname) {
			return true, "nickname", re.String()
		}
	}

	for _, re := range d.patterns {
		if re.MatchString(bio) {
			return true, "bio", re.String()
		}
	}

	return false, "", ""
}
