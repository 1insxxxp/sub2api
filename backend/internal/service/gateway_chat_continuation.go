package service

func addClaudeUsage(dst *ClaudeUsage, src ClaudeUsage) {
	if dst == nil {
		return
	}
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.CacheCreationInputTokens += src.CacheCreationInputTokens
	dst.CacheReadInputTokens += src.CacheReadInputTokens
	dst.CacheCreation5mTokens += src.CacheCreation5mTokens
	dst.CacheCreation1hTokens += src.CacheCreation1hTokens
	dst.ImageOutputTokens += src.ImageOutputTokens
}
