package testadapter

import "github.com/elymas/universal-search/pkg/types"

type Adapter struct{}

func (a *Adapter) Name() string { return "testadapter" }

func (a *Adapter) Capabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "testadapter",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   42,
		DefaultMaxResults: 10,
	}
}
