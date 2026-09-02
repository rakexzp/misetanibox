package clash

import (
	"encoding/json"
	"goclashz/core/utils"
	"os"
	"path/filepath"
	"sync"
)

type SubIndexItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// заголовок подписки от панели (profile-title) — обновляется при каждой загрузке
	Title    string `json:"title,omitempty"`
	URL      string `json:"url"`
	Type     string `json:"type"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
	Total    int64  `json:"total"`
	Expire   int64  `json:"expire"`
	Updated  int64  `json:"updated"`
	// Запасные ссылки на ту же подписку: пробуются вместе с основной, берётся первая ответившая.
	FallbackURLs []string `json:"fallbackUrls,omitempty"`
	// Свои HTTP-заголовки для запроса подписки (перекрывают стандартные).
	Headers map[string]string `json:"headers,omitempty"`
	// Личный кабинет / страница продления — приходит заголовком от панели.
	WebPageURL string `json:"webPageUrl,omitempty"`
	// Конвертировать Xray-JSON в mihomo-YAML при загрузке (панель отдаёт xray-формат).
	Convert bool `json:"convert,omitempty"`
}

var (
	IndexLock sync.RWMutex
	indexIOMu sync.Mutex
	SubIndex  []SubIndexItem
)

func getIndexPath() string {
	return filepath.Join(utils.GetProfilesDir(), "index.json")
}

func LoadIndex() error {
	IndexLock.Lock()
	defer IndexLock.Unlock()
	data, err := os.ReadFile(getIndexPath())
	if err != nil {
		SubIndex = []SubIndexItem{}
		return nil
	}
	return json.Unmarshal(data, &SubIndex)
}

func SaveIndex() error {

	IndexLock.RLock()
	data, err := json.MarshalIndent(SubIndex, "", "  ")
	IndexLock.RUnlock()

	if err != nil {
		return err
	}

	indexIOMu.Lock()
	defer indexIOMu.Unlock()

	indexPath := getIndexPath()
	tempPath := indexPath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tempPath, indexPath); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}

func ListSubIndex() []SubIndexItem {
	IndexLock.RLock()
	defer IndexLock.RUnlock()

	out := make([]SubIndexItem, len(SubIndex))
	copy(out, SubIndex)
	return out
}

func FindSubIndexByID(id string) (SubIndexItem, bool) {
	IndexLock.RLock()
	defer IndexLock.RUnlock()

	for _, item := range SubIndex {
		if item.ID == id {
			return item, true
		}
	}
	return SubIndexItem{}, false
}

func ReplaceSubIndex(next []SubIndexItem) error {
	IndexLock.Lock()
	SubIndex = append([]SubIndexItem(nil), next...)
	IndexLock.Unlock()

	return SaveIndex()
}

func UpdateSubIndex(mutator func([]SubIndexItem) ([]SubIndexItem, error)) error {
	IndexLock.Lock()
	next, err := mutator(append([]SubIndexItem(nil), SubIndex...))
	if err == nil {
		SubIndex = next
	}
	IndexLock.Unlock()

	if err != nil {
		return err
	}
	return SaveIndex()
}
