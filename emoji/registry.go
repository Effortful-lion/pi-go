package emoji

// Registry 主题注册表：管理主题的注册和查询
type Registry struct {
	themes map[string]Theme
}

// NewRegistry 创建新的主题注册表
func NewRegistry() *Registry {
	return &Registry{
		themes: make(map[string]Theme),
	}
}

// Register 注册一个主题
// 如果同名主题已存在，会被覆盖
func (r *Registry) Register(theme Theme) {
	r.themes[theme.Name] = theme
}

// Get 根据名称获取主题
// 第二个返回值表示主题是否存在
func (r *Registry) Get(name string) (Theme, bool) {
	theme, ok := r.themes[name]
	return theme, ok
}

// Has 检查主题是否存在
func (r *Registry) Has(name string) bool {
	_, ok := r.themes[name]
	return ok
}
