package search

import "strings"

var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// ContainsPattern 与 SQL 的 ESCAPE '!' 配合，将用户输入按字面匹配。
func ContainsPattern(keyword string) string {
	return "%" + likeEscaper.Replace(keyword) + "%"
}
