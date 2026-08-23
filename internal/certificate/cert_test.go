package certificate

import (
	"testing"
	"time"

	"task185-nvwear/internal/model"
)

func TestIssue(t *testing.T) {
	i := New()
	c := i.Issue("c1", "policy", "p1", "cfg1", "hash1", `{"die":2}`, `[{"lpn":1}]`)
	if c.Status != model.CertPublished {
		t.Fatalf("status = %s", c.Status)
	}
	if c.ConfigSnapshot == "" || c.MappingSnapshot == "" || c.PlanHash != "hash1" {
		t.Fatalf("证书快照不完整: %+v", c)
	}
	if c.IssuedAt.IsZero() {
		t.Fatal("签发时间为零")
	}
	// 配置快照与映射快照必须各归其位，不得错位。
	if c.ConfigSnapshot != `{"die":2}` {
		t.Fatalf("配置快照错位: got %s", c.ConfigSnapshot)
	}
	if c.MappingSnapshot != `[{"lpn":1}]` {
		t.Fatalf("映射快照错位: got %s", c.MappingSnapshot)
	}
}

func TestRevoke(t *testing.T) {
	i := New()
	now := time.Now()
	i.Now = func() time.Time { return now }
	c := i.Issue("c1", "policy", "p1", "cfg1", "h", "{}", "[]")
	if err := i.Revoke(c, "geometry updated"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if c.Status != model.CertRevoked || c.RevokeReason != "geometry updated" {
		t.Fatalf("撤销后: %+v", c)
	}
	if c.RevokedAt == nil || !c.RevokedAt.Equal(now) {
		t.Fatal("撤销时间未设置")
	}
	if err := i.Revoke(c, "again"); err == nil {
		t.Fatal("重复撤销应报错")
	}
}

func TestRevokeRequiresReason(t *testing.T) {
	i := New()
	c := i.Issue("c1", "policy", "p1", "cfg1", "h", "{}", "[]")
	if err := i.Revoke(c, ""); err == nil {
		t.Fatal("空原因应报错")
	}
}
