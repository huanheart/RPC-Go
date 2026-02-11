package common

import (
	"context"
	"encoding/json"
	"sync"
)

// FallbackFunc 降级函数
type FallbackFunc func(ctx context.Context, args interface{}) (interface{}, error)

// FallbackHandler 降级处理器
type FallbackHandler struct {
	mu       sync.RWMutex
	fallback map[string]FallbackFunc
	enabled  bool
}

// NewFallbackHandler 创建降级处理器
func NewFallbackHandler() *FallbackHandler {
	return &FallbackHandler{
		fallback: make(map[string]FallbackFunc),
		enabled:  true,
	}
}

// Register 注册降级函数
func (f *FallbackHandler) Register(method string, fb FallbackFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fallback[method] = fb
}

// Get 获取降级函数
func (f *FallbackHandler) Get(method string) (FallbackFunc, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	fb, ok := f.fallback[method]
	return fb, ok
}

// Enable 启用降级
func (f *FallbackHandler) Enable() {
	f.mu.Lock()
	f.enabled = true
	f.mu.Unlock()
}

// Disable 禁用降级
func (f *FallbackHandler) Disable() {
	f.mu.Lock()
	f.enabled = false
	f.mu.Unlock()
}

// IsEnabled 检查是否启用
func (f *FallbackHandler) IsEnabled() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.enabled
}

// Handle 执行降级
func (f *FallbackHandler) Handle(ctx context.Context, method string, args interface{}) (interface{}, error) {
	if !f.IsEnabled() {
		return nil, nil
	}

	fb, ok := f.Get(method)
	if !ok {
		return nil, nil
	}

	return fb(ctx, args)
}

// MockFallback mock 降级返回预设值
func MockFallback(defaultValue string) FallbackFunc {
	return func(ctx context.Context, args interface{}) (interface{}, error) {
		var result interface{}
		json.Unmarshal([]byte(defaultValue), &result)
		return result, nil
	}
}

// CacheFallback 缓存降级
func CacheFallback(cache map[string]interface{}) FallbackFunc {
	return func(ctx context.Context, args interface{}) (interface{}, error) {
		return cache, nil
	}
}
