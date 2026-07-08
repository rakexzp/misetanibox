package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"goclashz/core/clash"
	"goclashz/core/utils"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manifest struct {
	App           string   `json:"app"`
	BackupVersion int      `json:"backupVersion"`
	AppVersion    string   `json:"appVersion"`
	CreatedAt     int64    `json:"createdAt"`
	Contains      []string `json:"contains"`
}

const CurrentBackupVersion = 2

func Export(dataDir, destPath string, appVersion string) error {

	stagingDir, err := os.MkdirTemp(filepath.Dir(dataDir), ".goclashz-export-*")
	if err != nil {
		return fmt.Errorf("не удалось создать временный каталог экспорта: %v", err)
	}
	defer os.RemoveAll(stagingDir)

	targets := []string{
		"Settings",
		"Subscriptions",
		"profiles",
		"config.yaml",
		"theme_setting.txt",
	}

	contains := []string{}

	for _, target := range targets {
		src := filepath.Join(dataDir, target)
		dst := filepath.Join(stagingDir, target)

		info, err := os.Stat(src)
		if err != nil {
			continue
		}

		if target == "profiles" {

			clash.IndexLock.RLock()
			err = copyDir(src, dst)
			clash.IndexLock.RUnlock()
		} else if info.IsDir() {
			err = copyDir(src, dst)
		} else {
			err = copyFile(src, dst)
		}

		if err == nil {
			contains = append(contains, strings.ToLower(target))
		}
	}

	manifest := Manifest{
		App:           "Misetanibox",
		BackupVersion: CurrentBackupVersion,
		AppVersion:    appVersion,
		CreatedAt:     time.Now().Unix(),
		Contains:      contains,
	}
	mBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("не удалось сформировать manifest резервной копии: %w", err)
	}
	if err := utils.WriteFileAtomic(filepath.Join(stagingDir, "manifest.json"), mBytes, 0644); err != nil {
		return fmt.Errorf("не удалось записать manifest резервной копии: %w", err)
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл резервной копии: %v", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	return filepath.Walk(stagingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(stagingDir, path)
		w, err := zw.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = w.Write(content)
		return err
	})
}

type BackupStagingResult struct {
	Index    []clash.SubIndexItem
	HasIndex bool
	IndexErr error
}

func RestoreTransactional(ctx context.Context, dataDir, archivePath, mode string) error {

	if err := validateRestoreMode(mode); err != nil {
		return err
	}

	workDir, err := os.MkdirTemp(filepath.Dir(dataDir), ".goclashz-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	stagingDir := filepath.Join(workDir, "staging")
	rollbackDir := filepath.Join(workDir, "rollback")
	os.MkdirAll(stagingDir, 0755)
	os.MkdirAll(rollbackDir, 0755)

	stagingResult, err := extractAndNormalizeToStaging(archivePath, stagingDir)
	if err != nil {
		return err
	}

	if mode == "all" || mode == "subs" {
		if !stagingResult.HasIndex {
			rebuilt, err := rebuildIndexFromSubscriptions(stagingDir)
			if err != nil {
				return fmt.Errorf("в резервной копии отсутствует profiles/index.json, и восстановить индекс из Subscriptions не удалось: %w", err)
			}
			stagingResult.Index = rebuilt
			stagingResult.HasIndex = true
		}

		if stagingResult.IndexErr != nil {
			return fmt.Errorf("не удалось разобрать индекс подписок в резервной копии: %v", stagingResult.IndexErr)
		}
	}

	if err := validateManifest(stagingDir); err != nil {
		return err
	}

	plan := buildRestorePlan(mode)

	if err := validateRestorePlanInputs(stagingDir, plan, mode); err != nil {
		return err
	}

	if err := backupCurrentTargets(dataDir, plan, rollbackDir); err != nil {
		return fmt.Errorf("не удалось создать резервную копию текущих данных, восстановление отменено: %v", err)
	}

	if mode == "subs-merge" && len(stagingResult.Index) == 0 {
		rebuilt, err := rebuildIndexFromSubscriptions(stagingDir)
		if err == nil {
			stagingResult.Index = rebuilt
		}
	}

	if err := applyRestorePlan(dataDir, stagingDir, plan, mode, stagingResult.Index); err != nil {

		_ = rollbackRestorePlan(dataDir, rollbackDir, plan)
		return fmt.Errorf("восстановление не удалось, выполнен автоматический откат: %v", err)
	}

	return nil
}

type RestorePlan struct {
	ReplaceDirs  []string
	ReplaceFiles []string
	MergeDirs    []string
}

func buildRestorePlan(mode string) *RestorePlan {
	plan := &RestorePlan{}
	switch mode {
	case "all":
		plan.ReplaceDirs = []string{"Settings", "Subscriptions", "profiles"}
		plan.ReplaceFiles = []string{"config.yaml", "theme_setting.txt"}
	case "settings":
		plan.ReplaceDirs = []string{"Settings"}
		plan.ReplaceFiles = []string{"theme_setting.txt"}
	case "subs":
		plan.ReplaceDirs = []string{"Subscriptions", "profiles"}
		plan.ReplaceFiles = []string{"config.yaml"}
	case "subs-merge":
		plan.MergeDirs = []string{"Subscriptions"}
	}
	return plan
}

func validateRestorePlanInputs(stagingDir string, plan *RestorePlan, mode string) error {
	required := append([]string{}, plan.ReplaceDirs...)
	required = append(required, plan.ReplaceFiles...)

	optional := map[string]bool{
		"theme_setting.txt": true,
	}

	for _, target := range required {
		if optional[target] {
			continue
		}

		if _, err := os.Stat(filepath.Join(stagingDir, target)); err != nil {
			return fmt.Errorf("в архиве резервной копии отсутствует цель, необходимая для режима восстановления %s: %s", mode, target)
		}
	}

	return nil
}

func applyRestorePlan(dataDir, stagingDir string, plan *RestorePlan, mode string, backupIndex []clash.SubIndexItem) error {

	for _, dir := range plan.ReplaceDirs {
		src := filepath.Join(stagingDir, dir)
		dst := filepath.Join(dataDir, dir)
		if _, err := os.Stat(src); err == nil {
			_ = os.RemoveAll(dst)
			if err := copyDir(src, dst); err != nil {
				return err
			}
		}
	}

	for _, file := range plan.ReplaceFiles {
		src := filepath.Join(stagingDir, file)
		dst := filepath.Join(dataDir, file)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, dst); err != nil {
				return err
			}
		}
	}

	if mode == "subs-merge" {
		src := filepath.Join(stagingDir, "Subscriptions")
		dst := filepath.Join(dataDir, "Subscriptions")
		if _, err := os.Stat(src); err == nil {
			if err := copyDir(src, dst); err != nil {
				return err
			}
		}
	}

	switch mode {
	case "all", "subs":

		return clash.ReplaceSubIndex(backupIndex)
	case "subs-merge":

		return mergeBackupIndex(backupIndex)
	}

	return nil
}

func backupCurrentTargets(dataDir string, plan *RestorePlan, rollbackDir string) error {
	allTargets := append([]string{}, plan.ReplaceDirs...)
	allTargets = append(allTargets, plan.ReplaceFiles...)
	allTargets = append(allTargets, plan.MergeDirs...)

	for _, target := range allTargets {
		src := filepath.Join(dataDir, target)
		dst := filepath.Join(rollbackDir, target)

		info, err := os.Stat(src)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("не удалось прочитать текущую цель %s: %w", target, err)
		}

		if info.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return fmt.Errorf("не удалось создать резервную копию каталога %s: %w", target, err)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("не удалось создать резервную копию файла %s: %w", target, err)
			}
		}
	}

	return nil
}

func rollbackRestorePlan(dataDir, rollbackDir string, plan *RestorePlan) error {
	allTargets := append([]string{}, plan.ReplaceDirs...)
	allTargets = append(allTargets, plan.ReplaceFiles...)
	allTargets = append(allTargets, plan.MergeDirs...)

	for _, target := range allTargets {
		src := filepath.Join(rollbackDir, target)
		dst := filepath.Join(dataDir, target)

		_ = os.RemoveAll(dst)

		if _, err := os.Stat(src); err == nil {
			info, _ := os.Stat(src)
			if info.IsDir() {
				_ = copyDir(src, dst)
			} else {
				_ = copyFile(src, dst)
			}
		}
	}
	return nil
}

func validateRestoreMode(mode string) error {
	switch mode {
	case "all", "settings", "subs", "subs-merge":
		return nil
	default:
		return fmt.Errorf("неподдерживаемый режим восстановления: %s", mode)
	}
}

func validateManifest(stagingDir string) error {
	path := filepath.Join(stagingDir, "manifest.json")

	if _, err := os.Stat(path); os.IsNotExist(err) {

		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("не удалось прочитать manifest резервной копии: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("manifest резервной копии повреждён: %w", err)
	}

	if app := strings.TrimSpace(manifest.App); app != "Misetanibox" && app != "GoclashZ" {
		return fmt.Errorf("проверка файла резервной копии не пройдена: приложение не совпадает (%s)", manifest.App)
	}

	if manifest.BackupVersion <= 0 {
		return fmt.Errorf("в manifest резервной копии нет корректного backupVersion")
	}

	if manifest.BackupVersion > CurrentBackupVersion {
		return fmt.Errorf(
			"версия резервной копии слишком новая, не поддерживается текущей программой: backupVersion=%d, supported=%d",
			manifest.BackupVersion,
			CurrentBackupVersion,
		)
	}

	return nil
}

func extractAndNormalizeToStaging(archivePath, stagingDir string) (*BackupStagingResult, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	result := &BackupStagingResult{}

	const (
		maxRestoreFiles  = 1000
		maxRestoreTotal  = 300 * 1024 * 1024
		maxRestoreSingle = 50 * 1024 * 1024
	)

	if len(zr.File) > maxRestoreFiles {
		return nil, fmt.Errorf("слишком много файлов в архиве резервной копии")
	}

	var totalUncompressed uint64

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		destRel, _, ok := normalizeBackupEntry(f.Name)
		if !ok {
			continue
		}

		if f.UncompressedSize64 > maxRestoreSingle {
			return nil, fmt.Errorf("файл слишком большой: %s", f.Name)
		}
		totalUncompressed += f.UncompressedSize64
		if totalUncompressed > maxRestoreTotal {
			return nil, fmt.Errorf("общий размер архива резервной копии превышает лимит")
		}

		if destRel == "profiles/index.json" {
			result.HasIndex = true
			rc, err := f.Open()
			if err == nil {
				result.IndexErr = json.NewDecoder(rc).Decode(&result.Index)
				rc.Close()
			} else {
				result.IndexErr = err
			}
		}

		destPath, err := safeJoinUnder(stagingDir, destRel)
		if err != nil {
			return nil, err
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, err
		}

		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		dstFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			rc.Close()
			return nil, err
		}
		_, err = io.Copy(dstFile, rc)
		closeErr1 := dstFile.Close()
		closeErr2 := rc.Close()
		if err != nil {
			return nil, err
		}
		if closeErr1 != nil {
			return nil, closeErr1
		}
		if closeErr2 != nil {
			return nil, closeErr2
		}
	}
	return result, nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()
	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeBackupEntry(name string) (destRel string, kind string, ok bool) {
	n := filepath.ToSlash(filepath.Clean(name))
	lower := strings.ToLower(n)
	switch {
	case lower == "theme_setting.txt":
		return "theme_setting.txt", "theme", true
	case lower == "config.yaml":
		return "config.yaml", "config", true
	case lower == "manifest.json":
		return "manifest.json", "manifest", true
	case lower == "behavior.json":
		return filepath.ToSlash(filepath.Join("Settings", "user_behavior.json")), "settings", true
	case lower == "dns.json":
		return filepath.ToSlash(filepath.Join("Settings", "user_dns.json")), "settings", true
	case lower == "network.json":
		return filepath.ToSlash(filepath.Join("Settings", "user_network.json")), "settings", true
	case lower == "tun.json":
		return filepath.ToSlash(filepath.Join("Settings", "user_tun.json")), "settings", true
	case strings.HasPrefix(lower, "settings/"):
		parts := strings.Split(n, "/")
		rest := strings.Join(parts[1:], "/")
		rest, ok := cleanBackupRest(rest)
		if !ok {
			return "", "", false
		}
		switch strings.ToLower(rest) {
		case "behavior.json":
			rest = "user_behavior.json"
		case "dns.json":
			rest = "user_dns.json"
		case "network.json":
			rest = "user_network.json"
		case "tun.json":
			rest = "user_tun.json"
		}
		return filepath.ToSlash(filepath.Join("Settings", rest)), "settings", true
	case strings.HasPrefix(lower, "subscriptions/"):
		parts := strings.Split(n, "/")
		rest := strings.Join(parts[1:], "/")
		rest, ok := cleanBackupRest(rest)
		if !ok {
			return "", "", false
		}
		return filepath.ToSlash(filepath.Join("Subscriptions", rest)), "subs", true
	case strings.HasPrefix(lower, "profiles/"):
		parts := strings.Split(n, "/")
		rest := strings.Join(parts[1:], "/")
		rest, ok := cleanBackupRest(rest)
		if !ok {
			return "", "", false
		}
		return filepath.ToSlash(filepath.Join("profiles", rest)), "profiles", true
	}
	return "", "", false
}

func mergeBackupIndex(backupIndex []clash.SubIndexItem) error {
	if len(backupIndex) == 0 {
		return nil
	}
	if err := clash.LoadIndex(); err != nil {
		return err
	}

	return clash.UpdateSubIndex(func(localIndex []clash.SubIndexItem) ([]clash.SubIndexItem, error) {
		localIndexMap := make(map[string]int)
		for i, item := range localIndex {
			localIndexMap[item.ID] = i
		}

		for _, bItem := range backupIndex {
			if idx, exists := localIndexMap[bItem.ID]; exists {
				localIndex[idx] = bItem
			} else {
				localIndex = append(localIndex, bItem)
			}
		}

		return localIndex, nil
	})
}

func safeJoinUnder(base, rel string) (string, error) {
	rel = filepath.FromSlash(rel)

	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("абсолютный путь отклонён: %s", rel)
	}

	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("обход пути (path traversal) отклонён: %s", rel)
	}

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}

	targetAbs, err := filepath.Abs(filepath.Join(baseAbs, clean))
	if err != nil {
		return "", err
	}

	backRel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil || backRel == ".." || strings.HasPrefix(backRel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("целевой путь выходит за пределы staging: %s", rel)
	}

	return targetAbs, nil
}

func cleanBackupRest(rest string) (string, bool) {
	rest = filepath.ToSlash(filepath.Clean(rest))
	if rest == "." || rest == ".." || strings.HasPrefix(rest, "../") {
		return "", false
	}
	if strings.Contains(rest, "\x00") {
		return "", false
	}
	return rest, true
}

func uniqueID(base string, seen map[string]bool) string {
	if !seen[base] {
		return base
	}

	for i := 2; ; i++ {
		id := fmt.Sprintf("%s_%d", base, i)
		if !seen[id] {
			return id
		}
	}
}

func rebuildIndexFromSubscriptions(stagingDir string) ([]clash.SubIndexItem, error) {
	subDir := filepath.Join(stagingDir, "Subscriptions")

	if _, err := os.Stat(subDir); err != nil {
		return nil, err
	}

	items := make([]clash.SubIndexItem, 0)
	seen := make(map[string]bool)

	err := filepath.WalkDir(subDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		lower := strings.ToLower(name)

		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			return nil
		}

		id := strings.TrimSuffix(name, filepath.Ext(name))

		safeID, err := utils.SanitizeFilename(id)
		if err != nil || safeID != id || id == "" {
			return nil
		}

		finalID := uniqueID(id, seen)
		seen[finalID] = true

		items = append(items, clash.SubIndexItem{
			ID:   finalID,
			Name: finalID,
			Type: "local",
		})

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("в резервной копии не найдено конфигураций подписок для восстановления")
	}

	return items, nil
}
