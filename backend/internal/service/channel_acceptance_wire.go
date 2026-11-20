package service

func ProvideChannelAcceptanceService(repo ChannelAcceptanceRepository, accounts AccountRepository, groups GroupRepository, billing *BillingService, channels *ChannelService, executor ChannelAcceptanceExecutor) *ChannelAcceptanceService {
	s := NewChannelAcceptanceService(repo, accounts, groups, billing, executor)
	s.channels = channels
	s.StartWorkers()
	return s
}
