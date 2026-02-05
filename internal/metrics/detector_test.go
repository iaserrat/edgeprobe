package metrics

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestNextOutageIDFormat(t *testing.T) {
	d := NewDetector(60)
	ts := time.Unix(0, 123456789)
	id := d.nextOutageID("target", ts)

	if strings.Contains(id, "%!") {
		t.Fatalf("outage id contains formatting artifacts: %s", id)
	}

	re := regexp.MustCompile(`^target-123456789-\d{6}$`)
	if !re.MatchString(id) {
		t.Fatalf("outage id format mismatch: %s", id)
	}
	if !strings.HasSuffix(id, "000001") {
		t.Fatalf("outage counter not zero-padded: %s", id)
	}
}
