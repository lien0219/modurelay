package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type CompositeRouteResolver struct {
	repo                   CompositeModelRouteRepository
	modelOwnershipResolver CompositeModelOwnershipResolver
}

func NewCompositeRouteResolver(repo CompositeModelRouteRepository) *CompositeRouteResolver {
	return &CompositeRouteResolver{repo: repo}
}

func (r *CompositeRouteResolver) SetModelOwnershipResolver(resolver CompositeModelOwnershipResolver) {
	if r != nil {
		r.modelOwnershipResolver = resolver
	}
}

func (r *CompositeRouteResolver) Resolve(ctx context.Context, groupID int64, model, endpoint string) (CompositeRouteDecision, error) {
	return r.ResolveAllowed(ctx, groupID, model, endpoint, nil)
}

// ResolveAllowed filters concrete route candidates before priority selection.
// A nil predicate preserves the legacy unrestricted behavior.
func (r *CompositeRouteResolver) ResolveAllowed(ctx context.Context, groupID int64, model, endpoint string, allowPlatform func(string) bool) (CompositeRouteDecision, error) {
	model = strings.TrimSpace(model)
	endpoint = normalizeCompositeRouteEndpoint(endpoint)
	decision := CompositeRouteDecision{
		GroupID:     groupID,
		PublicModel: model,
		Endpoint:    endpoint,
	}
	if model == "" {
		decision.Reason = "model is required"
		return decision, nil
	}

	if r != nil && r.repo != nil && groupID > 0 {
		routes, err := r.repo.ListByGroup(ctx, groupID, false)
		if err != nil {
			return decision, fmt.Errorf("list composite routes: %w", err)
		}
		matchedRoute, hadMatch := matchCompositeRoute(routes, model, endpoint)
		if allowPlatform != nil {
			allowedRoutes := make([]CompositeModelRoute, 0, len(routes))
			for _, route := range routes {
				if allowPlatform(strings.TrimSpace(route.TargetPlatform)) {
					allowedRoutes = append(allowedRoutes, route)
				}
			}
			routes = allowedRoutes
		}
		if route, ok := matchCompositeRoute(routes, model, endpoint); ok {
			upstreamModel := strings.TrimSpace(route.UpstreamModel)
			if upstreamModel == "" {
				upstreamModel = model
			}
			return CompositeRouteDecision{
				Matched:        true,
				Source:         CompositeRouteSourceExplicit,
				GroupID:        groupID,
				PublicModel:    model,
				TargetPlatform: route.TargetPlatform,
				UpstreamModel:  upstreamModel,
				Endpoint:       endpoint,
				Route:          &route,
			}, nil
		}
		if hadMatch && allowPlatform != nil {
			decision.PolicyDenied = true
			decision.TargetPlatform = strings.TrimSpace(matchedRoute.TargetPlatform)
			decision.Reason = "all matching composite routes are denied by platform policy"
			return decision, nil
		}
	}

	if r != nil && r.modelOwnershipResolver != nil && groupID > 0 {
		ownership, err := r.modelOwnershipResolver(ctx, groupID, model)
		if err != nil {
			// A recognizable model can still use the existing detector when the
			// account catalog is temporarily unavailable. Unknown aliases cannot.
			if _, detectable := DetectModelPlatform(model); !detectable {
				return decision, fmt.Errorf("resolve account model ownership: %w", err)
			}
		} else if ownership.Ambiguous {
			if allowPlatform != nil {
				allowedPlatforms := make([]string, 0, len(ownership.Platforms))
				for _, platform := range ownership.Platforms {
					if allowPlatform(strings.TrimSpace(platform)) {
						allowedPlatforms = append(allowedPlatforms, strings.TrimSpace(platform))
					}
				}
				if len(allowedPlatforms) == 1 {
					return CompositeRouteDecision{
						Matched:        true,
						Source:         CompositeRouteSourceAccount,
						GroupID:        groupID,
						PublicModel:    model,
						TargetPlatform: allowedPlatforms[0],
						UpstreamModel:  model,
						Endpoint:       endpoint,
					}, nil
				}
				if len(allowedPlatforms) == 0 {
					decision.PolicyDenied = true
					if len(ownership.Platforms) > 0 {
						decision.TargetPlatform = ownership.Platforms[0]
					}
					decision.Reason = "all ambiguous model ownership platforms are denied by policy"
					return decision, nil
				}
			}
			decision.Reason = "model is exposed by multiple provider platforms"
			return decision, nil
		} else if ownership.Matched {
			platform := strings.TrimSpace(ownership.TargetPlatform)
			if !isConcreteRequestPlatform(platform) {
				decision.Reason = "account model ownership has no concrete target platform"
				return decision, nil
			}
			if allowPlatform != nil && !allowPlatform(platform) {
				decision.PolicyDenied = true
				decision.TargetPlatform = platform
				decision.Reason = "account model platform is denied by policy"
				return decision, nil
			}
			return CompositeRouteDecision{
				Matched:        true,
				Source:         CompositeRouteSourceAccount,
				GroupID:        groupID,
				PublicModel:    model,
				TargetPlatform: platform,
				UpstreamModel:  model,
				Endpoint:       endpoint,
			}, nil
		}
	}

	if platform, ok := DetectModelPlatform(model); ok {
		if allowPlatform != nil && !allowPlatform(platform) {
			decision.PolicyDenied = true
			decision.TargetPlatform = platform
			decision.Reason = "detected model platform is denied by policy"
			return decision, nil
		}
		return CompositeRouteDecision{
			Matched:        true,
			Source:         CompositeRouteSourceDetector,
			GroupID:        groupID,
			PublicModel:    model,
			TargetPlatform: platform,
			UpstreamModel:  model,
			Endpoint:       endpoint,
		}, nil
	}
	decision.Reason = "no explicit route or built-in detector match"
	return decision, nil
}

func matchCompositeRoute(routes []CompositeModelRoute, model, endpoint string) (CompositeModelRoute, bool) {
	if len(routes) == 0 {
		return CompositeModelRoute{}, false
	}

	type candidate struct {
		route          CompositeModelRoute
		matchStrength  int
		endpointWeight int
		prefixLen      int
	}
	candidates := make([]candidate, 0, len(routes))
	for _, route := range routes {
		route.Endpoint = normalizeCompositeRouteEndpoint(route.Endpoint)
		if route.Endpoint != endpoint && route.Endpoint != CompositeRouteEndpointAny {
			continue
		}
		route.MatchType = normalizeCompositeRouteMatchType(route.MatchType)
		publicModel := strings.TrimSpace(route.PublicModel)
		if publicModel == "" {
			continue
		}

		matchStrength := 0
		prefixLen := len(publicModel)
		switch route.MatchType {
		case CompositeRouteMatchExact:
			if publicModel != model {
				continue
			}
			matchStrength = 2
		case CompositeRouteMatchPrefix:
			if !strings.HasPrefix(model, publicModel) {
				continue
			}
			matchStrength = 1
		default:
			continue
		}
		endpointWeight := 0
		if route.Endpoint == endpoint {
			endpointWeight = 1
		}
		candidates = append(candidates, candidate{
			route:          route,
			matchStrength:  matchStrength,
			endpointWeight: endpointWeight,
			prefixLen:      prefixLen,
		})
	}
	if len(candidates) == 0 {
		return CompositeModelRoute{}, false
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.matchStrength != b.matchStrength {
			return a.matchStrength > b.matchStrength
		}
		if a.endpointWeight != b.endpointWeight {
			return a.endpointWeight > b.endpointWeight
		}
		if a.prefixLen != b.prefixLen {
			return a.prefixLen > b.prefixLen
		}
		if a.route.Priority != b.route.Priority {
			return a.route.Priority < b.route.Priority
		}
		return a.route.ID < b.route.ID
	})
	return candidates[0].route, true
}
