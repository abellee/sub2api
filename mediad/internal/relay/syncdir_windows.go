package relay

// Windows 不支持对目录执行 FlushFileBuffers；文件本身已经 Sync。
func SyncDirectory(string) error { return nil }
