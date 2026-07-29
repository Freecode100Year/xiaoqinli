package codegen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CodegenStrategyConfig defines performance optimization strategies for a target language backend.
type CodegenStrategyConfig struct {
	Target              string            `json:"target"`
	PreferComprehension bool              `json:"prefer_comprehension"`
	InlineThreshold     int               `json:"inline_threshold"`
	OptimizationFlags   map[string]string `json:"optimization_flags"`
	BenchmarkScore      float64           `json:"benchmark_score"`
	LastUpdated         string            `json:"last_updated"`
}

var (
	stratMutex sync.RWMutex
	strategies = map[string]*CodegenStrategyConfig{}
)

// UpdateCodegenStrategy updates performance strategy settings.
func UpdateCodegenStrategy(s CodegenStrategyConfig) *CodegenStrategyConfig {
	stratMutex.Lock()
	defer stratMutex.Unlock()

	norm := strings.ToLower(strings.TrimSpace(s.Target))
	s.Target = norm
	s.LastUpdated = time.Now().Format("2006-01-02 15:04:05")
	if s.OptimizationFlags == nil {
		s.OptimizationFlags = make(map[string]string)
	}
	strategies[norm] = &s
	return &s
}

// InspectCodegenStrategy gets current performance strategy for a target.
func InspectCodegenStrategy(target string) *CodegenStrategyConfig {
	stratMutex.RLock()
	defer stratMutex.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(target))
	if s, ok := strategies[norm]; ok {
		cp := *s
		return &cp
	}
	return &CodegenStrategyConfig{
		Target:              norm,
		PreferComprehension: true,
		InlineThreshold:     30,
		OptimizationFlags:   map[string]string{"opt_level": "2"},
		BenchmarkScore:      100.0,
		LastUpdated:         time.Now().Format("2006-01-02"),
	}
}

// ListAllCodegenStrategies returns all registered target strategies.
func ListAllCodegenStrategies() map[string]*CodegenStrategyConfig {
	stratMutex.RLock()
	defer stratMutex.RUnlock()

	res := make(map[string]*CodegenStrategyConfig, len(strategies))
	for k, v := range strategies {
		cp := *v
		res[k] = &cp
	}
	return res
}

// LoadStrategiesFromFile loads persisted strategy configs from disk.
func LoadStrategiesFromFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	var loaded map[string]*CodegenStrategyConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	stratMutex.Lock()
	defer stratMutex.Unlock()
	for k, v := range loaded {
		strategies[strings.ToLower(k)] = v
	}
	return nil
}

// SaveStrategiesToFile saves codegen strategy configs to disk.
func SaveStrategiesToFile(filePath string) error {
	stratMutex.RLock()
	defer stratMutex.RUnlock()

	data, err := json.MarshalIndent(strategies, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(filePath), 0755)
	return os.WriteFile(filePath, data, 0644)
}
