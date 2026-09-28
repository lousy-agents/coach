package coachapi

import (
	"time"
)

func cloneTimePtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}
