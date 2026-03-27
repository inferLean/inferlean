package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func writePrometheusConfig(path string, scrapeEvery time.Duration, vllmTarget, nodeTarget, dcgmTarget string) error {
	var b strings.Builder
	b.WriteString("global:\n")
	b.WriteString(fmt.Sprintf("  scrape_interval: %s\n", scrapeEvery))
	b.WriteString(fmt.Sprintf("  evaluation_interval: %s\n", scrapeEvery))
	b.WriteString("scrape_configs:\n")
	for _, job := range buildScrapeJobs(vllmTarget, nodeTarget, dcgmTarget) {
		writeScrapeJob(&b, job[0], job[1])
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write prometheus config: %w", err)
	}
	return nil
}

func buildScrapeJobs(vllmTarget, nodeTarget, dcgmTarget string) [][2]string {
	jobs := [][2]string{{"node_exporter", nodeTarget}}
	if vllmTarget != "" {
		jobs = append(jobs, [2]string{"vllm", vllmTarget})
	}
	if dcgmTarget != "" {
		jobs = append(jobs, [2]string{"dcgm", dcgmTarget})
	}
	return jobs
}

func writeScrapeJob(b *strings.Builder, name, target string) {
	b.WriteString(fmt.Sprintf("  - job_name: %q\n", name))
	b.WriteString("    static_configs:\n")
	b.WriteString("      - targets:\n")
	b.WriteString(fmt.Sprintf("          - %q\n", target))
}

func (s Service) waitForPrometheusReady(ctx context.Context, baseURL string) error {
	return waitForDeadline(healthTimeout, 250*time.Millisecond, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/-/ready", nil)
		if err != nil {
			return false
		}
		resp, err := s.client.Do(req)
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, "prometheus did not become ready in time")
}

func (s Service) waitForPrometheusTargets(ctx context.Context, baseURL string, jobs []string) error {
	if len(jobs) == 0 {
		return nil
	}
	required := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		required[job] = struct{}{}
	}
	return waitForDeadline(healthTimeout, 500*time.Millisecond, func() bool {
		healthy, err := s.healthyPrometheusJobs(ctx, baseURL, required)
		return err == nil && len(healthy) == len(required)
	}, "not all scrape targets became healthy")
}

func waitForDeadline(timeout, interval time.Duration, ready func() bool, message string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ready() {
			return nil
		}
		time.Sleep(interval)
	}
	return errors.New(message)
}

func (s Service) healthyPrometheusJobs(ctx context.Context, baseURL string, required map[string]struct{}) (map[string]bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/v1/targets", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var body promTargetsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status != "success" {
		return nil, err
	}

	healthy := map[string]bool{}
	for _, target := range body.Data.ActiveTargets {
		job := target.Labels["job"]
		if _, ok := required[job]; ok && target.Health == "up" {
			healthy[job] = true
		}
	}
	return healthy, nil
}

func fetchToFile(ctx context.Context, client *http.Client, rawURL, path string) error {
	if rawURL == "" {
		return errors.New("raw metrics URL was empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return writeResponse(path, resp.Body)
}

func writeResponse(path string, body io.Reader) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, body)
	return err
}
