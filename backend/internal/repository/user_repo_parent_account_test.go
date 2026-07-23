package repository

import (
	"context"
	"testing"

	"github.com/SSYC-LJS/sub2api/internal/pkg/pagination"
	"github.com/SSYC-LJS/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryUpdatePersistsAndReloadsIsParentAccount(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	ctx := context.Background()

	user := &service.User{
		Email:        "parent@example.com",
		Username:     "parent-user",
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  1,
	}
	require.NoError(t, repo.Create(ctx, user))
	require.False(t, user.IsParentAccount)

	user.IsParentAccount = true
	require.NoError(t, repo.Update(ctx, user))

	byID, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, byID.IsParentAccount, "GetByID should return the persisted parent-account flag")

	byEmail, err := repo.GetByEmail(ctx, "parent@example.com")
	require.NoError(t, err)
	require.True(t, byEmail.IsParentAccount, "GetByEmail should return the persisted parent-account flag")

	items, _, err := repo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, service.UserListFilters{Search: "parent@example.com"})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].IsParentAccount, "ListWithFilters should return the persisted parent-account flag")
}
