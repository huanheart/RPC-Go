package middleware

import (
	"context"
)

// Middleware RPC中间件类型
type Middleware func(ctx context.Context, handler HandlerFunc) HandlerFunc

// HandlerFunc 处理函数
type HandlerFunc func(ctx context.Context) error

// BuildChain 构建中间件链
func BuildChain(mws []Middleware, final HandlerFunc) HandlerFunc {
	return func(ctx context.Context) error {
		h := final
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](ctx, h)
		}
		return h(ctx)
	}
}

// UnaryInterceptor 一元拦截器
type UnaryInterceptor func(ctx context.Context, method string, args interface{}, reply interface{}) error
