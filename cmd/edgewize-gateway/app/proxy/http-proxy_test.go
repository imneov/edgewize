package proxy

import "testing"

func Test_getHeaderString(t *testing.T) {
	header := map[string]interface{}{
		"fstr": "abc",
		"fnil": nil,
		"fnum": 1.2,
	}
	tests := []struct {
		name    string
		key     string
		wantVal string
	}{
		{"1", "fstr", "abc"},
		{"2", "fnil", ""},
		{"3", "fnum", ""},
		{"4", "unknown", ""},
		{"5", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotVal := getHeaderString(header, tt.key); gotVal != tt.wantVal {
				t.Errorf("getHeaderString() = %v, want %v", gotVal, tt.wantVal)
			}
		})
	}
}
