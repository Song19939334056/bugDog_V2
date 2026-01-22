package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var numberPattern = regexp.MustCompile(`\d+`)

func (a *App) scrape(ctx context.Context, cfg Config) (Stats, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		return Stats{}, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if cfg.Cookie != "" {
		req.Header.Set("Cookie", cfg.Cookie)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return Stats{}, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Stats{}, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return Stats{}, resp.StatusCode, fmt.Errorf("bad response: %s", resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return Stats{}, resp.StatusCode, err
	}

	stats := parseStats(doc)
	stats.LastUpdated = time.Now()
	return stats, resp.StatusCode, nil
}

func parseStats(doc *goquery.Document) Stats {
	severityIndex := findSeverityIndex(doc)
	rows := findBugRows(doc)
	severity := SeverityCounts{}

	rows.Each(func(_ int, row *goquery.Selection) {
		sevText := extractSeverityText(row, severityIndex)
		switch normalizeSeverity(sevText) {
		case "critical":
			severity.Critical++
		case "severe":
			severity.Severe++
		case "major":
			severity.Major++
		case "minor":
			severity.Minor++
		}
	})

	total := parseTotalCount(doc)
	if total == 0 {
		total = rows.Length()
	}

	return Stats{
		Total:    total,
		Severity: severity,
	}
}

func findBugRows(doc *goquery.Document) *goquery.Selection {
	candidates := []*goquery.Selection{
		doc.Find("table#bugList tbody tr"),
		doc.Find("#bugList tbody tr"),
		doc.Find("table.datatable tbody tr"),
		doc.Find("table tbody tr"),
	}
	for _, rows := range candidates {
		filtered := rows.FilterFunction(func(_ int, sel *goquery.Selection) bool {
			return sel.Find("td").Length() > 0
		})
		if filtered.Length() > 0 {
			return filtered
		}
	}
	return doc.Find("table#bugList tbody tr")
}

func findSeverityIndex(doc *goquery.Document) int {
	headers := doc.Find("table#bugList thead th")
	if headers.Length() == 0 {
		headers = doc.Find("table thead th")
	}
	index := -1
	headers.Each(func(i int, header *goquery.Selection) {
		text := strings.TrimSpace(header.Text())
		lower := strings.ToLower(text)
		if strings.Contains(text, "严重") || strings.Contains(text, "致命") || strings.Contains(lower, "severity") {
			index = i
		}
	})
	return index
}

func extractSeverityText(row *goquery.Selection, index int) string {
	thirdCellSpan := strings.TrimSpace(row.Find("td:nth-child(3) span").First().Text())
	if thirdCellSpan != "" {
		return thirdCellSpan
	}
	selectors := []string{
		"td.c-severity",
		"td.severity",
		"td[data-col='severity']",
		"td[data-type='severity']",
	}
	for _, selector := range selectors {
		text := strings.TrimSpace(row.Find(selector).First().Text())
		if text != "" {
			return text
		}
	}
	if index >= 0 {
		cells := row.Find("td")
		if index < cells.Length() {
			return strings.TrimSpace(cells.Eq(index).Text())
		}
	}
	return ""
}

func normalizeSeverity(text string) string {
	if text == "" {
		return ""
	}
	value := strings.TrimSpace(text)
	lower := strings.ToLower(value)
	if strings.Contains(value, "致命") || strings.Contains(lower, "critical") || strings.Contains(lower, "blocker") {
		return "critical"
	}
	if strings.Contains(value, "严重") || strings.Contains(lower, "high") {
		return "severe"
	}
	if strings.Contains(value, "主要") || strings.Contains(lower, "major") {
		return "major"
	}
	if strings.Contains(value, "次要") || strings.Contains(value, "轻微") || strings.Contains(lower, "minor") {
		return "minor"
	}
	if match := numberPattern.FindString(lower); match != "" {
		if num, err := strconv.Atoi(match); err == nil {
			switch num {
			case 1:
				return "critical"
			case 2:
				return "severe"
			case 3:
				return "major"
			case 4:
				return "minor"
			}
		}
	}
	return ""
}

func parseTotalCount(doc *goquery.Document) int {
	selectors := []string{
		"#bugCount",
		".pager .page-summary",
		".pager .total",
		".page-summary",
		".table-footer",
		".table-actions",
	}
	for _, selector := range selectors {
		text := strings.TrimSpace(doc.Find(selector).First().Text())
		if count := parseNumber(text); count > 0 {
			return count
		}
	}
	return 0
}

func parseNumber(text string) int {
	match := numberPattern.FindString(text)
	if match == "" {
		return 0
	}
	value, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return value
}
