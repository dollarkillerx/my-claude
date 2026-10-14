package usage

import "testing"

func TestParse(t *testing.T) {
	r, err := Parse([]byte(`{
		"seven_day_opus": {"utilization": 3, "resets_at": null},
		"seven_day": {"utilization": 41.5, "resets_at": "2026-10-20T10:00:00+00:00"},
		"five_hour": {"utilization": 12, "resets_at": "2026-10-14T15:00:00Z"},
		"extra_usage": {"is_enabled": false},
		"seven_day_sonnet": null
	}`))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, w := range r.Windows {
		names = append(names, w.Name)
	}
	if len(names) != 3 || names[0] != "five_hour" || names[1] != "seven_day" || names[2] != "seven_day_opus" {
		t.Fatalf("windows = %v", names)
	}
	if r.Windows[1].Utilization != 41.5 || r.Windows[1].ResetsAt.IsZero() {
		t.Errorf("seven_day = %+v", r.Windows[1])
	}
	if !r.Windows[2].ResetsAt.IsZero() {
		t.Errorf("null resets_at should be zero")
	}
}
