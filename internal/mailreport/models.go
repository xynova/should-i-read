package mailreport

import "strings"

// PickJudgeAndAuthorModels chooses SystemOne (JEV) vs chat model ids.
func PickJudgeAndAuthorModels(ids []string, judgeYAML, authorYAML string) (judge, author string) {
	judge = strings.TrimSpace(judgeYAML)
	author = strings.TrimSpace(authorYAML)
	if judge == "" {
		for _, id := range ids {
			if isJevModelID(id) {
				judge = id
				break
			}
		}
	}
	if author == "" {
		author, _ = PickAuthorModel(ids, "")
	}
	return judge, author
}

func isJevModelID(id string) bool {
	s := strings.ToLower(strings.TrimSpace(id))
	if s == "" {
		return false
	}
	if strings.Contains(s, "typesafe/jev") {
		return true
	}
	return strings.HasSuffix(s, "/jev") || s == "jev"
}
