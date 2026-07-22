package extensions

import "testing"

func TestParseListRPCResultNested(t *testing.T) {
	in := map[string]interface{}{
		"result": map[string]interface{}{
			"extensions": []map[string]interface{}{
				{"id": "acme.sample", "enabled": true, "ui_panel": true, "ui_status_method": "info"},
			},
		},
	}
	rows := ParseListRPCResult(in)
	if len(rows) != 1 || rows[0].ID != "acme.sample" || !rows[0].UIPanel || !rows[0].Enabled {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
