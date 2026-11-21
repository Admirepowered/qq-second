//go:build dev

package app

// wails dev 构建使用独立互斥量/信号名，避免与正式版互踢。
const mutexName = `Local\QQSecondSingleInstanceDev`

const signalFile = "show-dev.signal"
