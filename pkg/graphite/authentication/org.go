package authentication

import (
	"context"

	"github.com/grafana/dskit/user"
)

// ExtractOrgID extracts the org id from the given context and returns
// it as the second return value.
// If there is no org id in the context it injects "fake" and returns
// the updated context together with the org id.
func ExtractOrgID(ctx context.Context) (context context.Context, userID string) {
	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		// if we got here without any org id in the context then
		// auth is disabled so we assume "fake" user
		userID = "fake"
		return user.InjectOrgID(ctx, userID), userID
	}
	return ctx, userID
}
