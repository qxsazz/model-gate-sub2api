//go:build unit

package service

import "testing"

func TestVIPPrivateName(t *testing.T) {
	for _, input := range []string{"1783613892@qq.com", "张三工作室", "a", "", "🙂test"} {
		got := VIPPrivateName(input)
		if got == input || got == "" {
			t.Errorf("identity exposed: %q -> %q", input, got)
		}
	}
	if got := VIPPrivateName("1783613892@qq.com"); got != "17***" {
		t.Fatal(got)
	}
}
