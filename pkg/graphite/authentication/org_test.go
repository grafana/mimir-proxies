package authentication

import (
	"context"
	"testing"

	"github.com/grafana/dskit/user"
)

func TestExtractOrgID(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		expectUser string
	}{
		{
			name:       "expect injected user to get returned",
			ctx:        user.InjectOrgID(context.Background(), "testUser"),
			expectUser: "testUser",
		}, {
			name:       "expect fake user to get returned",
			ctx:        context.Background(),
			expectUser: "fake",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, returnedUser := ExtractOrgID(tc.ctx)
			if returnedUser != tc.expectUser {
				t.Fatalf("TC (%s): Expected returned user to be %q, but it was %q",
					tc.name,
					tc.expectUser,
					returnedUser)
			}

			extractedUser, err := user.ExtractOrgID(ctx)
			if err != nil {
				t.Fatalf("TC (%s): Unexpected error returned by .ExtractOrgId: %s", tc.name, err)
			}

			if extractedUser != tc.expectUser {
				t.Fatalf("TC (%s): Expected extracted user to be %q, but it was %q",
					tc.name,
					tc.expectUser,
					extractedUser)
			}
		})
	}
}
