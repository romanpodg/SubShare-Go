package delivery

import (
	"sort"
	"strconv"
	"strings"
)

func ApplyExclusionHeaders(headers interface{ Set(string, string) }, exclusions []Exclusion) {
	if len(exclusions) == 0 {
		return
	}
	counts := ExclusionCounts(exclusions)
	ordered := make([]string, 0, len(counts))
	for code := range counts {
		ordered = append(ordered, code)
	}
	sort.Strings(ordered)
	summary := make([]string, 0, len(ordered))
	for _, code := range ordered {
		summary = append(summary, code+"="+strconv.Itoa(counts[code]))
	}
	headers.Set("SubShare-Excluded-Count", strconv.Itoa(len(exclusions)))
	headers.Set("SubShare-Exclusion-Codes", strings.Join(ordered, ","))
	headers.Set("SubShare-Exclusion-Counts", strings.Join(summary, ","))
}

func ExclusionCounts(exclusions []Exclusion) map[string]int {
	allowed := make(map[string]struct{})
	for _, code := range ExclusionReasonCodes() {
		allowed[code] = struct{}{}
	}
	counts := make(map[string]int)
	for _, exclusion := range exclusions {
		if _, ok := allowed[exclusion.Reason]; ok {
			counts[exclusion.Reason]++
		}
	}
	return counts
}

func FailurePayload(generated Generated) Failure {
	return Failure{
		ErrorCode:       ReasonAllExcluded,
		OutputFormat:    generated.OutputFormat,
		EligibleCount:   generated.EligibleCount,
		ExcludedCount:   len(generated.Exclusions),
		ExclusionCounts: ExclusionCounts(generated.Exclusions),
	}
}
