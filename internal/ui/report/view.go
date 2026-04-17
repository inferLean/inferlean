package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type View struct{}

func NewView() View {
	return View{}
}

func (View) Render(report map[string]any) {
	fmt.Println("[report] brief view")
	if diagnosis, ok := report["diagnosis"].(map[string]any); ok {
		fmt.Printf("  diagnosis keys: %v\n", mapKeys(diagnosis))
	}
	if entitlement, ok := report["entitlement"].(map[string]any); ok {
		fmt.Printf("  entitlement: %v\n", entitlement)
	}
	fmt.Print("Show full report? (y/N): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "y") {
		return
	}
	fmt.Println("[report] full view")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("[report] failed to render full report: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func mapKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}
