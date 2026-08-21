package vibeproxy

import (
	"testing"

	"github.com/google/uuid"
)

func TestListCountriesInputQuery(t *testing.T) {
	poolID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	mobile := "mobile"
	httpProto := "http"
	empty := ""

	tests := []struct {
		name string
		in   ListCountriesInput
		want string
	}{
		{"no scope asks for everything", ListCountriesInput{}, ""},
		{
			"pool only",
			ListCountriesInput{PoolID: &poolID},
			"?pool_id=11111111-2222-3333-4444-555555555555",
		},
		{
			"lease axes travel together and stay sorted",
			ListCountriesInput{ConnectionType: &mobile, Protocol: &httpProto},
			"?connection_type=mobile&protocol=http",
		},
		{
			"an empty axis is not a filter",
			ListCountriesInput{ConnectionType: &empty, Protocol: &empty},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.query(); got != tt.want {
				t.Fatalf("query() = %q, want %q", got, tt.want)
			}
		})
	}
}
