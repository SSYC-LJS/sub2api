package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const SubAccountStatusActive = "active"
const SubAccountStatusDisabled = "disabled"

var ErrParentAccountRequired = errors.New("parent account permission required")
var ErrSubAccountNotFound = errors.New("sub account relation not found")
var ErrSubAccountAlreadyLinked = errors.New("user is already linked to a parent account")
var ErrSubAccountSelfLink = errors.New("parent and child account cannot be the same")

// SubAccountRelation describes an active parent-child user relation and the quota
// allocated by the parent account to the child account.
type SubAccountRelation struct {
	ID                   int64
	ParentUserID         int64
	ChildUserID          int64
	AllocatedQuota       float64
	UsedQuota            float64
	WeeklyAllocatedQuota float64
	WeeklyUsedQuota      float64
	WeeklyWindowStart    *time.Time
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time

	ParentUser *User
	ChildUser  *User
}

func (r *SubAccountRelation) RemainingQuota() float64 {
	if r == nil {
		return 0
	}
	remaining := r.AllocatedQuota - r.UsedQuota
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (r *SubAccountRelation) WeeklyRemainingQuota() float64 {
	if r == nil {
		return 0
	}
	remaining := r.WeeklyAllocatedQuota - r.WeeklyUsedQuota
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (r *SubAccountRelation) TotalRemainingQuota() float64 {
	if r == nil {
		return 0
	}
	return r.WeeklyRemainingQuota() + r.RemainingQuota()
}

type SubAccountUpsertInput struct {
	ChildUserID          int64
	AllocatedQuota       float64
	WeeklyAllocatedQuota float64
}

type SubAccountRepository interface {
	ListByParent(ctx context.Context, parentUserID int64, params pagination.PaginationParams) ([]SubAccountRelation, *pagination.PaginationResult, error)
	GetActiveByParentAndChild(ctx context.Context, parentUserID, childUserID int64) (*SubAccountRelation, error)
	GetActiveByChild(ctx context.Context, childUserID int64) (*SubAccountRelation, error)
	Upsert(ctx context.Context, parentUserID int64, input SubAccountUpsertInput) (*SubAccountRelation, error)
	UpdateQuota(ctx context.Context, parentUserID, childUserID int64, allocatedQuota, weeklyAllocatedQuota float64) (*SubAccountRelation, error)
	Remove(ctx context.Context, parentUserID, childUserID int64) error
}

type SubAccountService struct {
	repo      SubAccountRepository
	userRepo  UserRepository
	usageRepo UsageLogRepository
}

func NewSubAccountService(repo SubAccountRepository, userRepo UserRepository, usageRepo UsageLogRepository) *SubAccountService {
	return &SubAccountService{repo: repo, userRepo: userRepo, usageRepo: usageRepo}
}

func (s *SubAccountService) ensureParent(ctx context.Context, parentUserID int64) (*User, error) {
	if s == nil || s.repo == nil || s.userRepo == nil {
		return nil, ErrParentAccountRequired
	}
	user, err := s.userRepo.GetByID(ctx, parentUserID)
	if err != nil {
		return nil, err
	}
	if !user.IsParentAccount {
		return nil, ErrParentAccountRequired
	}
	return user, nil
}

func (s *SubAccountService) List(ctx context.Context, parentUserID int64, params pagination.PaginationParams) ([]SubAccountRelation, *pagination.PaginationResult, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, nil, err
	}
	return s.repo.ListByParent(ctx, parentUserID, params)
}

func (s *SubAccountService) SearchCandidates(ctx context.Context, parentUserID int64, query string, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, nil, err
	}
	query = strings.TrimSpace(query)
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 || params.PageSize > 20 {
		params.PageSize = 20
	}
	if query == "" {
		return []User{}, &pagination.PaginationResult{Total: 0, Page: params.Page, PageSize: params.PageSize, Pages: 0}, nil
	}
	includeSubscriptions := false
	users, page, err := s.userRepo.ListWithFilters(ctx, params, UserListFilters{Search: query, Role: RoleUser, IncludeSubscriptions: &includeSubscriptions})
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]User, 0, len(users))
	for _, user := range users {
		if user.ID == parentUserID {
			continue
		}
		if rel, err := s.repo.GetActiveByChild(ctx, user.ID); err == nil && rel != nil {
			continue
		} else if err != nil && !errors.Is(err, ErrSubAccountNotFound) {
			return nil, nil, err
		}
		candidates = append(candidates, user)
	}
	if page != nil {
		page.Total = int64(len(candidates))
		page.Pages = 1
	}
	return candidates, page, nil
}

func (s *SubAccountService) Add(ctx context.Context, parentUserID int64, input SubAccountUpsertInput) (*SubAccountRelation, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, err
	}
	if parentUserID == input.ChildUserID {
		return nil, ErrSubAccountSelfLink
	}
	if input.AllocatedQuota < 0 {
		input.AllocatedQuota = 0
	}
	if input.WeeklyAllocatedQuota < 0 {
		input.WeeklyAllocatedQuota = 0
	}
	child, err := s.userRepo.GetByID(ctx, input.ChildUserID)
	if err != nil {
		return nil, err
	}
	if child.Role == RoleAdmin {
		return nil, errors.New("admin user cannot be added as sub account")
	}
	if rel, err := s.repo.GetActiveByChild(ctx, input.ChildUserID); err == nil && rel != nil {
		return nil, ErrSubAccountAlreadyLinked
	} else if err != nil && !errors.Is(err, ErrSubAccountNotFound) {
		return nil, err
	}
	return s.repo.Upsert(ctx, parentUserID, input)
}

func (s *SubAccountService) UpdateQuota(ctx context.Context, parentUserID, childUserID int64, allocatedQuota, weeklyAllocatedQuota float64) (*SubAccountRelation, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, err
	}
	if allocatedQuota < 0 {
		allocatedQuota = 0
	}
	if weeklyAllocatedQuota < 0 {
		weeklyAllocatedQuota = 0
	}
	return s.repo.UpdateQuota(ctx, parentUserID, childUserID, allocatedQuota, weeklyAllocatedQuota)
}

func (s *SubAccountService) Remove(ctx context.Context, parentUserID, childUserID int64) error {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return err
	}
	return s.repo.Remove(ctx, parentUserID, childUserID)
}

func (s *SubAccountService) ListUsage(ctx context.Context, parentUserID int64, childUserID int64, params pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, nil, err
	}
	if childUserID > 0 {
		if _, err := s.repo.GetActiveByParentAndChild(ctx, parentUserID, childUserID); err != nil {
			return nil, nil, err
		}
	}
	if repo, ok := s.usageRepo.(interface {
		ListByParentAccount(context.Context, int64, int64, pagination.PaginationParams) ([]UsageLog, *pagination.PaginationResult, error)
	}); ok {
		return repo.ListByParentAccount(ctx, parentUserID, childUserID, params)
	}
	return nil, nil, errors.New("usage repository does not support parent account usage listing")
}

func (s *SubAccountService) UsageSummary(ctx context.Context, parentUserID int64, childUserID int64, filters UsageSummaryFilters) (*SubAccountUsageSummary, error) {
	if _, err := s.ensureParent(ctx, parentUserID); err != nil {
		return nil, err
	}
	if childUserID > 0 {
		if _, err := s.repo.GetActiveByParentAndChild(ctx, parentUserID, childUserID); err != nil {
			return nil, err
		}
	}
	if repo, ok := s.usageRepo.(interface {
		GetSubAccountUsageSummary(context.Context, int64, int64, UsageSummaryFilters) (*SubAccountUsageSummary, error)
	}); ok {
		return repo.GetSubAccountUsageSummary(ctx, parentUserID, childUserID, filters)
	}
	return nil, errors.New("usage repository does not support parent account usage summary")
}

type UsageSummaryFilters struct {
	StartTime time.Time
	EndTime   time.Time
}

type SubAccountUsageSummary struct {
	TotalRequests        int64       `json:"total_requests"`
	TotalTokens          int64       `json:"total_tokens"`
	TotalActualCost      float64     `json:"total_actual_cost"`
	TotalParentQuotaUsed float64     `json:"total_parent_quota_used"`
	Models               []ModelStat `json:"models"`
	Groups               []GroupStat `json:"groups"`
}

type ModelStat struct {
	Model       string  `json:"model"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actual_cost"`
}

type GroupStat struct {
	GroupID     int64   `json:"group_id"`
	GroupName   string  `json:"group_name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actual_cost"`
}

func NormalizeSubAccountStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", SubAccountStatusActive:
		return SubAccountStatusActive
	case SubAccountStatusDisabled:
		return SubAccountStatusDisabled
	default:
		return SubAccountStatusActive
	}
}
