package service

func ProvideChannelAcceptanceService(repo ChannelAcceptanceRepository, accounts AccountRepository, groups GroupRepository, billing *BillingService, executor ChannelAcceptanceExecutor) *ChannelAcceptanceService {
	s := NewChannelAcceptanceService(repo, accounts, groups, billing, executor)
	s.StartWorkers()
	return s
}
