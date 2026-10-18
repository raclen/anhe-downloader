//go:build !windows

package server

// 非 Windows 系统降级实现
func SelectFolderNative(title string) (string, error) {
	return "", nil
}
