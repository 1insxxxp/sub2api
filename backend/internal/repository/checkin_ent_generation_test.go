package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinblacklist"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinstatussnapshot"
)

func TestCheckinEntPackagesExposeProductionFields(t *testing.T) {
	requiredFields := []string{
		usercheckin.FieldCheckinDate,
		usercheckin.FieldRewardAmount,
		usercheckin.FieldBalanceBefore,
		usercheckin.FieldBalanceAfter,
		usercheckin.FieldStreakDay,
		usercheckin.FieldBaseRewardAmount,
		usercheckin.FieldBonusRewardAmount,
		usercheckin.FieldTotalRewardAmount,
		usercheckinstatussnapshot.FieldCurrentStreak,
		usercheckinstatussnapshot.FieldLastCheckinDate,
		usercheckinstatussnapshot.FieldLifetimeCheckinDays,
		usercheckinblacklist.FieldRemovedAt,
	}
	for _, field := range requiredFields {
		if field == "" {
			t.Fatal("check-in Ent field name must not be empty")
		}
	}
}
