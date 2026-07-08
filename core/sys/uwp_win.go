//go:build windows

package sys

import (
	"fmt"
	"goclashz/core/utils"
	"golang.org/x/sys/windows/registry"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type UwpApp struct {
	DisplayName       string `json:"displayName"`
	PackageFamilyName string `json:"packageFamilyName"`
	SID               string `json:"sid"`
	IsEnabled         bool   `json:"isEnabled"`
}

func GetUwpAppList() ([]UwpApp, error) {

	exemptedSids, err := getExemptedSids()
	if err != nil {
		return nil, err
	}

	const mappingKey = `Local Settings\Software\Microsoft\Windows\CurrentVersion\AppContainer\Mappings`
	k, err := registry.OpenKey(registry.CLASSES_ROOT, mappingKey, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать сопоставления в реестре: %v", err)
	}
	defer k.Close()

	sids, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil, err
	}

	var apps []UwpApp
	for _, sid := range sids {
		subKey, err := registry.OpenKey(registry.CLASSES_ROOT, mappingKey+`\`+sid, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		moniker, _, _ := subKey.GetStringValue("Moniker")
		displayName, _, _ := subKey.GetStringValue("DisplayName")
		subKey.Close()

		if moniker == "" {
			continue
		}

		finalName := displayName
		if finalName == "" || strings.HasPrefix(finalName, "@") {
			finalName = moniker
		}

		apps = append(apps, UwpApp{
			DisplayName:       finalName,
			PackageFamilyName: moniker,
			SID:               sid,
			IsEnabled:         exemptedSids[sid],
		})
	}

	return apps, nil
}

var uwpSidRegex = regexp.MustCompile(`S-1-15-[-0-9]+`)

func getExemptedSids() (map[string]bool, error) {
	exemptMap := make(map[string]bool)
	cmd := exec.Command("CheckNetIsolation.exe", "LoopbackExempt", "-s")
	utils.HideCommandWindow(cmd, 0)

	output, err := cmd.Output()
	if err != nil {
		return exemptMap, nil
	}

	matches := uwpSidRegex.FindAllString(string(output), -1)
	for _, sid := range matches {
		exemptMap[sid] = true
	}

	return exemptMap, nil
}

func SaveUwpExemptions(targetSids []string) error {

	currentExempted, err := getExemptedSids()
	if err != nil {
		return err
	}

	targetMap := make(map[string]bool)
	for _, sid := range targetSids {
		targetMap[sid] = true
	}

	sys32 := filepath.Join(os.Getenv("SystemRoot"), "System32", "CheckNetIsolation.exe")

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)

	runCmdAsync := func(args ...string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()

		cmd := exec.Command(sys32, args...)
		utils.HideCommandWindow(cmd, 0)
		_ = cmd.Run()
	}

	for sid := range currentExempted {
		if !targetMap[sid] {
			wg.Add(1)
			go runCmdAsync("LoopbackExempt", "-d", "-p="+sid)
		}
	}

	for sid := range targetMap {
		if !currentExempted[sid] {
			wg.Add(1)
			go runCmdAsync("LoopbackExempt", "-a", "-p="+sid)
		}
	}

	wg.Wait()
	return nil
}

func ExemptAllUWP() error {
	apps, err := GetUwpAppList()
	if err != nil {
		return err
	}

	var sids []string
	for _, app := range apps {
		sids = append(sids, app.SID)
	}

	return SaveUwpExemptions(sids)
}
