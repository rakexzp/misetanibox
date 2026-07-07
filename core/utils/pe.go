package utils

import (
	"fmt"
	"io"
	"os"
)

// ValidateWindowsPE 校验路径上的文件是否为有效的 Windows 可执行文件或 DLL
func ValidateWindowsPE(path string, minSize int64, maxSize int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	header := make([]byte, 2)
	if _, err := io.ReadFull(f, header); err != nil {
		return err
	}

	if header[0] != 'M' || header[1] != 'Z' {
		return fmt.Errorf("не является допустимым PE-файлом Windows (нет сигнатуры MZ)")
	}

	info, err := f.Stat()
	if err != nil {
		return err
	}

	if minSize > 0 && info.Size() < minSize {
		return fmt.Errorf("файл слишком мал: %d байт (минимум %d байт)", info.Size(), minSize)
	}

	if maxSize > 0 && info.Size() > maxSize {
		return fmt.Errorf("файл слишком велик: %d байт (максимум %d байт)", info.Size(), maxSize)
	}

	return nil
}
