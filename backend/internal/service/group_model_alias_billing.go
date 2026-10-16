package service

// A public alias must not accidentally select a model-family fallback price.
// Explicit administrator prices still take precedence over target pricing.
func groupAliasBillingModel(group *Group, model string, explicitlyPriced func(string) bool) string {
	upstream, matched := group.ResolveModelAlias(model)
	if !matched || (explicitlyPriced != nil && explicitlyPriced(model)) {
		return model
	}
	return upstream
}
