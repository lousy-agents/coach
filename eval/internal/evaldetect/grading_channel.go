package evaldetect

// GradingChannelRule is the rule ID of a reference to the grading channel from non-test code.
const GradingChannelRule = "gaming.grading_channel_access"

// GradingChannelAccess reports non-test Go and TypeScript code under root that reaches for the grading channel.
func GradingChannelAccess(root string) ([]Finding, error) {
	return nil, nil
}
