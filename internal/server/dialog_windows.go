//go:build windows

package server

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	ole32            = syscall.NewLazyDLL("ole32.dll")
	coInitializeEx   = ole32.NewProc("CoInitializeEx")
	coUninitialize   = ole32.NewProc("CoUninitialize")
	coCreateInstance = ole32.NewProc("CoCreateInstance")
	coTaskMemFree    = ole32.NewProc("CoTaskMemFree")

	shell32              = syscall.NewLazyDLL("shell32.dll")
	shBrowseForFolderW   = shell32.NewProc("SHBrowseForFolderW")
	shGetPathFromIDListW = shell32.NewProc("SHGetPathFromIDListW")
)

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

var (
	clsidFileOpenDialog = guid{0xDC1C5A9C, 0xE88A, 0x4DDE, [8]byte{0xA5, 0xA1, 0x60, 0xF8, 0x2A, 0x20, 0xAE, 0xF7}}
	iidIFileOpenDialog  = guid{0xD57C7288, 0xD4AD, 0x4768, [8]byte{0xBE, 0x02, 0x9D, 0x96, 0x95, 0x32, 0xD9, 0x60}}
)

const (
	clsctxInprocServer = 0x1
	fosPickFolders     = 0x20
	fosForceFilesystem = 0x40
	sigdnFilesyspath   = 0x80058000

	bifReturnOnlyFSDirs = 0x0001
	bifNewDialogStyle   = 0x0040
)

type browseInfoW struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// SelectFolderNative 在单独的 OS 线程中直接调用 Windows 原生 IFileOpenDialog 对话框
// 彻底杜绝 cmd/powershell 黑色控制台弹窗与中文乱码问题
func SelectFolderNative(title string) (string, error) {
	resultChan := make(chan struct {
		path string
		err  error
	}, 1)

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// 1. 初始化 COM STA 线程模型
		hr, _, _ := coInitializeEx.Call(0, 0x2 /* COINIT_APARTMENTTHREADED */)
		if int32(hr) < 0 && uint32(hr) != 0x80010106 /* RPC_E_CHANGED_MODE */ {
			resultChan <- struct {
				path string
				err  error
			}{"", fmt.Errorf("CoInitializeEx failed: 0x%x", hr)}
			return
		}
		defer coUninitialize.Call()

		// 2. 优先尝试现代 Windows 10/11 的 IFileOpenDialog (FOS_PICKFOLDERS)
		var pDialog *uintptr
		hr, _, _ = coCreateInstance.Call(
			uintptr(unsafe.Pointer(&clsidFileOpenDialog)),
			0,
			clsctxInprocServer,
			uintptr(unsafe.Pointer(&iidIFileOpenDialog)),
			uintptr(unsafe.Pointer(&pDialog)),
		)

		if int32(hr) >= 0 && pDialog != nil {
			dialogVtbl := *(**[30]uintptr)(unsafe.Pointer(pDialog))
			release := dialogVtbl[2]
			show := dialogVtbl[3]
			setOptions := dialogVtbl[9]
			setTitle := dialogVtbl[17]
			getResult := dialogVtbl[20]

			defer syscall.SyscallN(release, uintptr(unsafe.Pointer(pDialog)))

			// 原生宽字符 UTF-16 标题
			titleUtf16, _ := syscall.UTF16PtrFromString(title)
			syscall.SyscallN(setTitle, uintptr(unsafe.Pointer(pDialog)), uintptr(unsafe.Pointer(titleUtf16)))
			syscall.SyscallN(setOptions, uintptr(unsafe.Pointer(pDialog)), uintptr(fosPickFolders|fosForceFilesystem))

			rShow, _, _ := syscall.SyscallN(show, uintptr(unsafe.Pointer(pDialog)), 0)
			if uint32(rShow) == 0x800704C7 {
				// 用户取消
				resultChan <- struct {
					path string
					err  error
				}{"", nil}
				return
			}
			if int32(rShow) >= 0 {
				var pItem *uintptr
				rGetResult, _, _ := syscall.SyscallN(getResult, uintptr(unsafe.Pointer(pDialog)), uintptr(unsafe.Pointer(&pItem)))
				if int32(rGetResult) >= 0 && pItem != nil {
					itemVtbl := *(**[10]uintptr)(unsafe.Pointer(pItem))
					itemRelease := itemVtbl[2]
					getDisplayName := itemVtbl[5]
					defer syscall.SyscallN(itemRelease, uintptr(unsafe.Pointer(pItem)))

					var pszPath *uint16
					rName, _, _ := syscall.SyscallN(getDisplayName, uintptr(unsafe.Pointer(pItem)), uintptr(sigdnFilesyspath), uintptr(unsafe.Pointer(&pszPath)))
					if int32(rName) >= 0 && pszPath != nil {
						defer coTaskMemFree.Call(uintptr(unsafe.Pointer(pszPath)))
						selectedPath := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(pszPath))[:])
						resultChan <- struct {
							path string
							err  error
						}{selectedPath, nil}
						return
					}
				}
			}
		}

		// 3. 降级方案：调用 SHBrowseForFolderW (纯 Win32 原生，同样无任何黑框和乱码)
		titleUtf16, _ := syscall.UTF16PtrFromString(title)
		bi := browseInfoW{
			lpszTitle: titleUtf16,
			ulFlags:   bifReturnOnlyFSDirs | bifNewDialogStyle,
		}
		pidl, _, _ := shBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl == 0 {
			resultChan <- struct {
				path string
				err  error
			}{"", nil}
			return
		}
		defer coTaskMemFree.Call(pidl)

		var pathBuf [1024]uint16
		ret, _, _ := shGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
		if ret != 0 {
			selectedPath := syscall.UTF16ToString(pathBuf[:])
			resultChan <- struct {
				path string
				err  error
			}{selectedPath, nil}
			return
		}

		resultChan <- struct {
			path string
			err  error
		}{"", nil}
	}()

	res := <-resultChan
	return res.path, res.err
}
