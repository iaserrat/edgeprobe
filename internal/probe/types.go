package probe

import "time"

type PingResult struct {
	Target string
	Time   time.Time
	OK     bool
	RTTMs  float64
}

type DNSResult struct {
	Time time.Time
	OK   bool
}
