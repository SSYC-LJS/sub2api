package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type subAccountRepoStub struct {
	activeByChild *SubAccountRelation
	activeErr     error
	upsertCalled  bool
}

func (s *subAccountRepoStub) ListByParent(context.Context, int64, pagination.PaginationParams) ([]SubAccountRelation, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *subAccountRepoStub) GetActiveByParentAndChild(context.Context, int64, int64) (*SubAccountRelation, error) {
	return nil, ErrSubAccountNotFound
}

func (s *subAccountRepoStub) GetActiveByChild(context.Context, int64) (*SubAccountRelation, error) {
	return s.activeByChild, s.activeErr
}

func (s *subAccountRepoStub) Upsert(context.Context, int64, SubAccountUpsertInput) (*SubAccountRelation, error) {
	s.upsertCalled = true
	return &SubAccountRelation{}, nil
}

func (s *subAccountRepoStub) UpdateQuota(context.Context, int64, int64, float64, float64) (*SubAccountRelation, error) {
	return nil, nil
}

func (s *subAccountRepoStub) Remove(context.Context, int64, int64) error { return nil }

type subAccountUserRepoStub struct {
	users map[int64]*User
}

func (s *subAccountUserRepoStub) Create(context.Context, *User) error { return nil }
func (s *subAccountUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return user, nil
}
func (s *subAccountUserRepoStub) GetByIDIncludeDeleted(context.Context, int64) (*User, error) {
	return nil, ErrUserNotFound
}
func (s *subAccountUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}
func (s *subAccountUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	return nil, ErrUserNotFound
}
func (s *subAccountUserRepoStub) Update(context.Context, *User) error { return nil }
func (s *subAccountUserRepoStub) Delete(context.Context, int64) error { return nil }
func (s *subAccountUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	return nil, nil
}
func (s *subAccountUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	return nil, nil
}
func (s *subAccountUserRepoStub) DeleteUserAvatar(context.Context, int64) error { return nil }
func (s *subAccountUserRepoStub) List(context.Context, pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *subAccountUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, UserListFilters) ([]User, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *subAccountUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (s *subAccountUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (s *subAccountUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	return nil
}
func (s *subAccountUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (s *subAccountUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (s *subAccountUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (s *subAccountUserRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *subAccountUserRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *subAccountUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (s *subAccountUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *subAccountUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *subAccountUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *subAccountUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (s *subAccountUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	return nil
}
func (s *subAccountUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (s *subAccountUserRepoStub) EnableTotp(context.Context, int64) error                { return nil }
func (s *subAccountUserRepoStub) DisableTotp(context.Context, int64) error               { return nil }

func TestSubAccountServiceAddRejectsAlreadyLinkedChild(t *testing.T) {
	repo := &subAccountRepoStub{activeByChild: &SubAccountRelation{ParentUserID: 99, ChildUserID: 2}}
	userRepo := &subAccountUserRepoStub{users: map[int64]*User{
		1: {ID: 1, Role: RoleUser, Status: StatusActive, IsParentAccount: true},
		2: {ID: 2, Role: RoleUser, Status: StatusActive},
	}}
	svc := NewSubAccountService(repo, userRepo, nil)

	_, err := svc.Add(context.Background(), 1, SubAccountUpsertInput{ChildUserID: 2, AllocatedQuota: 10})

	require.ErrorIs(t, err, ErrSubAccountAlreadyLinked)
	require.False(t, repo.upsertCalled)
}

func TestSubAccountServiceAddPropagatesActiveRelationLookupError(t *testing.T) {
	lookupErr := errors.New("lookup failed")
	repo := &subAccountRepoStub{activeErr: lookupErr}
	userRepo := &subAccountUserRepoStub{users: map[int64]*User{
		1: {ID: 1, Role: RoleUser, Status: StatusActive, IsParentAccount: true},
		2: {ID: 2, Role: RoleUser, Status: StatusActive},
	}}
	svc := NewSubAccountService(repo, userRepo, nil)

	_, err := svc.Add(context.Background(), 1, SubAccountUpsertInput{ChildUserID: 2})

	require.ErrorIs(t, err, lookupErr)
	require.False(t, repo.upsertCalled)
}
