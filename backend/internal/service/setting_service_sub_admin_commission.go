package service

import (
	"context"
	"math"
	"strconv"
)

const SettingKeySubAdminCommissionRate = "sub_admin_commission_rate"

func (s *SettingService) GetSubAdminCommissionRate(ctx context.Context) float64 {
	if s == nil || s.settingRepo == nil {
		return 0
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySubAdminCommissionRate)
	if err != nil {
		return 0
	}
	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return 0
	}
	return rate
}

func (s *SettingService) SetSubAdminCommissionRate(ctx context.Context, rate float64) error {
	if s == nil || s.settingRepo == nil {
		return ErrSettingNotFound
	}
	return s.settingRepo.Set(ctx, SettingKeySubAdminCommissionRate, strconv.FormatFloat(rate, 'f', -1, 64))
}
