package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// billableModelWithFallback safely falls back to a concrete model when the
// selected model has neither channel nor global token pricing.
func (s *GatewayService) billableModelWithFallback(
	ctx context.Context,
	apiKey *APIKey,
	billingModel string,
	fallbacks ...string,
) string {
	if s.hasResolvableTokenPricing(ctx, billingModel, apiKey) {
		return billingModel
	}
	for _, fallback := range fallbacks {
		fallback = strings.TrimSpace(fallback)
		if fallback == "" || fallback == billingModel {
			continue
		}
		if s.hasResolvableTokenPricing(ctx, fallback, apiKey) {
			logger.LegacyPrintf("service.gateway", "[Billing] billing model %q has no pricing, falling back to concrete model %q", billingModel, fallback)
			return fallback
		}
	}
	return billingModel
}

// hasResolvableTokenPricing reports whether a model resolves to either an
// explicit channel price or a global token price.
func (s *GatewayService) hasResolvableTokenPricing(ctx context.Context, model string, apiKey *APIKey) bool {
	if strings.TrimSpace(model) == "" {
		return false
	}
	if s.resolveChannelPricing(ctx, model, apiKey) != nil {
		return true
	}
	if s.billingService == nil {
		return false
	}
	_, err := s.billingService.GetModelPricing(model)
	return err == nil
}
