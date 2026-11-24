//go:build !windows

package audiocap

import (
	"context"
	"errors"
)

// ErrNotSupported 表示当前平台不支持系统声音采集。
var ErrNotSupported = errors.New("当前平台不支持采集系统声音（仅 Windows 可用）")

// CaptureSystem 在非 Windows 平台直接返回不支持。
func CaptureSystem(ctx context.Context, seconds int) ([]int16, error) {
	return nil, ErrNotSupported
}
