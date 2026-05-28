package client

import (
	"net"
	"strings"
)

// RuleEngine 规则引擎，用于决定连接方式
type RuleEngine struct {
	// 直连域名规则
	domainRules []string

	// 默认是否直连
	defaultDirect bool
}

// NewRuleEngine 创建一个新的规则引擎
func NewRuleEngine(domainRules []string, defaultDirect bool) *RuleEngine {
	return &RuleEngine{
		domainRules:   domainRules,
		defaultDirect: defaultDirect,
	}
}

// ShouldDirectConnect 判断是否应该直连
func (r *RuleEngine) ShouldDirectConnect(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// 无法解析地址，使用默认策略
		return r.defaultDirect
	}

	// 判断是否是IP地址
	ip := net.ParseIP(host)
	if ip != nil {
		// IP地址使用代理
		return false
	}

	// 域名检查规则
	for _, rule := range r.domainRules {
		if matchDomain(host, rule) {
			return true
		}
	}

	return r.defaultDirect
}

// AddDomainRule 添加一个域名规则
func (r *RuleEngine) AddDomainRule(rule string) {
	r.domainRules = append(r.domainRules, rule)
}

// SetDefaultDirect 设置默认直连策略
func (r *RuleEngine) SetDefaultDirect(defaultDirect bool) {
	r.defaultDirect = defaultDirect
}

// matchDomain 检查域名是否匹配规则
func matchDomain(domain, rule string) bool {
	// 如果规则以*开头，进行后缀匹配
	if strings.HasPrefix(rule, "*.") {
		suffix := rule[1:] // 跳过*
		return strings.HasSuffix(domain, suffix)
	}

	// 如果规则以.开头，进行后缀匹配
	if strings.HasPrefix(rule, ".") {
		return strings.HasSuffix(domain, rule)
	}

	// 精确匹配
	return domain == rule
}
