package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type requestResponseCaptureSettingRepoStub struct {
	values map[string]string
}

func (r *requestResponseCaptureSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *requestResponseCaptureSettingRepoStub) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}

func (r *requestResponseCaptureSettingRepoStub) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *requestResponseCaptureSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *requestResponseCaptureSettingRepoStub) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *requestResponseCaptureSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *requestResponseCaptureSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func TestNormalizeRequestResponseCaptureSettings_UsesUnlimitedBodies(t *testing.T) {
	settings := normalizeRequestResponseCaptureSettings(RequestResponseCaptureSettings{
		Enabled:      true,
		GroupID:      -10,
		MaxBodyBytes: 1024 * 1024,
	})

	require.True(t, settings.Enabled)
	require.Zero(t, settings.GroupID)
	require.Zero(t, settings.MaxBodyBytes)
}

func TestRequestResponseCaptureSettings_CapturesGroup(t *testing.T) {
	group10 := int64(10)
	group20 := int64(20)

	require.True(t, (RequestResponseCaptureSettings{}).CapturesGroup(nil))
	require.True(t, (RequestResponseCaptureSettings{}).CapturesGroup(&group10))
	require.True(t, (RequestResponseCaptureSettings{GroupID: 10}).CapturesGroup(&group10))
	require.False(t, (RequestResponseCaptureSettings{GroupID: 10}).CapturesGroup(&group20))
	require.False(t, (RequestResponseCaptureSettings{GroupID: 10}).CapturesGroup(nil))
}

func TestSettingService_RequestResponseCapturePersistsGroupAndUnlimitedMode(t *testing.T) {
	repo := &requestResponseCaptureSettingRepoStub{values: map[string]string{
		SettingKeyRequestResponseCaptureEnabled:      "true",
		SettingKeyRequestResponseCaptureGroupID:      "42",
		SettingKeyRequestResponseCaptureMaxBodyBytes: "65536",
	}}
	cfg := &config.Config{}
	svc := NewSettingService(repo, cfg)

	settings := svc.GetRequestResponseCaptureSettings(context.Background())
	require.True(t, settings.Enabled)
	require.Equal(t, int64(42), settings.GroupID)
	require.Zero(t, settings.MaxBodyBytes)

	updated, err := svc.UpdateRequestResponseCaptureSettings(context.Background(), RequestResponseCaptureSettings{
		Enabled:      true,
		GroupID:      7,
		MaxBodyBytes: 1024,
	})
	require.NoError(t, err)
	require.Equal(t, int64(7), updated.GroupID)
	require.Zero(t, updated.MaxBodyBytes)
	require.Equal(t, "7", repo.values[SettingKeyRequestResponseCaptureGroupID])
	require.Equal(t, "0", repo.values[SettingKeyRequestResponseCaptureMaxBodyBytes])
}
