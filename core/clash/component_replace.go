package clash

import (
	"fmt"
	"os"
	"time"
)

func WaitFileReleased(path string, timeout time.Duration) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {

		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			_ = f.Close()
			return nil
		}

		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}

	if lastErr == nil {
		return fmt.Errorf("тайм-аут ожидания освобождения файла")
	}

	return lastErr
}

func retryRename(oldPath, newPath string, attempts int, delay time.Duration) error {
	var lastErr error

	for i := 0; i < attempts; i++ {
		if err := os.Rename(oldPath, newPath); err == nil {
			return nil
		} else {
			lastErr = err
			time.Sleep(delay)
		}
	}

	return lastErr
}

func ReplaceFileWithBackup(newPath, destPath string) error {
	backupPath := destPath + ".bak"

	if err := WaitFileReleased(destPath, 5*time.Second); err != nil {
		return fmt.Errorf("не удалось дождаться освобождения целевого файла: %w", err)
	}

	_ = os.Remove(backupPath)

	if _, err := os.Stat(destPath); err == nil {
		if err := retryRename(destPath, backupPath, 10, 300*time.Millisecond); err != nil {
			return fmt.Errorf("не удалось создать резервную копию старого файла: %w", err)
		}
	}

	if err := retryRename(newPath, destPath, 10, 300*time.Millisecond); err != nil {

		if _, statErr := os.Stat(backupPath); statErr == nil {
			_ = retryRename(backupPath, destPath, 10, 300*time.Millisecond)
		}

		return fmt.Errorf("не удалось заменить файл новым: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}
