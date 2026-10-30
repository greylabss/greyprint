package plan

import "testing"

func TestValidate(t *testing.T) {
	base := Options{Name: "sample", Module: "example.com/sample", Router: "stdlib", Database: "none", Access: "driver"}
	tests := []struct {
		name    string
		change  func(*Options)
		wantErr bool
	}{
		{"valid", func(*Options) {}, false},
		{"bad name", func(o *Options) { o.Name = "Bad" }, true},
		{"bad router", func(o *Options) { o.Router = "mux" }, true},
		{"bad database", func(o *Options) { o.Database = "oracle" }, true},
		{"bad access", func(o *Options) { o.Access = "sqlx" }, true},
		{"access without db", func(o *Options) { o.Access = "gorm" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := base
			tt.change(&o)
			if err := o.Validate(); (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}
