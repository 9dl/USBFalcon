package main

import (
	"fmt"
	"github.com/fatih/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/StackExchange/wmi"
)

type Volume struct {
	DriveLetter string
	DeviceID    string
	Capacity    uint64
	FreeSpace   uint64
	FileSystem  string
	Label       string
}

const (
	CheckInterval   = 500 * time.Millisecond
	MaxUSBSize      = 128 * 1024 * 1024 * 1024 // 128 GB
	VolumeDupeCheck = true
)

var DestinationDir = "C:\\usb_data"
var DumpedUSBs = make(map[string]struct{})

func main() {
	clearConsole()
	for range time.Tick(CheckInterval) {
		var volumes []Volume
		query := "SELECT DriveLetter, DeviceID, Capacity, FreeSpace, FileSystem, Label FROM Win32_Volume WHERE DriveType = 2"
		if err := wmi.Query(query, &volumes); err != nil {
			errorCheck(err)
		}

		if VolumeDupeCheck {
			for i := 0; i < len(volumes); i++ {
				if _, ok := DumpedUSBs[volumes[i].DeviceID]; ok {
					volumes = append(volumes[:i], volumes[i+1:]...)
					i--
				} else {
					DumpedUSBs[volumes[i].DeviceID] = struct{}{}
				}
			}
		}

		for _, v := range volumes {
			DestinationDir = DestinationDir + "\\" + v.Label

			if v.DriveLetter == "" || v.Capacity > MaxUSBSize {
				continue
			}

			usedSpace := v.Capacity - v.FreeSpace

			fmt.Println("=========================================")
			printInfo("USB Device Detected", "")
			printInfo("Drive Letter", v.DriveLetter)
			printInfo("Device ID", v.DeviceID)
			printInfo("Capacity (GB)", formatBytes(int64(v.Capacity)))
			printInfo("Free Space", formatBytes(int64(v.FreeSpace)))
			printInfo("Used Space", formatBytes(int64(usedSpace)))
			printInfo("File System", v.FileSystem)
			printInfo("Label", v.Label)
			fmt.Println("=========================================")

			if err := copyUSBFiles(v.DriveLetter); err != nil {
				errorCheck(err)
			}

			fmt.Println("=========================================")
			printInfo("Done. Waiting for new USB to be plugged in.", "")
			fmt.Println("=========================================")

		}
	}
}

func clearConsole() {
	cmd := exec.Command("cmd", "/c", "title USBFalcon - github.com/9dl/USBFalcon")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()

	cmd = exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}

func formatBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30: // GB
		return fmt.Sprintf("%.2f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20: // MB
		return fmt.Sprintf("%.2f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10: // KB
		return fmt.Sprintf("%.2f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

func copyUSBFiles(volumePath string) error {
	if err := os.MkdirAll(DestinationDir, 0755); err != nil {
		errorCheck(err)
	}

	return filepath.Walk(volumePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && (info.Name() == "System Volume Information" || info.Name() == "$RECYCLE.BIN") {
			return filepath.SkipDir
		}

		relativePath, err := filepath.Rel(volumePath, path)
		if err != nil {
			return fmt.Errorf("calculating relative path for %q: %v", path, err)
		}

		dst := filepath.Join(DestinationDir, relativePath)

		if info.IsDir() {
			printInfo("Creating Directory", path)
			if err := os.MkdirAll(dst, os.ModePerm); err != nil {
				return fmt.Errorf("creating directory %q: %v", dst, err)
			}
		} else {
			printInfo("Copying File", path)
			if err := copyFile(path, dst); err != nil {
				return fmt.Errorf("copying file %q to %q: %v", path, dst, err)
			}
		}

		return nil
	})
}

func printInfo(label string, value interface{}) {
	message := fmt.Sprintf(
		"%s%s%s %s",
		color.BlueString("["),
		color.CyanString("USBFalcon"),
		color.BlueString("]"),
		color.GreenString(label),
	)
	if value != "" {
		message += fmt.Sprintf("%s%s", color.WhiteString(": "), color.YellowString(fmt.Sprintf("%v", value)))
	}
	fmt.Println(message)
}

func errorCheck(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func(in *os.File) {
		err := in.Close()
		errorCheck(err)
	}(in)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		errorCheck(err)
	}(out)

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if srcInfo, err := os.Stat(src); err == nil {
		return os.Chmod(dst, srcInfo.Mode())
	}
	return nil
}
