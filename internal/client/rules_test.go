package client

import "testing"

func TestNewRuleEngine(t *testing.T) {
	rules := []string{"*.example.com", ".google.com"}
	engine := NewRuleEngine(rules, true)

	if engine == nil {
		t.Fatal("NewRuleEngine返回nil")
	}

	if len(engine.domainRules) != 2 {
		t.Errorf("规则数量错误，期望: 2, 实际: %d", len(engine.domainRules))
	}

	if !engine.defaultDirect {
		t.Error("默认直连配置错误")
	}
}

func TestMatchDomain(t *testing.T) {
	testCases := []struct {
		domain   string
		rule     string
		expected bool
	}{
		// 精确匹配
		{"example.com", "example.com", true},
		{"example.com", "example.org", false},

		// 通配符匹配
		{"sub.example.com", "*.example.com", true},
		{"othersub.example.com", "*.example.com", true},
		{"example.com", "*.example.com", false},
		{"sub.other.com", "*.example.com", false},

		// 后缀匹配
		{"www.google.com", ".google.com", true},
		{"google.com", ".google.com", false},
		{"othergoogle.com", ".google.com", false},
	}

	for _, tc := range testCases {
		result := matchDomain(tc.domain, tc.rule)
		if result != tc.expected {
			t.Errorf("matchDomain(%s, %s) = %v, 期望: %v", tc.domain, tc.rule, result, tc.expected)
		}
	}
}

func TestShouldDirectConnect(t *testing.T) {
	rules := []string{"*.example.com", ".google.com", "direct.com"}
	engine := NewRuleEngine(rules, true)

	testCases := []struct {
		addr     string
		expected bool
	}{
		// IP地址应走代理
		{"8.8.8.8:80", false},
		{"127.0.0.1:8080", false},
		{"[::1]:80", false},

		// 匹配规则的域名应直连
		{"www.example.com:443", true},
		{"api.example.com:80", true},
		{"mail.google.com:443", true},
		{"direct.com:80", true},

		// 不匹配规则的域名，使用默认策略(true)
		{"other.com:80", true},
		{"unknown.org:443", true},

		// 无法解析的地址应使用默认策略
		{"malformed", true},
	}

	for _, tc := range testCases {
		result := engine.ShouldDirectConnect(tc.addr)
		if result != tc.expected {
			t.Errorf("ShouldDirectConnect(%s) = %v, 期望: %v", tc.addr, result, tc.expected)
		}
	}

	// 测试默认策略为false的情况
	engine.SetDefaultDirect(false)

	if engine.ShouldDirectConnect("other.com:80") {
		t.Error("修改默认策略后，不匹配规则的域名应走代理")
	}
}

func TestRuleEngineAddRule(t *testing.T) {
	engine := NewRuleEngine([]string{}, false)

	// 初始没有规则
	if engine.ShouldDirectConnect("test.com:80") {
		t.Error("初始应该走代理")
	}

	// 添加规则
	engine.AddDomainRule("test.com")

	// 现在应该直连
	if !engine.ShouldDirectConnect("test.com:80") {
		t.Error("添加规则后应该直连")
	}
}
