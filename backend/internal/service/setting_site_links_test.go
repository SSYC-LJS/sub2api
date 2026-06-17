package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type siteLinksSettingRepoStub struct {
	values map[string]string
}

func (s *siteLinksSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *siteLinksSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *siteLinksSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *siteLinksSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		result[key] = s.values[key]
	}
	return result, nil
}

func (s *siteLinksSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if s.values == nil {
		s.values = make(map[string]string)
	}
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *siteLinksSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *siteLinksSettingRepoStub) Delete(ctx context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func TestGetAllSettingsReturnsSiteLinkFields(t *testing.T) {
	repo := &siteLinksSettingRepoStub{values: map[string]string{
		SettingKeyContactQRCodeURL:          "legacy-contact",
		SettingKeyContactWebmasterQRCodeURL: "webmaster-qr",
		SettingKeyContactGroupQRCodeURL:     "group-qr",
		SettingKeyDocURL:                    "https://docs.example.com",
		SettingKeyActivationCodePurchaseURL: "https://buy.example.com/code",
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAllSettings returned error: %v", err)
	}

	if settings.ContactQRCodeURL != "legacy-contact" {
		t.Fatalf("ContactQRCodeURL = %q", settings.ContactQRCodeURL)
	}
	if settings.ContactWebmasterQRCodeURL != "webmaster-qr" {
		t.Fatalf("ContactWebmasterQRCodeURL = %q", settings.ContactWebmasterQRCodeURL)
	}
	if settings.ContactGroupQRCodeURL != "group-qr" {
		t.Fatalf("ContactGroupQRCodeURL = %q", settings.ContactGroupQRCodeURL)
	}
	if settings.DocURL != "https://docs.example.com" {
		t.Fatalf("DocURL = %q", settings.DocURL)
	}
	if settings.ActivationCodePurchaseURL != "https://buy.example.com/code" {
		t.Fatalf("ActivationCodePurchaseURL = %q", settings.ActivationCodePurchaseURL)
	}
}

func TestUpdateSettingsPersistsAndReturnsSiteLinkFields(t *testing.T) {
	repo := &siteLinksSettingRepoStub{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})

	input := &SystemSettings{
		ContactQRCodeURL:                 "legacy-contact",
		ContactWebmasterQRCodeURL:        "webmaster-qr",
		ContactGroupQRCodeURL:            "group-qr",
		DocURL:                           "https://docs.example.com",
		ActivationCodePurchaseURL:        "https://buy.example.com/code",
		TableDefaultPageSize:             20,
		TablePageSizeOptions:             []int{10, 20, 50, 100},
		WeChatConnectFrontendRedirectURL: "/auth/wechat/callback",
		GitHubOAuthFrontendRedirectURL:   "/auth/oauth/callback",
		GoogleOAuthFrontendRedirectURL:   "/auth/oauth/callback",
	}
	if err := svc.UpdateSettings(context.Background(), input); err != nil {
		t.Fatalf("UpdateSettings returned error: %v", err)
	}

	settings, err := svc.GetAllSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAllSettings returned error: %v", err)
	}

	if settings.ContactQRCodeURL != "legacy-contact" {
		t.Fatalf("ContactQRCodeURL = %q", settings.ContactQRCodeURL)
	}
	if settings.ContactWebmasterQRCodeURL != "webmaster-qr" {
		t.Fatalf("ContactWebmasterQRCodeURL = %q", settings.ContactWebmasterQRCodeURL)
	}
	if settings.ContactGroupQRCodeURL != "group-qr" {
		t.Fatalf("ContactGroupQRCodeURL = %q", settings.ContactGroupQRCodeURL)
	}
	if settings.DocURL != "https://docs.example.com" {
		t.Fatalf("DocURL = %q", settings.DocURL)
	}
	if settings.ActivationCodePurchaseURL != "https://buy.example.com/code" {
		t.Fatalf("ActivationCodePurchaseURL = %q", settings.ActivationCodePurchaseURL)
	}
}
