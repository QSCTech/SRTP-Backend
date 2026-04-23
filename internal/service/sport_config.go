package service

import "strings"

// SportConfig defines TYYS reservation rules for a specific sport type.
// Centralising this avoids scattered switch/if chains across service methods.
type SportConfig struct {
	// RequiresBuddyCode is true for pair sports (e.g. 羽毛球, 网球) where TYYS
	// requires a partner/buddy code in the order form.
	RequiresBuddyCode bool

	// MinMemberCount is the minimum number of joined room members required
	// before a reservation plan (preview/submit) is allowed. Pair sports need
	// at least 2 players in the room so the booking makes sense.
	MinMemberCount int
}

// sportConfigs maps TYYS sport name to its reservation config.
// Add new sport types here as TYYS expands its offerings.
var sportConfigs = map[string]SportConfig{
	"羽毛球": {RequiresBuddyCode: true, MinMemberCount: 2},
	"网球":  {RequiresBuddyCode: true, MinMemberCount: 2},
	"健身":  {RequiresBuddyCode: false, MinMemberCount: 1},
	"游泳":  {RequiresBuddyCode: false, MinMemberCount: 1},
}

// getSportConfig returns the SportConfig for a given sport type name.
// Unknown sports fall back to a safe default (no buddy code, 1 member minimum).
func getSportConfig(sport string) SportConfig {
	if cfg, ok := sportConfigs[strings.TrimSpace(sport)]; ok {
		return cfg
	}
	return SportConfig{RequiresBuddyCode: false, MinMemberCount: 1}
}
