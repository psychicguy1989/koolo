package shopbot

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

// D2RProcess represents a detected Diablo 2 Resurrected process.
type D2RProcess struct {
	PID           uint32   `json:"pid"`
	HWND          win.HWND `json:"-"`
	HWNDValue     uintptr  `json:"hwnd"`
	WindowTitle   string   `json:"windowTitle"`
	CharacterName string   `json:"characterName"`
	IsInGame      bool     `json:"isInGame"`
}

// FindAllD2RProcesses enumerates all running D2R.exe processes and their windows.
func FindAllD2RProcesses() ([]D2RProcess, error) {
	var processes []D2RProcess

	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("creating process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snapshot, &entry); err != nil {
		return nil, fmt.Errorf("reading first process: %w", err)
	}

	for {
		exeName := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if exeName == "d2r.exe" {
			hwnd := findWindowForPID(entry.ProcessID)
			title := getWindowTitleForHWND(hwnd)

			processes = append(processes, D2RProcess{
				PID:         entry.ProcessID,
				HWND:        hwnd,
				HWNDValue:   uintptr(hwnd),
				WindowTitle: title,
			})
		}

		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if err == windows.ERROR_NO_MORE_FILES {
				break
			}
			return nil, fmt.Errorf("reading next process: %w", err)
		}
	}

	return processes, nil
}

// findWindowForPID finds the main window handle for a given process ID.
func findWindowForPID(pid uint32) win.HWND {
	var foundHWND win.HWND

	cb := syscall.NewCallback(func(hwnd windows.HWND, lParam uintptr) uintptr {
		var windowPID uint32
		windows.GetWindowThreadProcessId(hwnd, &windowPID)
		if windowPID == pid {
			// Check if it's a visible, top-level window
			if win.IsWindowVisible(win.HWND(hwnd)) {
				foundHWND = win.HWND(hwnd)
				return 0 // Stop enumeration
			}
		}
		return 1 // Continue
	})

	windows.EnumWindows(cb, nil)
	return foundHWND
}

// getWindowTitleForHWND gets the window title text.
func getWindowTitleForHWND(hwnd win.HWND) string {
	if hwnd == 0 {
		return "(no window)"
	}

	user32 := windows.NewLazySystemDLL("user32.dll")
	getWindowText := user32.NewProc("GetWindowTextW")

	var title [256]uint16
	getWindowText.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(&title[0])),
		uintptr(len(title)),
	)

	return syscall.UTF16ToString(title[:])
}

// FormatProcessList returns a human-readable list of detected D2R processes.
func FormatProcessList(processes []D2RProcess) string {
	if len(processes) == 0 {
		return "No D2R processes found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d D2R process(es):\n", len(processes)))
	for i, p := range processes {
		charInfo := p.CharacterName
		if charInfo == "" {
			charInfo = "(detecting...)"
		}
		sb.WriteString(fmt.Sprintf("  [%d] PID: %d | Window: %q | Character: %s\n",
			i+1, p.PID, p.WindowTitle, charInfo))
	}
	return sb.String()
}
