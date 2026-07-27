package lg

import (
	"fmt"
	"sync"
)

// Router 按模块名将日志分流到不同的 Writer。
//
// 典型用法：
//
//	router := lg.NewRouter(lg.NewConsoleWriter(os.Stdout, lg.LevelInfo))
//	router.Route("user", userFileWriter)   // user 模块 → user.log
//	router.Route("shop", shopFileWriter)   // shop 模块 → shop.log
//
//	logger := lg.New(router)
//
//	// 使用：
//	logger.Module("user").Info("用户登录成功")
//	logger.Module("shop").Warn("库存不足")
type Router struct {
	defaultWriter Writer
	routes        map[string]Writer
	mu            sync.RWMutex
}

// NewRouter 创建一个日志路由器。
// defaultWriter: 未匹配到模块时的默认输出（通常为控制台）
func NewRouter(defaultWriter Writer) *Router {
	if defaultWriter == nil {
		defaultWriter = NewConsoleWriter(nil, LevelInfo)
	}
	return &Router{
		defaultWriter: defaultWriter,
		routes:        make(map[string]Writer),
	}
}

// Route 注册一个模块路由，将该模块的日志输出到指定 Writer。
// 多次注册同名模块会覆盖之前的设置。
func (r *Router) Route(module string, writer Writer) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[module] = writer
	return r
}

// Unroute 取消某个模块的路由，该模块将回退到默认 Writer。
func (r *Router) Unroute(module string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.routes, module)
}

// Resolve 根据模块名查找对应的 Writer。
// 如果未找到路由，返回默认 Writer。
func (r *Router) Resolve(module string) Writer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if wr, ok := r.routes[module]; ok {
		return wr
	}
	return r.defaultWriter
}

// Routes 返回当前所有路由的模块名列表。
func (r *Router) Routes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.routes))
	for k := range r.routes {
		keys = append(keys, k)
	}
	return keys
}

// Level 返回所有路由 Writer 中的最低级别。
func (r *Router) Level() Level {
	r.mu.RLock()
	defer r.mu.RUnlock()
	minLevel := r.defaultWriter.Level()
	for _, wr := range r.routes {
		if wr.Level() < minLevel {
			minLevel = wr.Level()
		}
	}
	return minLevel
}

// Write 根据 entry.Module 路由到对应的 Writer。
func (r *Router) Write(entry *Entry) error {
	wr := r.Resolve(entry.Module)
	if entry.Level < wr.Level() {
		return nil
	}
	if err := wr.Write(entry); err != nil {
		return fmt.Errorf("lg router write to %q: %w", entry.Module, err)
	}
	return nil
}

// Close 关闭路由器和所有已注册的 Writer。
func (r *Router) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, wr := range r.routes {
		_ = wr.Close()
	}
	return r.defaultWriter.Close()
}
