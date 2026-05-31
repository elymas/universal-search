package testsocial

import "github.com/elymas/universal-search/pkg/types"

type Adapter struct{ subSource string }

func (a *Adapter) Capabilities() types.Capabilities {
	switch a.subSource {
	case "alpha":
		return alphaCapabilities()
	case "beta":
		return betaCapabilities()
	default:
		return types.Capabilities{SourceID: a.subSource}
	}
}

func alphaCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "alpha",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   600,
		DefaultMaxResults: 25,
	}
}

func betaCapabilities() types.Capabilities {
	return types.Capabilities{
		SourceID:          "beta",
		RequiresAuth:      false,
		AuthEnvVars:       nil,
		RateLimitPerMin:   0,
		DefaultMaxResults: 0,
	}
}
