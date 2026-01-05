package infra

import (
	"encoding/json"
	"fmt"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

//nolint:govet,nestif // expected complexity
func ParseRawResult[T any](raw interface{}) ([]T, int, error) {
	keys := []T{}
	var total int

	if resultSlice, ok := raw.([]interface{}); ok && len(resultSlice) > 0 {
		// First element is total count
		switch v := resultSlice[0].(type) {
		case int64:
			total = int(v)
		case int:
			total = v
		}

		// Parse remaining elements: [key, map, key, map, ...]
		for i := 1; i < len(resultSlice); i += 2 {
			// Skip key name at i, get map at i+1
			if i+1 >= len(resultSlice) {
				break
			}

			if docMap, ok := resultSlice[i+1].(map[interface{}]interface{}); ok {
				if jsonStr, ok := docMap["$"].(string); ok {
					var key T
					if err := json.Unmarshal([]byte(jsonStr), &key); err == nil {
						keys = append(keys, key)
					}
				}
			}
		}
	}

	return keys, total, nil
}

type SearchResultItem struct {
	Entity sshkeys.Entity `json:"entity"`
}

type SearchResult []SearchResultItem

func (s SearchResult) ToSearchEntities() []sshkeys.Entity {
	sr := []sshkeys.Entity{}

	for _, item := range s {
		sr = append(sr, item.Entity)
	}

	return sr
}

//nolint:govet,nestif // expected complexity
func ParseSearchResult(raw interface{}) (SearchResult, int, error) {
	var items SearchResult
	var total int

	if resultSlice, ok := raw.([]interface{}); ok && len(resultSlice) > 0 {
		switch v := resultSlice[0].(type) {
		case int64:
			total = int(v)
		case int:
			total = v
		}

		for i := 1; i < len(resultSlice); i += 2 {
			if i+1 >= len(resultSlice) {
				break
			}

			if docMap, ok := resultSlice[i+1].(map[interface{}]interface{}); ok {
				if jsonStr, ok := docMap["$"].(string); ok {
					item := SearchResultItem{}
					if err := json.Unmarshal([]byte(jsonStr), &item.Entity); err == nil {
						items = append(items, item)
					}
				}
			}
		}
	}

	return items, total, nil
}

func FullIndexKey(indexKey string) string {
	return fmt.Sprintf("idx:%s", indexKey)
}
