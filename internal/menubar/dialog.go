package menubar

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func pickDirectory() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return pickDirectoryMac()
	case "linux":
		return pickDirectoryLinux()
	case "windows":
		return pickDirectoryWindows()
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func pickDirectoryMac() (string, error) {
	out, err := exec.Command("osascript", "-e",
		`POSIX path of (choose folder with prompt "Select titles directory")`,
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func pickDirectoryLinux() (string, error) {
	out, err := exec.Command("zenity", "--file-selection", "--directory",
		"--title=Select titles directory",
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func pickDirectoryWindows() (string, error) {
	script := `
Add-Type -AssemblyName System.Windows.Forms
$fb = New-Object System.Windows.Forms.FolderBrowserDialog
$fb.Description = 'Select titles directory'
if ($fb.ShowDialog() -eq 'OK') { Write-Output $fb.SelectedPath }
`
	out, err := exec.Command("powershell", "-Command", script).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
