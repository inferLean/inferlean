package report

import (
	"sort"

	"github.com/inferLean/inferlean-main/cli/pkg/contracts"
)

func displayIssues(report contracts.FinalReport) []contracts.Issue {
	issues := append([]contracts.Issue(nil), report.Issues...)
	sort.SliceStable(issues, func(i, j int) bool {
		return rankSortValue(issues[i].Rank) < rankSortValue(issues[j].Rank)
	})
	return issues
}

func displayOpportunities(report contracts.FinalReport) []contracts.Opportunity {
	opportunities := append([]contracts.Opportunity(nil), report.Opportunities...)
	sort.SliceStable(opportunities, func(i, j int) bool {
		return rankSortValue(opportunities[i].Rank) < rankSortValue(opportunities[j].Rank)
	})
	return opportunities
}

func rankSortValue(rank int) int {
	if rank <= 0 {
		return 99
	}
	return rank
}
