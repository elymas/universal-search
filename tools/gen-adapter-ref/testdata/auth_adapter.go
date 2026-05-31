package testauth

import "github.com/elymas/universal-search/pkg/types"

type Adapter struct{}

func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "testauthsrc",
		RequiresAuth:      true,
		AuthEnvVars:       []string{"TEST_API_KEY", "TEST_API_SECRET"},
		RateLimitPerMin:   30,
		DefaultMaxResults: 25,
	}
}
