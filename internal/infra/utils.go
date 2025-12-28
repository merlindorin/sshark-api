package infra

import (
	"encoding/json"
	"fmt"

	"github.com/merlindorin/sshark-api/internal/domain/sshkeys"
)

//nolint:govet,nestif,gocognit // expected complexity
func ParseRawResult[T any](raw interface{}) ([]T, int, error) {
	keys := []T{}
	var total int

	if resultMap, ok := raw.(map[interface{}]interface{}); ok {
		if totalResults, ok := resultMap["total_results"].(int64); ok {
			total = int(totalResults)
		}
		if results, ok := resultMap["results"]; ok {
			if resultsSlice, ok := results.([]interface{}); ok {
				for _, r := range resultsSlice {
					if doc, ok := r.(map[interface{}]interface{}); ok {
						if extraAttrs, ok := doc["extra_attributes"]; ok {
							if attrs, ok := extraAttrs.(map[interface{}]interface{}); ok {
								if jsonStr, ok := attrs["$"].(string); ok {
									var key T

									err := json.Unmarshal([]byte(jsonStr), &key)
									if err != nil {
										return nil, 0, fmt.Errorf("failed to parse JSON result: %w", err)
									}

									keys = append(keys, key)
								}
							}
						}
					}
				}
			}
		}
	}

	return keys, total, nil
}

type SearchResultItemWithScore struct {
	Entity sshkeys.Entity `json:"entity"`
	Score  float64        `json:"score"`
}

type SearchResultWithScore []SearchResultItemWithScore

func (s SearchResultWithScore) ToSearchEntities() []sshkeys.Entity {
	sr := []sshkeys.Entity{}

	for _, item := range s {
		sr = append(sr, item.Entity)
	}

	return sr
}

//nolint:govet,nestif,gocognit // expected complexity
func ParseSearchResult(raw interface{}) (SearchResultWithScore, int, error) {
	var items SearchResultWithScore
	var total int

	if resultMap, ok := raw.(map[interface{}]interface{}); ok {
		if totalResults, ok := resultMap["total_results"].(int64); ok {
			total = int(totalResults)
		}
		if results, ok := resultMap["results"]; ok {
			if resultsSlice, ok := results.([]interface{}); ok {
				for _, r := range resultsSlice {
					if doc, ok := r.(map[interface{}]interface{}); ok {
						item := SearchResultItemWithScore{}

						// Extract score
						if score, ok := doc["score"].(float64); ok {
							item.Score = score
						}

						// Extract entity from extra_attributes
						if extraAttrs, ok := doc["extra_attributes"]; ok {
							if attrs, ok := extraAttrs.(map[interface{}]interface{}); ok {
								if jsonStr, ok := attrs["$"].(string); ok {
									err := json.Unmarshal([]byte(jsonStr), &item.Entity)
									if err != nil {
										return nil, 0, fmt.Errorf("failed to parse JSON result: %w", err)
									}
								}
							}
						}

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
