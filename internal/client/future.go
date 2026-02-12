package client

import (
	"context"
	"encoding/json"
	"errors"
	"kamaRPC/internal/codec"
	"sync"
	"time"
)

// Future 异步调用结果
type Future struct {
	done  chan struct{}
	res   []byte
	err   error
	mu    sync.Mutex
	codec codec.Codec
}

// NewFuture 创建新的 Future
func NewFuture(opts ...FutureOption) (*Future, error) {

	f := &Future{
		done: make(chan struct{}),
	}

	for _, opt := range opts {
		if err := opt(f); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// Done 设置结果
func (f *Future) Done(res []byte, err error) {
	f.mu.Lock()
	f.res = res
	f.err = err
	f.mu.Unlock()
	close(f.done)
}

// Wait 等待结果
func (f *Future) Wait() ([]byte, error) {
	<-f.done
	f.mu.Lock()
	res, err := f.res, f.err
	f.mu.Unlock()
	return res, err
}

// WaitWithTimeout 带超时等待
func (f *Future) WaitWithTimeout(timeout time.Duration) ([]byte, error) {
	select {
	case <-f.done:
		f.mu.Lock()
		res, err := f.res, f.err
		f.mu.Unlock()
		return res, err
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	}
}

// GetResult 获取结果并反序列化到 reply
func (f *Future) GetResult(reply interface{}) error {
	<-f.done
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return f.err
	}

	return f.codec.Unmarshal(f.res, reply)
}

// GetResultWithTimeout 带超时获取结果
func (f *Future) GetResultWithTimeout(reply interface{}, timeout time.Duration) error {
	select {
	case <-f.done:
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.err != nil {
			return f.err
		}
		return json.Unmarshal(f.res, reply)
	case <-time.After(timeout):
		return errors.New("timeout")
	}
}

// IsDone 检查是否完成
func (f *Future) IsDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

// WithContext 将 Future 包装为 Context 支持的版本
func (f *Future) WithContext(ctx context.Context) *FutureWithContext {
	return &FutureWithContext{
		future: f,
		ctx:    ctx,
	}
}

// FutureWithContext 支持 Context 的 Future
type FutureWithContext struct {
	future *Future
	ctx    context.Context
}

// Wait 等待结果，支持 Context 取消
func (fw *FutureWithContext) Wait() ([]byte, error) {
	select {
	case <-fw.future.done:
		return fw.future.Wait()
	case <-fw.ctx.Done():
		return nil, fw.ctx.Err()
	}
}

// GetResult 获取结果并反序列化
func (fw *FutureWithContext) GetResult(reply interface{}) error {
	select {
	case <-fw.future.done:
		return fw.future.GetResult(reply)
	case <-fw.ctx.Done():
		return fw.ctx.Err()
	}
}
