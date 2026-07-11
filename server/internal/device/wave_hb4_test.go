package device

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestDownloadBundle_HB4_RangeHeader is the permanent HB-4 guard (§11.4.115
// GREEN polarity). DownloadBundle built the Range as `bytes=<offset>-<offset+
// size-1>`, which for the "resume from offset, size unknown" case (offset>0,
// size==0) emitted an INVALID range with end < start (e.g. "bytes=4096-4095").
// The fix emits an open-ended range in that case. Captures the ACTUAL Range
// header the client sends.
//
// Anti-tautology anchor: reverting the open-ended `case offset > 0:` branch to
// the original `bytes=%d-%d, offset+size-1` form makes the offset>0/size==0
// subtest see "bytes=4096-4095" -> RED; restore -> GREEN.
func TestDownloadBundle_HB4_RangeHeader(t *testing.T) {
	cases := []struct {
		name         string
		offset, size int64
		wantRange    string
	}{
		{"resume_offset_unknown_size", 4096, 0, "bytes=4096-"},
		{"offset_and_size", 100, 50, "bytes=100-149"},
		{"from_start_size", 0, 10, "bytes=0-9"},
		{"full_no_range", 0, 0, ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var gotRange string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotRange = r.Header.Get("Range")
				_, _ = w.Write([]byte("payload"))
			}))
			defer srv.Close()

			c := NewApplyPortClient(srv.URL)
			if _, err := c.DownloadBundle(context.Background(), srv.URL, tc.offset, tc.size); err != nil {
				t.Fatalf("DownloadBundle(offset=%d,size=%d): %v", tc.offset, tc.size, err)
			}
			if gotRange != tc.wantRange {
				t.Fatalf("HB-4 %s: Range header = %q, want %q", tc.name, gotRange, tc.wantRange)
			}
		})
	}
}
