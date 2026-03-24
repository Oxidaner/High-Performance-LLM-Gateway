package provider

import (
	"context"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"llm-gateway/internal/config"
	"llm-gateway/internal/service"
)

const (
	defaultFailureThreshold = 3
	defaultOpenTimeout      = 30 * time.Second
)

type routeCandidate struct {
	modelName    string
	providerName string
	weight       int
	fallback     string
}

// Registry routes model requests to provider clients.
type Registry struct {
	providers      map[string]Client
	modelRoutes    map[string][]routeCandidate
	modelFallbacks map[string]string
	circuits       map[string]*service.CircuitBreaker

	nowFunc  func() time.Time
	randIntn func(int) int
}

// NewRegistry creates provider clients and model routing metadata from config.
func NewRegistry(cfg *config.Config) *Registry {
	registry := &Registry{
		providers:      make(map[string]Client, 3),
		modelRoutes:    make(map[string][]routeCandidate, len(cfg.Models)),
		modelFallbacks: make(map[string]string, len(cfg.Models)),
		circuits:       make(map[string]*service.CircuitBreaker),
		nowFunc:        time.Now,
		randIntn:       rand.Intn,
	}

	httpClient := &http.Client{Timeout: 30 * time.Second}
	registry.providers["openai"] = newOpenAIProvider(cfg.Providers.OpenAI, httpClient)
	registry.providers["anthropic"] = newAnthropicProvider(cfg.Providers.Anthropic, httpClient)
	registry.providers["minimax"] = newMiniMaxProvider(cfg.Providers.MiniMax, httpClient)

	for _, m := range cfg.Models {
		if strings.TrimSpace(m.Name) == "" {
			continue
		}
		if !m.IsActive {
			continue
		}

		providerName := strings.ToLower(strings.TrimSpace(m.Provider))
		if providerName == "" {
			providerName = detectProvider(m.Name)
		}

		weight := m.Weight
		if weight <= 0 {
			weight = 1
		}

		candidate := routeCandidate{
			modelName:    m.Name,
			providerName: providerName,
			weight:       weight,
			fallback:     strings.TrimSpace(m.Fallback),
		}
		registry.modelRoutes[m.Name] = append(registry.modelRoutes[m.Name], candidate)
		if candidate.fallback != "" {
			if _, exists := registry.modelFallbacks[m.Name]; !exists {
				registry.modelFallbacks[m.Name] = candidate.fallback
			}
		}
		registry.circuits[registry.circuitKey(candidate)] = service.NewCircuitBreaker(defaultFailureThreshold, defaultOpenTimeout)
	}

	return registry
}

// ChatCompletion routes a request with weighted routing, fallback, and circuit breaking.
func (r *Registry) ChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, *UpstreamError) {
	if r == nil {
		return nil, &UpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "provider registry is not initialized",
		}
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "gpt-4"
	}

	queue := r.buildRouteQueue(model)
	if len(queue) == 0 {
		return nil, &UpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "no provider route available for model " + model,
		}
	}

	var lastErr *UpstreamError
	skippedByCircuit := 0
	for _, candidate := range queue {
		client := r.providers[candidate.providerName]
		if client == nil {
			lastErr = &UpstreamError{
				Provider:   candidate.providerName,
				Model:      candidate.modelName,
				StatusCode: http.StatusServiceUnavailable,
				Message:    "provider client is not configured",
			}
			continue
		}

		breaker := r.ensureCircuit(candidate)
		now := r.now()
		if !breaker.Allow(now) {
			skippedByCircuit++
			continue
		}

		callReq := req
		callReq.Model = candidate.modelName
		resp, err := client.ChatCompletion(ctx, callReq)
		if err == nil {
			breaker.RecordSuccess()
			return resp, nil
		}

		err.Provider = candidate.providerName
		err.Model = candidate.modelName
		lastErr = err

		if shouldTripCircuit(err) {
			breaker.RecordFailure(now)
		} else {
			breaker.RecordSuccess()
		}

		if !shouldFallback(err) {
			return nil, err
		}
	}

	if skippedByCircuit == len(queue) {
		return nil, &UpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "all candidate routes are circuit-open",
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}

	return nil, &UpstreamError{
		StatusCode: http.StatusServiceUnavailable,
		Message:    "all routes failed for model " + model,
	}
}

func (r *Registry) buildRouteQueue(model string) []routeCandidate {
	modelChain := r.resolveModelChain(model)
	seen := make(map[string]struct{})
	queue := make([]routeCandidate, 0, len(modelChain))

	for _, m := range modelChain {
		candidates := r.routesForModel(m)
		ordered := r.weightedOrder(candidates)
		for _, c := range ordered {
			key := r.circuitKey(c)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			queue = append(queue, c)
		}
	}
	return queue
}

func (r *Registry) resolveModelChain(model string) []string {
	chain := make([]string, 0, 4)
	seen := make(map[string]struct{})
	current := strings.TrimSpace(model)

	for current != "" {
		if _, exists := seen[current]; exists {
			break
		}
		seen[current] = struct{}{}
		chain = append(chain, current)

		next := strings.TrimSpace(r.modelFallbacks[current])
		if next == "" {
			break
		}
		current = next
	}

	if len(chain) == 0 && model != "" {
		chain = append(chain, model)
	}

	return chain
}

func (r *Registry) routesForModel(model string) []routeCandidate {
	if candidates, ok := r.modelRoutes[model]; ok && len(candidates) > 0 {
		out := make([]routeCandidate, len(candidates))
		copy(out, candidates)
		return out
	}

	providerName := detectProvider(model)
	return []routeCandidate{
		{
			modelName:    model,
			providerName: providerName,
			weight:       1,
		},
	}
}

func (r *Registry) weightedOrder(candidates []routeCandidate) []routeCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	pool := make([]routeCandidate, len(candidates))
	copy(pool, candidates)
	ordered := make([]routeCandidate, 0, len(candidates))

	for len(pool) > 0 {
		idx := r.weightedPickIndex(pool)
		ordered = append(ordered, pool[idx])
		pool = append(pool[:idx], pool[idx+1:]...)
	}
	return ordered
}

func (r *Registry) weightedPickIndex(candidates []routeCandidate) int {
	total := 0
	for _, c := range candidates {
		if c.weight > 0 {
			total += c.weight
		}
	}
	if total <= 0 {
		return 0
	}

	pick := r.rand(total)
	for i, c := range candidates {
		weight := c.weight
		if weight <= 0 {
			continue
		}
		if pick < weight {
			return i
		}
		pick -= weight
	}

	return len(candidates) - 1
}

func (r *Registry) ensureCircuit(candidate routeCandidate) *service.CircuitBreaker {
	key := r.circuitKey(candidate)
	if breaker, ok := r.circuits[key]; ok {
		return breaker
	}
	breaker := service.NewCircuitBreaker(defaultFailureThreshold, defaultOpenTimeout)
	r.circuits[key] = breaker
	return breaker
}

func (r *Registry) circuitKey(candidate routeCandidate) string {
	return candidate.providerName + ":" + candidate.modelName
}

func (r *Registry) rand(n int) int {
	if n <= 1 {
		return 0
	}
	if r.randIntn == nil {
		return rand.Intn(n)
	}
	return r.randIntn(n)
}

func (r *Registry) now() time.Time {
	if r.nowFunc == nil {
		return time.Now()
	}
	return r.nowFunc()
}

func detectProvider(model string) string {
	model = strings.ToLower(model)
	switch {
	case strings.HasPrefix(model, "claude"):
		return "anthropic"
	case strings.HasPrefix(model, "abab"):
		return "minimax"
	default:
		return "openai"
	}
}

func shouldTripCircuit(err *UpstreamError) bool {
	if err == nil {
		return false
	}
	if err.Err != nil {
		return true
	}
	switch err.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func shouldFallback(err *UpstreamError) bool {
	if err == nil {
		return false
	}
	// Client request errors should fail fast.
	if err.StatusCode == http.StatusBadRequest {
		return false
	}
	return true
}
