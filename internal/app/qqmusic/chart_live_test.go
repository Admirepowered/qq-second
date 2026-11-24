package qqmusic

// 榜单映射回归测试（真实链路）：每个配置的 topId 必须返回同名榜单且非空。
// topId 写错时服务端要么返回别的榜单、要么 cgi 10007，这个测试能直接抓住。

import (
	"os"
	"testing"
)

func TestLiveChartMapping(t *testing.T) {
	if os.Getenv("QQ_LIVE") == "" {
		t.Skip("set QQ_LIVE=1 to run live test")
	}
	c := NewClient()
	for _, ch := range Charts() {
		detail, err := c.ChartDetail(ch.ID)
		if err != nil {
			t.Errorf("topId=%d (%s) 请求失败: %v", ch.ID, ch.Name, err)
			continue
		}
		if detail.Info.Title != ch.Name {
			t.Errorf("topId=%d 名称不符: 配置 %q, 服务端返回 %q", ch.ID, ch.Name, detail.Info.Title)
		}
		if len(detail.Tracks) == 0 {
			t.Errorf("topId=%d (%s) 没有歌曲", ch.ID, ch.Name)
		}
	}
}
