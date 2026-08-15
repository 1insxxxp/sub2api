//go:build unit

package admin

func newCheckinHandlerForCampaignService(campaignService checkinCampaignAdminService) *CheckinHandler {
	return &CheckinHandler{campaignService: campaignService}
}
