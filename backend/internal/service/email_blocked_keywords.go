package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

const (
	maxEmailBlockedKeywords      = 100
	maxEmailBlockedKeywordLength = 200
)

func NormalizeEmailBlockedKeywords(keywords []string) ([]string, error) {
	if len(keywords) > maxEmailBlockedKeywords {
		return nil, fmt.Errorf("email_blocked_keywords must contain at most %d keywords", maxEmailBlockedKeywords)
	}
	normalized := make([]string, 0, len(keywords))
	seen := make(map[string]bool, len(keywords))
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if keyword == "" {
			continue
		}
		if utf8.RuneCountInString(keyword) > maxEmailBlockedKeywordLength {
			return nil, fmt.Errorf("email_blocked_keywords entries must contain at most %d characters", maxEmailBlockedKeywordLength)
		}
		key := strings.ToLower(keyword)
		if !seen[key] {
			seen[key] = true
			normalized = append(normalized, keyword)
		}
	}
	return normalized, nil
}

func parseEmailBlockedKeywords(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return []string{}, nil
	}
	var keywords []string
	if err := json.Unmarshal([]byte(raw), &keywords); err != nil || keywords == nil {
		return nil, fmt.Errorf("invalid email_blocked_keywords setting: expected a string array")
	}
	return NormalizeEmailBlockedKeywords(keywords)
}

func emailMatchesBlockedKeywords(keywords []string, subject, body string) bool {
	if len(keywords) == 0 {
		return false
	}
	rawSubject := strings.ToLower(subject)
	subject = strings.ToLower(sanitizeEmailHeader(subject))
	rawBody := strings.ToLower(body)
	visibleBody := strings.ToLower(emailVisibleText(body))
	for _, keyword := range keywords {
		keyword = strings.ToLower(keyword)
		if strings.Contains(rawSubject, keyword) || strings.Contains(subject, keyword) || strings.Contains(rawBody, keyword) || strings.Contains(visibleBody, keyword) {
			return true
		}
	}
	return false
}

func emailVisibleText(body string) string {
	tokenizer := html.NewTokenizer(strings.NewReader(body))
	var text strings.Builder
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return text.String()
		case html.TextToken:
			// Text decodes entities; adjacent inline text remains contiguous.
			_, _ = text.Write(tokenizer.Text())
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, _ := tokenizer.TagName()
			switch string(name) {
			case "address", "article", "aside", "blockquote", "br", "dd", "div", "dl", "dt", "fieldset", "figcaption", "figure", "footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header", "hr", "li", "main", "nav", "ol", "p", "pre", "section", "table", "td", "th", "tr", "ul":
				_ = text.WriteByte('\n')
			}
		}
	}
}
