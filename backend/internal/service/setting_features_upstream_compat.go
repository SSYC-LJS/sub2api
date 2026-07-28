package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func (s *SettingService) IsAffiliateAdminRechargeEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAffiliateAdminRechargeEnabled)
	if err != nil {
		return AdminRechargeRebateEnabledDefault
	}
	return value == "true"
}

func (s *SettingService) PasskeyEnabled(ctx context.Context) (bool, error) {
	if !s.passkeyConfigured() {
		return false, nil
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyPasskeyEnabled)
	if errors.Is(err, ErrSettingNotFound) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read passkey setting: %w", err)
	}
	return value == "true", nil
}

func (s *SettingService) PasskeyConfiguration() (configured bool, rpID string, origins []string) {
	if s == nil || s.cfg == nil {
		return false, "", []string{}
	}
	origins = append([]string{}, s.cfg.WebAuthn.RPOrigins...)
	return s.cfg.WebAuthn.Enabled, strings.TrimSpace(s.cfg.WebAuthn.RPID), origins
}

func (s *SettingService) passkeyConfigured() bool {
	return s != nil && s.cfg != nil && s.cfg.WebAuthn.Enabled
}

func (s *SettingService) passkeySettingEnabled(settings map[string]string) bool {
	if !s.passkeyConfigured() {
		return false
	}
	value, exists := settings[SettingKeyPasskeyEnabled]
	if !exists {
		return true
	}
	return value == "true"
}

func (s *SettingService) IsSessionBindingEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySessionBindingEnabled)
	return err == nil && value == "true"
}

func (s *SettingService) IsStepUpEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyStepUpEnabled)
	return err == nil && value == "true"
}

const defaultAuditLogRetentionDays = 180

func (s *SettingService) GetAuditLogRetentionDays(ctx context.Context) int {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAuditLogRetentionDays)
	if err != nil {
		return defaultAuditLogRetentionDays
	}
	return parseAuditLogRetentionDays(value)
}

func parseAuditLogRetentionDays(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultAuditLogRetentionDays
	}
	days, err := strconv.Atoi(value)
	if err != nil {
		return defaultAuditLogRetentionDays
	}
	if days < 0 {
		return 0
	}
	return days
}

func parseOpenAIOAuthSchedulingRateMultiplier(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return defaultOpenAIOAuthSchedulingRateMultiplier
	}
	return value
}

func parseForwardedClientIPHeadersSetting(value string) ([]string, error) {
	var headers []string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return nil, fmt.Errorf("parse forwarded_client_ip_headers: %w", err)
	}
	if headers == nil {
		return nil, fmt.Errorf("parse forwarded_client_ip_headers: value must be a JSON array")
	}
	normalized, err := config.NormalizeForwardedClientIPHeaders(headers)
	if err != nil {
		return nil, fmt.Errorf("parse forwarded_client_ip_headers: %w", err)
	}
	return normalized, nil
}

type ModelPlazaRuntime struct {
	Enabled     bool
	RequireAuth bool
	Description string
}

func (s *SettingService) GetModelPlazaRuntime(ctx context.Context) ModelPlazaRuntime {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyModelPlazaEnabled,
		SettingKeyModelPlazaRequireAuth,
		SettingKeyModelPlazaDescription,
	})
	if err != nil {
		return ModelPlazaRuntime{Enabled: false}
	}
	return ModelPlazaRuntime{
		Enabled:     values[SettingKeyModelPlazaEnabled] == "true",
		RequireAuth: values[SettingKeyModelPlazaRequireAuth] == "true",
		Description: values[SettingKeyModelPlazaDescription],
	}
}
