package releaseinfo

import "testing"

func TestCatalogsFollowReleaseOrder(t *testing.T) {
	published := Releases()
	if len(published) == 0 || published[0].Version != "0.3.2" || published[0].Date != "2026-08-16" || published[0].Summary == "" {
		t.Fatalf("latest release = %#v", published)
	}
	planned := Roadmap()
	if len(planned) == 0 || planned[0].Version != "0.4.0" || planned[0].Date != "" || planned[0].Summary == "" {
		t.Fatalf("next planned release = %#v", planned)
	}
}
